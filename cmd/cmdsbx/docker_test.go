package cmdsbx

import (
	"slices"
	"strings"
	"testing"
)

func TestBuildRunArgsEphemeral(t *testing.T) {
	args, err := BuildRunArgs(&RunOptions{
		Image:   "ubuntu:24.04",
		Command: []string{"echo", "hello"},
	})
	if err != nil {
		t.Fatal(err)
	}
	want := []string{
		"run", "--init", "--label", "sandbox.managed=1", "--rm", "-i",
		"--network", "none",
		"ubuntu:24.04", "echo", "hello",
	}
	if !slices.Equal(args, want) {
		t.Errorf("got %q, want %q", args, want)
	}
}

func TestBuildRunArgsNoPull(t *testing.T) {
	args, err := BuildRunArgs(&RunOptions{
		Image:   "ubuntu:24.04",
		NoPull:  true,
		Command: []string{"true"},
	})
	if err != nil {
		t.Fatal(err)
	}
	if !containsPair(args, "--pull", "never") {
		t.Errorf("NoPull should emit --pull never: %q", args)
	}
}

func TestBuildRunArgsAllowEgress(t *testing.T) {
	args, err := BuildRunArgs(&RunOptions{
		Image:       "ubuntu:24.04",
		AllowEgress: true,
		Command:     []string{"true"},
	})
	if err != nil {
		t.Fatal(err)
	}
	if slices.Contains(args, "none") {
		t.Errorf("network should not be disabled: %q", args)
	}
}

func TestBuildRunArgsRootfs(t *testing.T) {
	args, err := BuildRunArgs(&RunOptions{
		Image:   "ubuntu:24.04",
		Rootfs:  "/work/proj",
		Command: []string{"true"},
	})
	if err != nil {
		t.Fatal(err)
	}
	if !containsPair(args, "-v", "/work/proj:/work/proj:ro") {
		t.Errorf("missing read-only rootfs mount: %q", args)
	}
	if !containsPair(args, "-w", "/work/proj") {
		t.Errorf("workdir should default to rootfs: %q", args)
	}
}

func TestBuildRunArgsWrite(t *testing.T) {
	args, err := BuildRunArgs(&RunOptions{
		Image:   "ubuntu:24.04",
		Rootfs:  "/work/proj",
		Write:   true,
		Mounts:  []Mount{{Source: "/data", Target: "/data"}, {Source: "/ro", Target: "/ro", Mode: "ro"}},
		Command: []string{"true"},
	})
	if err != nil {
		t.Fatal(err)
	}
	for _, v := range []string{"/work/proj:/work/proj:rw", "/data:/data:rw", "/ro:/ro:ro"} {
		if !containsPair(args, "-v", v) {
			t.Errorf("missing mount %q: %q", v, args)
		}
	}
}

func TestBuildRunArgsWorkdirOverride(t *testing.T) {
	args, err := BuildRunArgs(&RunOptions{
		Image:   "ubuntu:24.04",
		Rootfs:  "/work/proj",
		Workdir: "/tmp",
		Command: []string{"true"},
	})
	if err != nil {
		t.Fatal(err)
	}
	if !containsPair(args, "-w", "/tmp") {
		t.Errorf("explicit workdir not honored: %q", args)
	}
}

func TestBuildRunArgsPersistDirEnv(t *testing.T) {
	args, err := BuildRunArgs(&RunOptions{
		Image:      "ubuntu:24.04",
		PersistDir: "/state",
		Env:        []string{"FOO=bar"},
		Command:    []string{"true"},
	})
	if err != nil {
		t.Fatal(err)
	}
	if !containsPair(args, "-v", "/state:/state:rw") {
		t.Errorf("missing persist-dir mount: %q", args)
	}
	if !containsPair(args, "-e", "FOO=bar") {
		t.Errorf("missing env: %q", args)
	}
}

