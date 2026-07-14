package ccwrap

import (
	"io"
	"os"
	"path/filepath"
	"reflect"
	"strings"
	"testing"
)

func chdir(t *testing.T, dir string) {
	t.Helper()
	old, err := os.Getwd()
	if err != nil {
		t.Fatal(err)
	}
	if err := os.Chdir(dir); err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { os.Chdir(old) })
}

func TestFindOverrides(t *testing.T) {
	global := map[string]any{
		"model": "opus",
		"sandbox": map[string]any{
			"enabled": true,
			"filesystem": map[string]any{
				"denyRead":  []any{"~/"},
				"denyWrite": []any{"/"},
			},
		},
	}
	project := map[string]any{
		"permissions": map[string]any{"allow": []any{"Bash(go build:*)"}},
		"sandbox": map[string]any{
			"enabled": true,
			"filesystem": map[string]any{
				"denyRead": []any{},
			},
		},
	}

	overrides := findOverrides(global, project, "")
	if len(overrides) != 1 {
		t.Fatalf("expected 1 override, got %d: %+v", len(overrides), overrides)
	}
	if overrides[0].path != "sandbox.filesystem.denyRead" {
		t.Errorf("unexpected path: %s", overrides[0].path)
	}
}

func TestFindOverridesTypeMismatch(t *testing.T) {
	global := map[string]any{"sandbox": map[string]any{"enabled": true}}
	project := map[string]any{"sandbox": false}

	overrides := findOverrides(global, project, "")
	if len(overrides) != 1 {
		t.Fatalf("expected 1 override, got %d", len(overrides))
	}
	if overrides[0].path != "sandbox" {
		t.Errorf("unexpected path: %s", overrides[0].path)
	}
}

func TestFindOverridesNil(t *testing.T) {
	if overrides := findOverrides(nil, map[string]any{"model": "opus"}, ""); len(overrides) != 0 {
		t.Errorf("expected no overrides, got %+v", overrides)
	}
	if overrides := findOverrides(map[string]any{"model": "opus"}, nil, ""); len(overrides) != 0 {
		t.Errorf("expected no overrides, got %+v", overrides)
	}
}

func TestPathWithinDir(t *testing.T) {
	const (
		cwd  = "/Users/akiym/src/proj"
		home = "/Users/akiym"
	)
	tests := []struct {
		path string
		want bool
	}{
		{"/Users/akiym/src/proj", true},
		{"/Users/akiym/src/proj/build", true},
		{"~/src/proj/build", true},
		{"./build", true},
		{"build", true},
		{"../", false},
		{"../../.ssh", false},
		{"/Users/akiym/.config", false},
		{"~/.config/gcloud", false},
		{"~", false},
		{"/Users/akiym/src/proj/../../.config", false},
		{"/", false},
	}
	for _, tt := range tests {
		if got := pathWithinDir(tt.path, cwd, home); got != tt.want {
			t.Errorf("pathWithinDir(%q) = %v, want %v", tt.path, got, tt.want)
		}
	}
}

func TestReadLine(t *testing.T) {
	tests := []struct {
		in      string
		want    string
		wantErr error
	}{
		{"y\n", "y", nil},
		{"y", "y", io.EOF},
		{"", "", io.EOF},
		{"no\nrest", "no", nil},
	}
	for _, tt := range tests {
		r := strings.NewReader(tt.in)
		got, err := readLine(r)
		if got != tt.want || err != tt.wantErr {
			t.Errorf("readLine(%q) = (%q, %v), want (%q, %v)", tt.in, got, err, tt.want, tt.wantErr)
		}
	}
}

func TestReadLineLeavesRemainderUnread(t *testing.T) {
	r := strings.NewReader("y\nfor claude")
	if _, err := readLine(r); err != nil {
		t.Fatal(err)
	}
	rest, err := io.ReadAll(r)
	if err != nil {
		t.Fatal(err)
	}
	if string(rest) != "for claude" {
		t.Errorf("remainder = %q, want %q", rest, "for claude")
	}
}

