package ccwrap

import (
	"bytes"
	"encoding/json"
	"fmt"
	"io"
	"io/fs"
	"os"
	"path/filepath"
	"reflect"
	"sort"
	"strings"
)

type settingsOverride struct {
	path    string
	global  any
	project any
}

type sandboxConcern struct {
	file  string
	path  string
	value any
}

func loadSettings(path string) (map[string]any, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		if os.IsNotExist(err) {
			return nil, nil
		}
		return nil, err
	}
	dec := json.NewDecoder(bytes.NewReader(data))
	dec.UseNumber()
	var settings map[string]any
	if err := dec.Decode(&settings); err != nil {
		return nil, fmt.Errorf("parse %s: %w", path, err)
	}
	return settings, nil
}

func sortedKeys[V any](m map[string]V) []string {
	keys := make([]string, 0, len(m))
	for k := range m {
		keys = append(keys, k)
	}
	sort.Strings(keys)
	return keys
}

func findOverrides(global, project map[string]any, prefix string) []settingsOverride {
	var overrides []settingsOverride
	for _, k := range sortedKeys(project) {
		gv, ok := global[k]
		if !ok {
			continue
		}
		pv := project[k]
		path := k
		if prefix != "" {
			path = prefix + "." + k
		}
		gm, gok := gv.(map[string]any)
		pm, pok := pv.(map[string]any)
		if gok && pok {
			overrides = append(overrides, findOverrides(gm, pm, path)...)
			continue
		}
		if !reflect.DeepEqual(gv, pv) {
			overrides = append(overrides, settingsOverride{path: path, global: gv, project: pv})
		}
	}
	return overrides
}

// pathWithinDir はサンドボックス設定のパス表記(~/, /, プロジェクト相対)が
// dir 配下を指しているかを判定する
func pathWithinDir(path, dir, home string) bool {
	switch {
	case path == "~":
		path = home
	case strings.HasPrefix(path, "~/"):
		path = filepath.Join(home, path[2:])
	case strings.HasPrefix(path, "/"):
		path = filepath.Clean(path)
	default:
		path = filepath.Join(dir, path)
	}
	rel, err := filepath.Rel(dir, path)
	if err != nil {
		return false
	}
	return rel == "." || (rel != ".." && !strings.HasPrefix(rel, "../"))
}

// findSandboxConcerns はプロジェクト側の設定からサンドボックスの制限を
// 緩和しうる項目を集める。安全と確定できるもの(deny系、enabled: true、
// cwd配下のallowRead/allowWrite)だけを除外し、それ以外は未知のキーも
// 含めて値にかかわらずすべて報告する
func findSandboxConcerns(file string, settings map[string]any, cwd, home string) []sandboxConcern {
	sandbox, ok := settings["sandbox"].(map[string]any)
	if !ok {
		if v, exists := settings["sandbox"]; exists {
			return []sandboxConcern{{file: file, path: "sandbox", value: v}}
		}
		return nil
	}

	var concerns []sandboxConcern
	add := func(path string, value any) {
		concerns = append(concerns, sandboxConcern{file: file, path: "sandbox." + path, value: value})
	}

	for _, key := range sortedKeys(sandbox) {
		value := sandbox[key]
		switch key {
		case "enabled", "failIfUnavailable":
			if v, ok := value.(bool); ok && v {
				continue
			}
			add(key, value)
		case "autoAllowBashIfSandboxed", "credentials":
			continue
		case "filesystem":
			fs, ok := value.(map[string]any)
			if !ok {
				add(key, value)
				continue
			}
			for _, fkey := range sortedKeys(fs) {
				fval := fs[fkey]
				switch fkey {
				case "denyRead", "denyWrite":
					continue
				case "allowRead", "allowWrite":
					if outside := pathsOutsideDir(fval, cwd, home); len(outside) > 0 {
						add(key+"."+fkey, outside)
					}
				default:
					add(key+"."+fkey, fval)
				}
			}
		case "network":
			network, ok := value.(map[string]any)
			if !ok {
				add(key, value)
				continue
			}
			for _, nkey := range sortedKeys(network) {
				if nkey == "deniedDomains" {
					continue
				}
				add(key+"."+nkey, network[nkey])
			}
		default:
			add(key, value)
		}
	}
	return concerns
}

func pathsOutsideDir(v any, dir, home string) []any {
	entries, ok := v.([]any)
	if !ok {
		return []any{v}
	}
	var outside []any
	for _, e := range entries {
		s, ok := e.(string)
		if !ok || !pathWithinDir(s, dir, home) {
			outside = append(outside, e)
		}
	}
	return outside
}

func jsonString(v any) string {
	b, err := json.Marshal(v)
	if err != nil {
		return fmt.Sprintf("%v", v)
	}
	return string(b)
}

