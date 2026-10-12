package ccwrap

import (
	"context"
	"fmt"
	"net"
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

	ctx, cancel := context.WithTimeout(t.Context(), 30*time.Second)
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

func TestWaitForListenParsesMitmdumpLine(t *testing.T) {
	r, w, err := os.Pipe()
	if err != nil {
		t.Fatal(err)
	}
	go func() {
		defer w.Close()
		fmt.Fprintln(w, "[01:42:00.706] Loading script foo")
		fmt.Fprintln(w, "[01:42:00.706] reverse proxy to https://api.anthropic.com listening at 127.0.0.1:58527.")
		fmt.Fprintln(w, "[01:42:01.000] client connect")
	}()
	port, err := waitForListen(r, time.Second)
	if err != nil {
		t.Fatal(err)
	}
	if port != 58527 {
		t.Errorf("port = %d, want 58527", port)
	}
}

func TestWaitForListenFailsOnExit(t *testing.T) {
	r, w, err := os.Pipe()
	if err != nil {
		t.Fatal(err)
	}
	go func() {
		defer w.Close()
		fmt.Fprintln(w, "[01:42:00.706] Error: address already in use")
	}()
	if _, err := waitForListen(r, time.Second); err == nil {
		t.Fatal("expected an error when mitmdump exits without listening")
	}
}

func TestWaitForListenTimesOut(t *testing.T) {
	r, w, err := os.Pipe()
	if err != nil {
		t.Fatal(err)
	}
	defer w.Close()
	if _, err := waitForListen(r, 50*time.Millisecond); err == nil {
		t.Fatal("expected a timeout")
	}
}

// Needs a real mitmdump on PATH; the port it reports must be one it
// actually bound, so a connection to it must succeed.
func TestStartMitmdumpReportsBoundPort(t *testing.T) {
	if _, err := exec.LookPath("mitmdump"); err != nil {
		t.Skip("mitmdump not installed")
	}
	mitmFile := filepath.Join(t.TempDir(), "x.mitm")
	proc, err := startMitmdump(mitmFile)
	if err != nil {
		t.Fatal(err)
	}
	defer proc.stop()
	if proc.port <= 0 {
		t.Fatalf("port = %d", proc.port)
	}
	conn, err := net.Dial("tcp", fmt.Sprintf("127.0.0.1:%d", proc.port))
	if err != nil {
		t.Fatalf("dial reported port: %v", err)
	}
	conn.Close()
}