func concernPaths(concerns []sandboxConcern) []string {
	paths := make([]string, len(concerns))
	for i, c := range concerns {
		paths[i] = c.path
	}
	return paths
}

func TestFindSandboxConcernsSafeSettings(t *testing.T) {
	const (
		cwd  = "/Users/akiym/src/proj"
		home = "/Users/akiym"
	)
	settings := map[string]any{
		"permissions": map[string]any{"allow": []any{"Bash(go build:*)"}},
		"sandbox": map[string]any{
			"enabled":                  true,
			"autoAllowBashIfSandboxed": true,
			"credentials":              map[string]any{"files": []any{map[string]any{"path": "~/.config", "mode": "deny"}}},
			"filesystem": map[string]any{
				"allowRead":  []any{cwd, "./build"},
				"allowWrite": []any{cwd},
				"denyRead":   []any{"~/Documents"},
			},
			"network": map[string]any{
				"deniedDomains": []any{"example.com"},
			},
		},
	}
	if concerns := findSandboxConcerns("f", settings, cwd, home); len(concerns) != 0 {
		t.Errorf("expected no concerns, got %v", concernPaths(concerns))
	}
}

func TestFindSandboxConcernsRelaxations(t *testing.T) {
	const (
		cwd  = "/Users/akiym/src/proj"
		home = "/Users/akiym"
	)
	settings := map[string]any{
		"sandbox": map[string]any{
			"enabled":                  false,
			"allowUnsandboxedCommands": true,
			"excludedCommands":         []any{"docker *"},
			"filesystem": map[string]any{
				"allowRead": []any{cwd, "~/.config/gcloud", "../escape"},
			},
			"network": map[string]any{
				"allowedDomains": []any{"*"},
			},
			"unknownFalsyKey":  false,
			"unknownFutureKey": "on",
		},
	}
	concerns := findSandboxConcerns("f", settings, cwd, home)
	want := []string{
		"sandbox.allowUnsandboxedCommands",
		"sandbox.enabled",
		"sandbox.excludedCommands",
		"sandbox.filesystem.allowRead",
		"sandbox.network.allowedDomains",
		"sandbox.unknownFalsyKey",
		"sandbox.unknownFutureKey",
	}
	if got := concernPaths(concerns); !reflect.DeepEqual(got, want) {
		t.Errorf("concerns = %v, want %v", got, want)
	}
	for _, c := range concerns {
		if c.path == "sandbox.filesystem.allowRead" {
			if !reflect.DeepEqual(c.value, []any{"~/.config/gcloud", "../escape"}) {
				t.Errorf("allowRead concern = %v, want only paths outside cwd", c.value)
			}
		}
	}
}

func TestFindSandboxConcernsNoSandbox(t *testing.T) {
	if concerns := findSandboxConcerns("f", nil, "/cwd", "/home"); concerns != nil {
		t.Errorf("expected nil, got %v", concerns)
	}
	settings := map[string]any{"permissions": map[string]any{}}
	if concerns := findSandboxConcerns("f", settings, "/cwd", "/home"); concerns != nil {
		t.Errorf("expected nil, got %v", concerns)
	}
}

func readLocalSettings(t *testing.T, dir string) map[string]any {
	t.Helper()
	settings, err := loadSettings(filepath.Join(dir, ".claude", "settings.local.json"))
	if err != nil {
		t.Fatal(err)
	}
	return settings
}

func writeLocalSettings(t *testing.T, dir, content string) string {
	t.Helper()
	if err := os.MkdirAll(filepath.Join(dir, ".claude"), 0o755); err != nil {
		t.Fatal(err)
	}
	path := filepath.Join(dir, ".claude", "settings.local.json")
	if err := os.WriteFile(path, []byte(content), 0o644); err != nil {
		t.Fatal(err)
	}
	return path
}

