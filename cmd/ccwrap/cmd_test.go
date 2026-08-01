package ccwrap

import (
	"context"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"
	"time"
)

func TestDataDir(t *testing.T) {
	base := t.TempDir()
	t.Setenv("XDG_DATA_HOME", base)

	dir, err := dataDir()
	if err != nil {
		t.Fatal(err)
	}
	if want := filepath.Join(base, "akitools", "ccwrap"); dir != want {
		t.Errorf("dataDir = %q, want %q", dir, want)
	}
}

func TestNeedsHerdrAgentHint(t *testing.T) {
	tests := []struct {
		name   string
		paneID string
		agent  string
		want   bool
	}{
		{name: "herdr pane without hint", paneID: "w6:p1", want: true},
		{name: "outside herdr", want: false},
		{name: "hint already set by caller", paneID: "w6:p1", agent: "codex", want: false},
		{name: "agent set outside herdr", agent: "claude", want: false},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Setenv("HERDR_PANE_ID", tt.paneID)
			t.Setenv("HERDR_AGENT", tt.agent)
			if got := needsHerdrAgentHint(); got != tt.want {
				t.Errorf("needsHerdrAgentHint() = %v, want %v", got, tt.want)
			}
		})
	}
}

func TestApplyHerdrAgentHintSkipsOutsideHerdr(t *testing.T) {
	t.Setenv("HERDR_PANE_ID", "")
	t.Setenv("HERDR_AGENT", "")
	// execしてしまうとテストプロセスが置き換わるので、戻ること自体が検証になる
	if err := applyHerdrAgentHint(); err != nil {
		t.Fatal(err)
	}
}

// TestApplyHerdrAgentHintExecs はテストバイナリ自身をexec対象にして、
// 実際に環境が差し替わること、そしてexec後は再execせず止まることを確かめる。
// 停止条件が壊れるとexecループになるのでtimeoutで打ち切る
func TestApplyHerdrAgentHintExecs(t *testing.T) {
	const marker = "CCWRAP_TEST_HERDR_EXEC"
	if os.Getenv(marker) == "1" {
		if err := applyHerdrAgentHint(); err != nil {
			fmt.Printf("apply failed: %v\n", err)
			return
		}
		fmt.Printf("result HERDR_AGENT=%q\n", os.Getenv("HERDR_AGENT"))
		return
	}

	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()
	cmd := exec.CommandContext(ctx, os.Args[0], "-test.run=^TestApplyHerdrAgentHintExecs$", "-test.v")
	cmd.Env = append(filterEnv(filterEnv(os.Environ(), "HERDR_AGENT"), "HERDR_PANE_ID"),
		marker+"=1",
		"HERDR_PANE_ID=w6:p1",
	)
	out, err := cmd.CombinedOutput()
	if ctx.Err() != nil {
		t.Fatalf("re-exec did not settle (exec loop?): %s", out)
	}
	if err != nil {
		t.Fatalf("child failed: %v\n%s", err, out)
	}
	if !strings.Contains(string(out), `result HERDR_AGENT="claude"`) {
		t.Errorf("hint was not applied across exec:\n%s", out)
	}
}

func TestDataDirMigratesOldLayout(t *testing.T) {
	base := t.TempDir()
	t.Setenv("XDG_DATA_HOME", base)
	oldDir := filepath.Join(base, "ccwrap")
	writeTestFiles(t, oldDir, []string{"ws/20240101-000000.har"})

	dir, err := dataDir()
	if err != nil {
		t.Fatal(err)
	}
	if _, err := os.Stat(filepath.Join(dir, "ws", "20240101-000000.har")); err != nil {
		t.Errorf("old data was not migrated: %v", err)
	}
	if _, err := os.Stat(oldDir); !os.IsNotExist(err) {
		t.Error("old dir still exists")
	}
}

func TestDataDirKeepsExistingNewLayout(t *testing.T) {
	base := t.TempDir()
	t.Setenv("XDG_DATA_HOME", base)
	writeTestFiles(t, base, []string{
		"ccwrap/old.har",
		"akitools/ccwrap/new.har",
	})

	dir, err := dataDir()
	if err != nil {
		t.Fatal(err)
	}
	if _, err := os.Stat(filepath.Join(dir, "new.har")); err != nil {
		t.Errorf("new layout data missing: %v", err)
	}
	if _, err := os.Stat(filepath.Join(base, "ccwrap", "old.har")); err != nil {
		t.Error("old dir should be left untouched when new dir exists")
	}
}
