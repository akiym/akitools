package sbxbackup

import (
	"bufio"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"io/fs"
	"os"
	"os/exec"
	"path"
	"path/filepath"
	"strings"

	"github.com/mattn/go-isatty"
	"github.com/spf13/cobra"
)

// Requirements:
// - sbx (Docker Sandboxes CLI)

var agentPaths = map[string][]string{
	"claude": {".claude/projects"},
	"codex":  {".codex/sessions"},
	"opencode": {
		".local/share/opencode/opencode.db",
		".local/share/opencode/opencode.db-wal",
		".local/share/opencode/opencode.db-shm",
	},
}

var logPaths = []struct {
	src string
	dst string
}{
	{".claude/projects", "claude/projects"},
	{".codex/sessions", "codex/sessions"},
}

var (
	flagKeep bool
	flagYes  bool
	flagAll  bool
)

var Cmd = &cobra.Command{
	Use:   "sbx-backup [SANDBOX...]",
	Short: "Backup agent logs (claude, codex, opencode) from stopped sbx sandboxes, then remove them",
	RunE: func(cmd *cobra.Command, args []string) error {
		return run(args)
	},
}

func init() {
	Cmd.Flags().BoolVar(&flagKeep, "keep", false, "backup only; do not remove sandboxes")
	Cmd.Flags().BoolVarP(&flagYes, "yes", "y", false, "remove sandboxes without confirmation")
	Cmd.Flags().BoolVarP(&flagAll, "all", "a", false, "include running sandboxes")
}

type sandbox struct {
	Name   string
	ID     string
	Agent  string
	Status string
}

func run(args []string) error {
	sandboxes, err := listSandboxes()
	if err != nil {
		return err
	}

	var targets []sandbox
	if len(args) > 0 {
		byName := make(map[string]sandbox, len(sandboxes))
		for _, s := range sandboxes {
			byName[s.Name] = s
		}
		for _, name := range args {
			s, ok := byName[name]
			if !ok {
				return fmt.Errorf("sandbox not found: %s", name)
			}
			if isRunning(s.Status) && !flagAll {
				return fmt.Errorf("sandbox %s is running; stop it first or use --all", name)
			}
			targets = append(targets, s)
		}
	} else {
		for _, s := range sandboxes {
			if flagAll || !isRunning(s.Status) {
				targets = append(targets, s)
			}
		}
		if len(targets) == 0 {
			fmt.Println("no stopped sandboxes")
			return nil
		}
	}

	root := filepath.Join(xdgDataHome(), "akitools", "sbx-backup")
	for _, s := range targets {
		if err := backupOne(s, root); err != nil {
			return fmt.Errorf("%s: %w", s.Name, err)
		}
	}
	return nil
}

func listSandboxes() ([]sandbox, error) {
	out, err := exec.Command("sbx", "ls", "--json").Output()
	if err != nil {
		return nil, fmt.Errorf("sbx ls --json: %w", err)
	}
	return parseSandboxes(out)
}

func parseSandboxes(data []byte) ([]sandbox, error) {
	var v any
	if err := json.Unmarshal(data, &v); err != nil {
		return nil, fmt.Errorf("parse sbx ls output: %w", err)
	}
	items, ok := v.([]any)
	if !ok {
		m, ok := v.(map[string]any)
		if !ok {
			return nil, errors.New("unexpected sbx ls output format")
		}
		for _, key := range []string{"sandboxes", "items"} {
			if a, ok := m[key].([]any); ok {
				items = a
				break
			}
		}
		if items == nil {
			return nil, errors.New("unexpected sbx ls output format")
		}
	}
	var out []sandbox
	for _, it := range items {
		m, ok := it.(map[string]any)
		if !ok {
			continue
		}
		s := sandbox{
			Name:   stringField(m, "name", "Name", "id", "ID"),
			ID:     stringField(m, "id", "ID"),
			Agent:  stringField(m, "agent", "Agent"),
			Status: stringField(m, "status", "state", "Status", "State"),
		}
		if s.Name != "" {
			out = append(out, s)
		}
	}
	return out, nil
}

func stringField(m map[string]any, keys ...string) string {
	for _, k := range keys {
		if v, ok := m[k].(string); ok && v != "" {
			return v
		}
	}
	return ""
}

func isRunning(status string) bool {
	return strings.Contains(strings.ToLower(status), "running")
}

func xdgDataHome() string {
	if v := os.Getenv("XDG_DATA_HOME"); v != "" {
		return v
	}
	home, err := os.UserHomeDir()
	if err != nil {
		return ".local/share"
	}
	return filepath.Join(home, ".local", "share")
}

func backupPaths(agent string) []string {
	if paths, ok := agentPaths[agent]; ok {
		return paths
	}
	var all []string
	for _, a := range []string{"claude", "codex", "opencode"} {
		all = append(all, agentPaths[a]...)
	}
	return all
}