func TestBuildRunArgsOverlay(t *testing.T) {
	args, err := BuildRunArgs(&RunOptions{
		Image:      "ubuntu:24.04",
		Rootfs:     "/work/proj",
		OverlayDir: "/tmp/ovl",
		Command:    []string{"make", "test"},
	})
	if err != nil {
		t.Fatal(err)
	}
	if !containsPair(args, "-v", "/work/proj:/work/proj:ro") {
		t.Errorf("rootfs must stay read-only under overlay: %q", args)
	}
	if !containsPair(args, "-v", "/tmp/ovl:/.sandbox/overlay:rw") {
		t.Errorf("missing overlay mount: %q", args)
	}
	if !containsPair(args, "--cap-add", "SYS_ADMIN") {
		t.Errorf("overlay requires SYS_ADMIN: %q", args)
	}
	i := slices.Index(args, "ubuntu:24.04")
	if i < 0 || len(args) < i+7 {
		t.Fatalf("unexpected command tail: %q", args)
	}
	tail := args[i+1:]
	if tail[0] != "/bin/sh" || tail[1] != "-c" {
		t.Fatalf("overlay command should be wrapped in sh -c: %q", tail)
	}
	script := tail[2]
	for _, part := range []string{"mount -t overlay", "lowerdir='/work/proj'", "upperdir=/.sandbox/overlay/upper", `cd "$PWD"; exec "$@"`} {
		if !strings.Contains(script, part) {
			t.Errorf("overlay script missing %q: %s", part, script)
		}
	}
	if !slices.Equal(tail[4:], []string{"make", "test"}) {
		t.Errorf("wrapped command mismatch: %q", tail[4:])
	}
}

func TestBuildRunArgsOverlayErrors(t *testing.T) {
	base := RunOptions{Image: "ubuntu:24.04", Command: []string{"true"}}

	o := base
	o.OverlayDir = "/tmp/ovl"
	if _, err := BuildRunArgs(&o); err == nil {
		t.Error("overlay without rootfs should fail")
	}

	o = base
	o.OverlayDir = "/tmp/ovl"
	o.Rootfs = "/work/proj"
	o.Write = true
	if _, err := BuildRunArgs(&o); err == nil {
		t.Error("overlay with --write should fail")
	}

	o = base
	o.OverlayDir = "/tmp/ovl"
	o.Rootfs = "/work/pro,j"
	if _, err := BuildRunArgs(&o); err == nil {
		t.Error("overlay with ',' in rootfs should fail")
	}
}

func TestBuildRunArgsColonPaths(t *testing.T) {
	for _, o := range []RunOptions{
		{Image: "x", Rootfs: "/work/pro:j", Command: []string{"true"}},
		{Image: "x", PersistDir: "/tmp/a:b", Command: []string{"true"}},
		{Image: "x", OverlayDir: "/tmp/o:vl", Rootfs: "/work/proj", Command: []string{"true"}},
	} {
		if _, err := BuildRunArgs(&o); err == nil {
			t.Errorf("colon path should fail: %+v", o)
		}
	}
}

func TestBuildRunArgsStateful(t *testing.T) {
	args, err := BuildRunArgs(&RunOptions{
		ID:     "s1",
		Detach: true,
		Image:  "ubuntu:24.04",
	})
	if err != nil {
		t.Fatal(err)
	}
	if !containsPair(args, "--name", "sandbox-s1") {
		t.Errorf("missing container name: %q", args)
	}
	if !containsPair(args, "--label", "sandbox.session=s1") {
		t.Errorf("missing session label: %q", args)
	}
	if !slices.Contains(args, "-d") {
		t.Errorf("missing -d: %q", args)
	}
	if slices.Contains(args, "--rm") {
		t.Errorf("stateful session must not be --rm: %q", args)
	}
	if !slices.Contains(args, keepAliveShell) {
		t.Errorf("missing keep-alive command: %q", args)
	}
}