func TestEnsureLocalSandboxSettingsCreate(t *testing.T) {
	dir := t.TempDir()
	chdir(t, dir)

	if err := ensureLocalSandboxSettings(); err != nil {
		t.Fatal(err)
	}

	settings := readLocalSettings(t, dir)
	fs := settings["sandbox"].(map[string]any)["filesystem"].(map[string]any)
	want := []any{"."}
	if !reflect.DeepEqual(fs["allowRead"], want) {
		t.Errorf("allowRead = %v, want %v", fs["allowRead"], want)
	}
	if !reflect.DeepEqual(fs["allowWrite"], want) {
		t.Errorf("allowWrite = %v, want %v", fs["allowWrite"], want)
	}
}

func TestEnsureLocalSandboxSettingsPreservesExistingKeys(t *testing.T) {
	dir := t.TempDir()
	chdir(t, dir)
	writeLocalSettings(t, dir, `{"permissions":{"allow":["Bash(go build:*)"]},"someId":9007199254740993}`)

	if err := ensureLocalSandboxSettings(); err != nil {
		t.Fatal(err)
	}

	settings := readLocalSettings(t, dir)
	if _, ok := settings["permissions"]; !ok {
		t.Error("permissions was dropped")
	}
	if _, ok := settings["sandbox"].(map[string]any)["filesystem"]; !ok {
		t.Error("sandbox.filesystem was not added")
	}
	data, err := os.ReadFile(filepath.Join(dir, ".claude", "settings.local.json"))
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(string(data), "9007199254740993") {
		t.Errorf("large integer was corrupted: %s", data)
	}
}

func TestEnsureLocalSandboxSettingsSkipsCustomFilesystem(t *testing.T) {
	dir := t.TempDir()
	chdir(t, dir)
	existing := `{"sandbox":{"filesystem":{"allowRead":["/custom"]}}}`
	path := writeLocalSettings(t, dir, existing)

	if err := ensureLocalSandboxSettings(); err != nil {
		t.Fatal(err)
	}

	data, err := os.ReadFile(path)
	if err != nil {
		t.Fatal(err)
	}
	if string(data) != existing {
		t.Errorf("file was modified: %s", data)
	}
}

func TestEnsureLocalSandboxSettingsUpdatesStalePath(t *testing.T) {
	dir := t.TempDir()
	chdir(t, dir)
	writeLocalSettings(t, dir, `{"sandbox":{"filesystem":{"allowRead":["/old/path"],"allowWrite":["/old/path"]}}}`)

	if err := ensureLocalSandboxSettings(); err != nil {
		t.Fatal(err)
	}

	settings := readLocalSettings(t, dir)
	fs := settings["sandbox"].(map[string]any)["filesystem"].(map[string]any)
	want := []any{"."}
	if !reflect.DeepEqual(fs["allowRead"], want) {
		t.Errorf("allowRead = %v, want %v", fs["allowRead"], want)
	}
	if !reflect.DeepEqual(fs["allowWrite"], want) {
		t.Errorf("allowWrite = %v, want %v", fs["allowWrite"], want)
	}
}

func TestEnsureLocalSandboxSettingsInvalidSandbox(t *testing.T) {
	dir := t.TempDir()
	chdir(t, dir)
	writeLocalSettings(t, dir, `{"sandbox":true}`)

	if err := ensureLocalSandboxSettings(); err == nil {
		t.Error("expected error for non-object sandbox value")
	}
}

func TestEnsureLocalSandboxSettingsSkipsHomeDir(t *testing.T) {
	dir := t.TempDir()
	chdir(t, dir)
	cwd, err := os.Getwd()
	if err != nil {
		t.Fatal(err)
	}
	t.Setenv("HOME", cwd)

	if err := ensureLocalSandboxSettings(); err != nil {
		t.Fatal(err)
	}

	if _, err := os.Stat(filepath.Join(dir, ".claude", "settings.local.json")); !os.IsNotExist(err) {
		t.Error("settings.local.json should not be created in the home directory")
	}
}