// readLine は1バイトずつ読んで改行までを返す。先読みバッファを持たないので、
// 確認行より後の入力はOSのパイプバッファに残り、後続のclaudeプロセスに渡る
func readLine(r io.Reader) (string, error) {
	var line []byte
	buf := make([]byte, 1)
	for {
		n, err := r.Read(buf)
		if n > 0 {
			if buf[0] == '\n' {
				return string(line), nil
			}
			line = append(line, buf[0])
		}
		if err != nil {
			return string(line), err
		}
	}
}

// findAutoLoadedFiles は cwd 配下でClaude Codeが自動で読み込みうるファイルを
// 列挙する。対象は任意の深さの .claude 配下(skills, agents, commands, rules,
// CLAUDE.mdなど。サブディレクトリの .claude はdirectory-scoped skillsとして
// 遅延発見される)、各階層の CLAUDE.md / CLAUDE.local.md、ルートの .mcp.json。
// ルート直下の settings.json / settings.local.json はconfirmSettingsが中身を
// 検査するうえ、settings.local.jsonはccwrap自身も書き込むため除外する
func findAutoLoadedFiles(cwd string) ([]string, error) {
	var files []string
	err := filepath.WalkDir(cwd, func(path string, d fs.DirEntry, err error) error {
		if err != nil {
			return err
		}
		if d.IsDir() {
			if d.Name() == ".git" && path != cwd {
				return fs.SkipDir
			}
			return nil
		}
		rel, err := filepath.Rel(cwd, path)
		if err != nil {
			return err
		}
		if autoLoadedPath(rel) {
			files = append(files, rel)
		}
		return nil
	})
	if err != nil {
		return nil, err
	}
	return files, nil
}

func autoLoadedPath(rel string) bool {
	switch rel {
	case filepath.Join(".claude", "settings.json"), filepath.Join(".claude", "settings.local.json"):
		return false
	case ".mcp.json":
		return true
	}
	switch filepath.Base(rel) {
	// ".claude" はディレクトリでなくファイルやsymlinkとして置かれている場合
	case "CLAUDE.md", "CLAUDE.local.md", ".claude":
		return true
	}
	for _, seg := range strings.Split(filepath.Dir(rel), string(filepath.Separator)) {
		if seg == ".claude" {
			return true
		}
	}
	return false
}

var projectSettingsFiles = []string{
	filepath.Join(".claude", "settings.json"),
	filepath.Join(".claude", "settings.local.json"),
}

// confirmationTargets は確認対象ファイルの相対パスと内容ハッシュを集める。
// 自動読み込みファイルすべてに加え、overrides/concernsがある場合は
// その元になるプロジェクト側settingsも対象にする
func confirmationTargets(cwd string, autoLoaded []string, hasSettingsFindings bool) (map[string]string, error) {
	current := make(map[string]string, len(autoLoaded))
	for _, f := range autoLoaded {
		h, err := hashFile(filepath.Join(cwd, f))
		if err != nil {
			return nil, fmt.Errorf("hash %s: %w", f, err)
		}
		current[f] = h
	}
	if hasSettingsFindings {
		for _, f := range projectSettingsFiles {
			h, err := hashFile(filepath.Join(cwd, f))
			if err != nil {
				if os.IsNotExist(err) {
					continue
				}
				return nil, fmt.Errorf("hash %s: %w", f, err)
			}
			current[f] = h
		}
	}
	return current, nil
}

func containsSettingsFile(paths ...[]string) bool {
	for _, list := range paths {
		for _, f := range list {
			for _, s := range projectSettingsFiles {
				if f == s {
					return true
				}
			}
		}
	}
	return false
}