func TestBuildRunArgsErrors(t *testing.T) {
	if _, err := BuildRunArgs(&RunOptions{Image: "x"}); err == nil {
		t.Error("do without command should fail")
	}
	if _, err := BuildRunArgs(&RunOptions{Command: []string{"true"}}); err == nil {
		t.Error("missing image should fail")
	}
	if _, err := BuildRunArgs(&RunOptions{ID: "bad/id", Image: "x"}); err == nil {
		t.Error("invalid id should fail")
	}
	if _, err := BuildRunArgs(&RunOptions{Image: "x", Env: []string{"=v"}, Command: []string{"true"}}); err == nil {
		t.Error("invalid env should fail")
	}
	if _, err := BuildRunArgs(&RunOptions{Image: "x", Env: []string{"FOO"}, Command: []string{"true"}}); err == nil {
		t.Error("env without '=' should fail (would inherit the host value)")
	}
	if _, err := BuildRunArgs(&RunOptions{Image: "x", Workdir: "rel", Command: []string{"true"}}); err == nil {
		t.Error("relative workdir should fail")
	}
}

func TestSplitDashDash(t *testing.T) {
	before, cmd := splitDashDash([]string{"--workdir", "/x", "s1", "--", "ls", "-la"})
	if !slices.Equal(before, []string{"--workdir", "/x", "s1"}) || !slices.Equal(cmd, []string{"ls", "-la"}) {
		t.Errorf("got before=%q cmd=%q", before, cmd)
	}
	before, cmd = splitDashDash([]string{"echo", "hi"})
	if !slices.Equal(before, []string{"echo", "hi"}) || cmd != nil {
		t.Errorf("got before=%q cmd=%q", before, cmd)
	}
}

func TestBuildExecArgs(t *testing.T) {
	args, err := BuildExecArgs(&ExecOptions{
		ID:      "s1",
		Workdir: "/work",
		Env:     []string{"A=1"},
		Command: []string{"ls", "-la"},
	})
	if err != nil {
		t.Fatal(err)
	}
	want := []string{
		"exec", "-i", "-w", "/work", "-e", "A=1",
		"sandbox-s1", "ls", "-la",
	}
	if !slices.Equal(args, want) {
		t.Errorf("got %q, want %q", args, want)
	}
}

func TestBuildExecArgsErrors(t *testing.T) {
	if _, err := BuildExecArgs(&ExecOptions{ID: "s1"}); err == nil {
		t.Error("exec without command should fail")
	}
	if _, err := BuildExecArgs(&ExecOptions{ID: "", Command: []string{"true"}}); err == nil {
		t.Error("exec without id should fail")
	}
	if _, err := BuildExecArgs(&ExecOptions{ID: "s1", Workdir: "rel", Command: []string{"true"}}); err == nil {
		t.Error("relative workdir should fail")
	}
}

func TestParseMount(t *testing.T) {
	m, err := ParseMount("/src:/dst")
	if err != nil {
		t.Fatal(err)
	}
	if m.Source != "/src" || m.Target != "/dst" || m.Mode != "" {
		t.Errorf("unexpected mount: %+v", m)
	}

	m, err = ParseMount("/src:/dst:rw")
	if err != nil {
		t.Fatal(err)
	}
	if m.Mode != "rw" {
		t.Errorf("unexpected mode: %+v", m)
	}

	for _, s := range []string{"", "/src", "/src:/dst:bad", ":/dst", "/src:", "/src:rel"} {
		if _, err := ParseMount(s); err == nil {
			t.Errorf("ParseMount(%q) should fail", s)
		}
	}
}

func TestValidateID(t *testing.T) {
	for _, id := range []string{"a", "abc-123", "A.b_c"} {
		if err := validateID(id); err != nil {
			t.Errorf("validateID(%q) = %v", id, err)
		}
	}
	for _, id := range []string{"", "-a", ".a", "a b", "a/b", strings.Repeat("a", 65)} {
		if err := validateID(id); err == nil {
			t.Errorf("validateID(%q) should fail", id)
		}
	}
}

func containsPair(args []string, flag, value string) bool {
	for i := 0; i+1 < len(args); i++ {
		if args[i] == flag && args[i+1] == value {
			return true
		}
	}
	return false
}