func writeTestFiles(t *testing.T, dir string, files []string) {
	t.Helper()
	for _, f := range files {
		path := filepath.Join(dir, f)
		if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
			t.Fatal(err)
		}
		if err := os.WriteFile(path, nil, 0o644); err != nil {
			t.Fatal(err)
		}
	}
}

func TestFindAutoLoadedFiles(t *testing.T) {
	dir := t.TempDir()
	writeTestFiles(t, dir, []string{
		".claude/settings.json",
		".claude/settings.local.json",
		".claude/CLAUDE.md",
		".claude/commands/deploy.md",
		".claude/rules/style.md",
		".mcp.json",
		"CLAUDE.md",
		"apps/web/.claude/settings.json",
		"apps/web/.claude/skills/deploy/SKILL.md",
		"apps/web/CLAUDE.local.md",
		"docs/CLAUDE.md",
		".git/CLAUDE.md",
		"src/main.go",
		"README.md",
	})

	files, err := findAutoLoadedFiles(dir)
	if err != nil {
		t.Fatal(err)
	}
	want := []string{
		".claude/CLAUDE.md",
		".claude/commands/deploy.md",
		".claude/rules/style.md",
		".mcp.json",
		"CLAUDE.md",
		"apps/web/.claude/settings.json",
		"apps/web/.claude/skills/deploy/SKILL.md",
		"apps/web/CLAUDE.local.md",
		"docs/CLAUDE.md",
	}
	if !reflect.DeepEqual(files, want) {
		t.Errorf("files = %v, want %v", files, want)
	}
}

func TestFindAutoLoadedFilesSettingsOnly(t *testing.T) {
	dir := t.TempDir()
	writeTestFiles(t, dir, []string{
		".claude/settings.json",
		".claude/settings.local.json",
		"src/main.go",
	})

	files, err := findAutoLoadedFiles(dir)
	if err != nil {
		t.Fatal(err)
	}
	if len(files) != 0 {
		t.Errorf("expected no files, got %v", files)
	}
}

func TestFindAutoLoadedFilesEmpty(t *testing.T) {
	files, err := findAutoLoadedFiles(t.TempDir())
	if err != nil {
		t.Fatal(err)
	}
	if files != nil {
		t.Errorf("expected nil, got %v", files)
	}
}

func TestFindAutoLoadedFilesClaudeNotDir(t *testing.T) {
	dir := t.TempDir()
	writeTestFiles(t, dir, []string{".claude"})

	files, err := findAutoLoadedFiles(dir)
	if err != nil {
		t.Fatal(err)
	}
	if !reflect.DeepEqual(files, []string{".claude"}) {
		t.Errorf("files = %v, want [.claude]", files)
	}
}

func TestManagedFilesystemPath(t *testing.T) {
	tests := []struct {
		name string
		fs   any
		want string
		ok   bool
	}{
		{"managed", map[string]any{"allowRead": []any{"/p"}, "allowWrite": []any{"/p"}}, "/p", true},
		{"differentPaths", map[string]any{"allowRead": []any{"/a"}, "allowWrite": []any{"/b"}}, "", false},
		{"extraKey", map[string]any{"allowRead": []any{"/p"}, "allowWrite": []any{"/p"}, "denyRead": []any{"/x"}}, "", false},
		{"multipleEntries", map[string]any{"allowRead": []any{"/p", "/q"}, "allowWrite": []any{"/p"}}, "", false},
		{"readOnly", map[string]any{"allowRead": []any{"/p"}}, "", false},
		{"notMap", true, "", false},
	}
	for _, tt := range tests {
		got, ok := managedFilesystemPath(tt.fs)
		if got != tt.want || ok != tt.ok {
			t.Errorf("%s: managedFilesystemPath = (%q, %v), want (%q, %v)", tt.name, got, ok, tt.want, tt.ok)
		}
	}
}