func backupOne(s sandbox, root string) error {
	fmt.Printf("backing up %s\n", s.Name)

	home, err := sandboxHome(s.Name)
	if err != nil {
		return err
	}

	staging, err := os.MkdirTemp("", "sbx-backup-")
	if err != nil {
		return err
	}
	defer os.RemoveAll(staging)

	copied := 0
	for _, rel := range backupPaths(s.Agent) {
		src := s.Name + ":" + path.Join(home, rel)
		dst := filepath.Join(staging, filepath.FromSlash(rel))
		if err := os.MkdirAll(filepath.Dir(dst), 0o755); err != nil {
			return err
		}
		out, err := exec.Command("sbx", "cp", src, dst).CombinedOutput()
		if err != nil {
			if !isOptionalPath(rel) {
				fmt.Printf("  skip %s: %s\n", rel, firstLine(out))
			}
			continue
		}
		fmt.Printf("  copied %s\n", rel)
		copied++
	}

	if copied == 0 {
		fmt.Printf("  nothing to backup\n")
	} else {
		if err := extractLogs(staging, root, s); err != nil {
			return err
		}
	}

	wasRunning := isRunning(s.Status)
	if flagKeep {
		if !wasRunning {
			stopSandbox(s.Name)
		}
		return nil
	}
	if !flagYes && !confirmRemoval(s.Name) {
		fmt.Printf("  kept %s\n", s.Name)
		if !wasRunning {
			stopSandbox(s.Name)
		}
		return nil
	}
	return removeSandbox(s.Name)
}

func isOptionalPath(rel string) bool {
	return strings.HasSuffix(rel, "-wal") || strings.HasSuffix(rel, "-shm")
}

func extractLogs(staging, root string, s sandbox) error {
	for _, p := range logPaths {
		src := filepath.Join(staging, filepath.FromSlash(p.src))
		if _, err := os.Stat(src); err != nil {
			continue
		}
		dst := filepath.Join(root, filepath.FromSlash(p.dst))
		if err := mergeCopy(src, dst); err != nil {
			return err
		}
		fmt.Printf("  logs: %s\n", dst)
	}

	matches, err := filepath.Glob(filepath.Join(staging, ".local", "share", "opencode", "opencode.db*"))
	if err != nil {
		return err
	}
	for _, m := range matches {
		suffix := strings.TrimPrefix(filepath.Base(m), "opencode.db")
		dst := filepath.Join(root, "opencode", fmt.Sprintf("%s-%s.opencode.db%s", s.Name, shortID(s.ID), suffix))
		if err := copyFile(m, dst); err != nil {
			return err
		}
		fmt.Printf("  logs: %s\n", dst)
	}
	return nil
}

func shortID(id string) string {
	if i := strings.IndexByte(id, '-'); i > 0 {
		return id[:i]
	}
	return id
}

func mergeCopy(src, dst string) error {
	return filepath.WalkDir(src, func(p string, d fs.DirEntry, err error) error {
		if err != nil {
			return err
		}
		rel, err := filepath.Rel(src, p)
		if err != nil {
			return err
		}
		target := filepath.Join(dst, rel)
		if d.IsDir() {
			return os.MkdirAll(target, 0o755)
		}
		if !d.Type().IsRegular() {
			return nil
		}
		return copyFile(p, target)
	})
}

func copyFile(src, dst string) error {
	in, err := os.Open(src)
	if err != nil {
		return err
	}
	defer in.Close()
	info, err := in.Stat()
	if err != nil {
		return err
	}
	if err := os.MkdirAll(filepath.Dir(dst), 0o755); err != nil {
		return err
	}
	out, err := os.OpenFile(dst, os.O_WRONLY|os.O_CREATE|os.O_TRUNC, info.Mode().Perm())
	if err != nil {
		return err
	}
	if _, err := io.Copy(out, in); err != nil {
		out.Close()
		return err
	}
	return out.Close()
}

func sandboxHome(name string) (string, error) {
	out, err := exec.Command("sbx", "exec", name, "sh", "-c", "echo $HOME").Output()
	if err != nil {
		return "", fmt.Errorf("detect home directory: %w", err)
	}
	home := strings.TrimSpace(string(out))
	if home == "" {
		return "", errors.New("detect home directory: empty $HOME")
	}
	return home, nil
}

func confirmRemoval(name string) bool {
	if !isatty.IsTerminal(os.Stdin.Fd()) {
		fmt.Printf("  stdin is not a terminal; use --yes to remove %s\n", name)
		return false
	}
	fmt.Printf("remove sandbox %s? [y/N]: ", name)
	line, err := bufio.NewReader(os.Stdin).ReadString('\n')
	if err != nil {
		return false
	}
	answer := strings.ToLower(strings.TrimSpace(line))
	return answer == "y" || answer == "yes"
}

func removeSandbox(name string) error {
	cmd := exec.Command("sbx", "rm", "--force", name)
	cmd.Stdout = os.Stdout
	cmd.Stderr = os.Stderr
	return cmd.Run()
}

func stopSandbox(name string) {
	if out, err := exec.Command("sbx", "stop", name).CombinedOutput(); err != nil {
		fmt.Printf("  warning: failed to stop %s: %s\n", name, firstLine(out))
	}
}

func firstLine(out []byte) string {
	s := strings.TrimSpace(string(out))
	if i := strings.IndexByte(s, '\n'); i >= 0 {
		s = s[:i]
	}
	if s == "" {
		return "unknown error"
	}
	return s
}
