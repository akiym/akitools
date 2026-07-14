package cmdsbx

import (
	"os"
	"os/exec"
	"slices"
	"testing"
)

func quietStderr(t *testing.T) {
	t.Helper()
	orig := os.Stderr
	devnull, err := os.OpenFile(os.DevNull, os.O_WRONLY, 0)
	if err != nil {
		t.Fatal(err)
	}
	os.Stderr = devnull
	t.Cleanup(func() {
		os.Stderr = orig
		devnull.Close()
	})
}

// stubExecCommand records docker invocations without running docker.
func stubExecCommand(t *testing.T, calls *[][]string) {
	t.Helper()
	orig := execCommand
	execCommand = func(name string, args ...string) *exec.Cmd {
		*calls = append(*calls, append([]string{name}, args...))
		return exec.Command("true")
	}
	t.Cleanup(func() { execCommand = orig })
}

func TestMainDoRejectsUnsafeFlags(t *testing.T) {
	quietStderr(t)
	for _, args := range [][]string{
		{"do", "--rootfs", "/x", "--", "true"},
		{"do", "--mount", "/a:/b", "--", "true"},
		{"do", "--allow-egress", "--", "true"},
		{"do", "--write", "--", "true"},
		{"do", "--persist-dir", "/p", "--", "true"},
		{"do", "--overlay-dir", "/o", "--", "true"},
		// removed entirely; guards against reintroducing a passthrough
		{"do", "--docker-arg", "--privileged", "--", "true"},
	} {
		if code := Main(args); code != 2 {
			t.Errorf("Main(%q) = %d, want 2", args, code)
		}
	}
}

// TestMainDoBuildsIsolatedCommand pins the exact docker argv `cmdsbx do`
// produces: any new argument — however it gets populated — must show up
// here and be justified against the safety contract.
func TestMainDoBuildsIsolatedCommand(t *testing.T) {
	var calls [][]string
	stubExecCommand(t, &calls)
	code := Main([]string{"do", "--image", "alpine:3", "--env", "A=1", "--workdir", "/w", "--", "echo", "hi"})
	if code != 0 {
		t.Fatalf("exit = %d, want 0", code)
	}
	if len(calls) != 1 {
		t.Fatalf("docker invocations = %d, want 1", len(calls))
	}
	want := []string{
		"docker", "run", "--init", "--label", "sandbox.managed=1", "--rm", "-i",
		"--network", "none", "--pull", "never",
		"-w", "/w", "-e", "A=1",
		"alpine:3", "echo", "hi",
	}
	if !slices.Equal(calls[0], want) {
		t.Errorf("got  %q\nwant %q", calls[0], want)
	}
}

func TestMainDoUsesPullNever(t *testing.T) {
	var calls [][]string
	stubExecCommand(t, &calls)
	code := Main([]string{"do", "--", "true"})
	if code != 0 {
		t.Fatalf("exit = %d, want 0", code)
	}
	if !containsPair(calls[0], "--pull", "never") {
		t.Errorf("do must pass --pull never to prevent implicit image pulls: %q", calls[0])
	}
}

func TestMainUnsafeDoesNotUsePullNever(t *testing.T) {
	var calls [][]string
	stubExecCommand(t, &calls)
	code := Main([]string{"unsafe", "--", "true"})
	if code != 0 {
		t.Fatalf("exit = %d, want 0", code)
	}
	if containsPair(calls[0], "--pull", "never") {
		t.Errorf("unsafe should still allow implicit pulls: %q", calls[0])
	}
}

func TestMainUnsafeRootfsMounts(t *testing.T) {
	var calls [][]string
	stubExecCommand(t, &calls)
	dir := t.TempDir()
	code := Main([]string{"unsafe", "--rootfs", dir, "--", "ls"})
	if code != 0 {
		t.Fatalf("exit = %d, want 0", code)
	}
	if len(calls) != 1 {
		t.Fatalf("docker invocations = %d, want 1", len(calls))
	}
	got := calls[0]
	if !containsPair(got, "-v", dir+":"+dir+":ro") {
		t.Errorf("missing read-only rootfs mount: %q", got)
	}
	if !containsPair(got, "-w", dir) {
		t.Errorf("workdir should default to rootfs: %q", got)
	}
}

func TestMainUnsafeAcceptsIsolationFlags(t *testing.T) {
	var calls [][]string
	stubExecCommand(t, &calls)
	code := Main([]string{"unsafe", "--allow-egress", "--env", "A=1", "--", "echo", "hi"})
	if code != 0 {
		t.Fatalf("exit = %d, want 0", code)
	}
	if len(calls) != 1 {
		t.Fatalf("docker invocations = %d, want 1", len(calls))
	}
	got := calls[0]
	if containsPair(got, "--network", "none") {
		t.Errorf("--allow-egress should drop --network none: %q", got)
	}
	if !slices.Contains(got, "--rm") {
		t.Errorf("unsafe stays ephemeral: %q", got)
	}
}
