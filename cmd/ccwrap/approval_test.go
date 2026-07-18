package ccwrap

import (
	"os"
	"path/filepath"
	"reflect"
	"testing"
)

func TestSaveLoadApproval(t *testing.T) {
	t.Setenv("XDG_STATE_HOME", t.TempDir())
	cwd := "/Users/akiym/src/proj"
	files := map[string]string{
		"CLAUDE.md":                      "abc",
		".claude/skills/deploy/SKILL.md": "def",
	}

	if err := saveApproval(cwd, files); err != nil {
		t.Fatal(err)
	}

	saved, err := loadApproval(cwd)
	if err != nil {
		t.Fatal(err)
	}
	if saved == nil {
		t.Fatal("expected approval, got nil")
	}
	if saved.Path != cwd {
		t.Errorf("path = %q, want %q", saved.Path, cwd)
	}
	if !reflect.DeepEqual(saved.Files, files) {
		t.Errorf("files = %v, want %v", saved.Files, files)
	}
	if saved.ApprovedAt.IsZero() {
		t.Error("approvedAt is zero")
	}
}

func TestLoadApprovalMissing(t *testing.T) {
	t.Setenv("XDG_STATE_HOME", t.TempDir())

	saved, err := loadApproval("/no/such/workspace")
	if err != nil {
		t.Fatal(err)
	}
	if saved != nil {
		t.Errorf("expected nil, got %+v", saved)
	}
}

func TestLoadApprovalCorrupt(t *testing.T) {
	t.Setenv("XDG_STATE_HOME", t.TempDir())
	cwd := "/Users/akiym/src/proj"
	path, err := approvalPath(cwd)
	if err != nil {
		t.Fatal(err)
	}
	writeTestFiles(t, filepath.Dir(path), []string{filepath.Base(path)})
	if err := os.WriteFile(path, []byte("not json"), 0o644); err != nil {
		t.Fatal(err)
	}

	saved, err := loadApproval(cwd)
	if err != nil {
		t.Fatal(err)
	}
	if saved != nil {
		t.Errorf("expected nil for corrupt record, got %+v", saved)
	}
}

func TestLoadApprovalPathMismatch(t *testing.T) {
	t.Setenv("XDG_STATE_HOME", t.TempDir())
	// ワークスペース名の"/"→"-"変換が衝突した場合を想定し、
	// 記録内のPathが一致しなければ無視されることを確認する
	if err := saveApproval("/a/b-c", map[string]string{"CLAUDE.md": "x"}); err != nil {
		t.Fatal(err)
	}

	saved, err := loadApproval("/a-b/c")
	if err != nil {
		t.Fatal(err)
	}
	if saved != nil {
		t.Errorf("expected nil for path mismatch, got %+v", saved)
	}
}

func TestDiffApproval(t *testing.T) {
	saved := &approval{Files: map[string]string{
		"CLAUDE.md":               "same",
		".claude/settings.json":   "old",
		".claude/skills/x/run.sh": "gone",
	}}
	current := map[string]string{
		"CLAUDE.md":             "same",
		".claude/settings.json": "new",
		".mcp.json":             "added",
	}

	added, changed, removed := diffApproval(saved, current)
	if want := []string{".mcp.json"}; !reflect.DeepEqual(added, want) {
		t.Errorf("added = %v, want %v", added, want)
	}
	if want := []string{".claude/settings.json"}; !reflect.DeepEqual(changed, want) {
		t.Errorf("changed = %v, want %v", changed, want)
	}
	if want := []string{".claude/skills/x/run.sh"}; !reflect.DeepEqual(removed, want) {
		t.Errorf("removed = %v, want %v", removed, want)
	}
}

func TestDiffApprovalNoRecord(t *testing.T) {
	current := map[string]string{"CLAUDE.md": "x"}

	added, changed, removed := diffApproval(nil, current)
	if want := []string{"CLAUDE.md"}; !reflect.DeepEqual(added, want) {
		t.Errorf("added = %v, want %v", added, want)
	}
	if len(changed) != 0 || len(removed) != 0 {
		t.Errorf("changed = %v, removed = %v, want empty", changed, removed)
	}
}

func TestConfirmationTargets(t *testing.T) {
	dir := t.TempDir()
	writeTestFiles(t, dir, []string{
		".claude/settings.json",
		".claude/settings.local.json",
		"CLAUDE.md",
		".claude/skills/deploy/SKILL.md",
		".claude/skills/deploy/run.sh",
	})
	autoLoaded := []string{
		"CLAUDE.md",
		".claude/skills/deploy/SKILL.md",
		".claude/skills/deploy/run.sh",
	}

	current, err := confirmationTargets(dir, autoLoaded, false)
	if err != nil {
		t.Fatal(err)
	}
	if got := sortedKeys(current); !reflect.DeepEqual(got, []string{
		".claude/skills/deploy/SKILL.md",
		".claude/skills/deploy/run.sh",
		"CLAUDE.md",
	}) {
		t.Errorf("targets = %v", got)
	}

	current, err = confirmationTargets(dir, autoLoaded, true)
	if err != nil {
		t.Fatal(err)
	}
	if got := sortedKeys(current); !reflect.DeepEqual(got, []string{
		".claude/settings.json",
		".claude/settings.local.json",
		".claude/skills/deploy/SKILL.md",
		".claude/skills/deploy/run.sh",
		"CLAUDE.md",
	}) {
		t.Errorf("targets = %v", got)
	}
}

func TestConfirmationTargetsMissingSettings(t *testing.T) {
	dir := t.TempDir()
	writeTestFiles(t, dir, []string{"CLAUDE.md"})

	current, err := confirmationTargets(dir, []string{"CLAUDE.md"}, true)
	if err != nil {
		t.Fatal(err)
	}
	if got := sortedKeys(current); !reflect.DeepEqual(got, []string{"CLAUDE.md"}) {
		t.Errorf("targets = %v", got)
	}
}