func confirmSettings() (bool, error) {
	home, err := os.UserHomeDir()
	if err != nil {
		return false, err
	}
	cwd, err := os.Getwd()
	if err != nil {
		return false, err
	}
	global, err := loadSettings(filepath.Join(home, ".claude", "settings.json"))
	if err != nil {
		return false, err
	}
	project, err := loadSettings(filepath.Join(cwd, ".claude", "settings.json"))
	if err != nil {
		return false, err
	}
	local, err := loadSettings(filepath.Join(cwd, ".claude", "settings.local.json"))
	if err != nil {
		return false, err
	}

	autoLoaded, err := findAutoLoadedFiles(cwd)
	if err != nil {
		return false, err
	}

	overrides := findOverrides(global, project, "")
	concerns := findSandboxConcerns(".claude/settings.json", project, cwd, home)
	concerns = append(concerns, findSandboxConcerns(".claude/settings.local.json", local, cwd, home)...)

	if len(overrides) == 0 && len(concerns) == 0 && len(autoLoaded) == 0 {
		return true, nil
	}

	current, err := confirmationTargets(cwd, autoLoaded, len(overrides) > 0 || len(concerns) > 0)
	if err != nil {
		return false, err
	}
	saved, err := loadApproval(cwd)
	if err != nil {
		return false, err
	}
	added, changed, removed := diffApproval(saved, current)
	if saved != nil && len(added)+len(changed)+len(removed) == 0 {
		return true, nil
	}

	showSettingsFindings := true
	if saved == nil {
		if len(autoLoaded) > 0 {
			fmt.Fprintln(os.Stderr, "ccwrap: project contains files Claude Code loads automatically:")
			for _, f := range autoLoaded {
				fmt.Fprintf(os.Stderr, "  %s\n", f)
			}
		}
	} else {
		fmt.Fprintln(os.Stderr, "ccwrap: previously approved files have changed:")
		for _, f := range added {
			fmt.Fprintf(os.Stderr, "  new: %s\n", f)
		}
		for _, f := range changed {
			fmt.Fprintf(os.Stderr, "  changed: %s\n", f)
		}
		for _, f := range removed {
			fmt.Fprintf(os.Stderr, "  removed: %s\n", f)
		}
		// settingsが承認済みのまま変わっていなければ詳細の再表示は省く
		showSettingsFindings = containsSettingsFile(added, changed)
	}
	if showSettingsFindings {
		if len(overrides) > 0 {
			fmt.Fprintln(os.Stderr, "ccwrap: .claude/settings.json overrides ~/.claude/settings.json:")
			for _, o := range overrides {
				fmt.Fprintf(os.Stderr, "  %s\n    global:  %s\n    project: %s\n", o.path, jsonString(o.global), jsonString(o.project))
			}
		}
		if len(concerns) > 0 {
			fmt.Fprintln(os.Stderr, "ccwrap: project settings relax sandbox restrictions:")
			for _, c := range concerns {
				fmt.Fprintf(os.Stderr, "  %s: %s = %s\n", c.file, c.path, jsonString(c.value))
			}
		}
	}
	fmt.Fprint(os.Stderr, "Continue? [y/N] ")
	line, err := readLine(os.Stdin)
	if err != nil && err != io.EOF {
		return false, err
	}
	switch strings.ToLower(strings.TrimSpace(line)) {
	case "y", "yes":
		if err := saveApproval(cwd, current); err != nil {
			fmt.Fprintf(os.Stderr, "ccwrap: failed to save approval: %v\n", err)
		}
		return true, nil
	}
	return false, nil
}

// managedFilesystemPath は sandbox.filesystem が ccwrap の書き込んだ形
// (allowRead/allowWriteのみ、同一パス1つずつ)かを判定し、そのパスを返す
func managedFilesystemPath(fs any) (string, bool) {
	m, ok := fs.(map[string]any)
	if !ok || len(m) != 2 {
		return "", false
	}
	read, ok := singleString(m["allowRead"])
	if !ok {
		return "", false
	}
	write, ok := singleString(m["allowWrite"])
	if !ok || read != write {
		return "", false
	}
	return read, true
}

func singleString(v any) (string, bool) {
	arr, ok := v.([]any)
	if !ok || len(arr) != 1 {
		return "", false
	}
	s, ok := arr[0].(string)
	return s, ok
}

func ensureLocalSandboxSettings() error {
	cwd, err := os.Getwd()
	if err != nil {
		return err
	}
	home, err := os.UserHomeDir()
	if err != nil {
		return err
	}
	// ホームディレクトリで起動すると ~/.claude/settings.local.json という
	// ユーザースコープのファイルにホーム全体の許可を書いてしまうため除外する
	if cwd == home {
		fmt.Fprintln(os.Stderr, "ccwrap: cwd is the home directory, not writing sandbox settings")
		return nil
	}
	path := filepath.Join(cwd, ".claude", "settings.local.json")
	settings, err := loadSettings(path)
	if err != nil {
		return err
	}
	if settings == nil {
		settings = map[string]any{}
	}
	sandbox, ok := settings["sandbox"].(map[string]any)
	if !ok {
		if v, exists := settings["sandbox"]; exists {
			return fmt.Errorf("unexpected sandbox value in %s: %s", path, jsonString(v))
		}
		sandbox = map[string]any{}
		settings["sandbox"] = sandbox
	}
	if fs, exists := sandbox["filesystem"]; exists {
		// ccwrap自身が書いた形なら旧形式(絶対パス)を"."へ移行する。
		// ユーザーがカスタマイズした形には触れない
		p, managed := managedFilesystemPath(fs)
		if !managed || p == "." {
			return nil
		}
	}
	sandbox["filesystem"] = map[string]any{
		"allowRead":  []string{"."},
		"allowWrite": []string{"."},
	}

	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		return err
	}
	data, err := json.MarshalIndent(settings, "", "  ")
	if err != nil {
		return err
	}
	return os.WriteFile(path, append(data, '\n'), 0o644)
}
