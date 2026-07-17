package cmdsbx

import (
	"errors"
	"flag"
	"fmt"
	"os"
	"path/filepath"
	"slices"
	"strings"
	"text/tabwriter"
)

const usageText = `Usage: cmdsbx <command> [options]

Run commands in disposable Docker containers, isolated from the host.

Commands:
  do      Run a command in an ephemeral sandbox (no mounts, no network)
  broker  Serve 'do' requests for sandboxed clients over a unix socket
  unsafe  Like do, but allows isolation-weakening options; never allow unconditionally
  run     Create a sandbox session
  exec    Run a command in an existing sandbox session
  delete  Delete sandbox sessions
  list    List sandbox sessions

Run 'cmdsbx <command> -h' for command-specific options.
`

// Main is the CLI entry point. It returns the process exit code.
func Main(args []string) int {
	if len(args) == 0 {
		fmt.Fprint(os.Stderr, usageText)
		return 2
	}
	switch args[0] {
	case "do":
		return cmdDo(args[1:])
	case "broker":
		return cmdBroker(args[1:])
	case "unsafe":
		return cmdUnsafe(args[1:])
	case "run":
		return cmdRun(args[1:])
	case "exec":
		return cmdExec(args[1:])
	case "delete", "rm":
		return cmdDelete(args[1:])
	case "list", "ls":
		return cmdList(args[1:])
	case "help", "-h", "--help":
		fmt.Print(usageText)
		return 0
	default:
		fmt.Fprintf(os.Stderr, "cmdsbx: unknown command %q\n\n", args[0])
		fmt.Fprint(os.Stderr, usageText)
		return 2
	}
}

type stringList []string

func (s *stringList) String() string {
	return strings.Join(*s, ",")
}

func (s *stringList) Set(v string) error {
	*s = append(*s, v)
	return nil
}

const defaultImage = "ubuntu:24.04"

func newFlagSet(name string) *flag.FlagSet {
	fs := flag.NewFlagSet(name, flag.ContinueOnError)
	fs.SetOutput(os.Stderr)
	return fs
}

// addSafeFlags registers the flags `cmdsbx do` is allowed to expose;
// the isolation-weakening commands add theirs via addRunFlags.
func addSafeFlags(fs *flag.FlagSet, o *RunOptions) {
	fs.StringVar(&o.Image, "image", defaultImage, "container image")
	fs.StringVar(&o.Workdir, "workdir", "", "working directory inside the container")
	fs.Var((*stringList)(&o.Env), "env", "environment variable KEY=VALUE (repeatable)")
}

func addRunFlags(fs *flag.FlagSet, o *RunOptions, mounts *stringList) {
	addSafeFlags(fs, o)
	fs.BoolVar(&o.AllowEgress, "allow-egress", false, "allow sandbox network egress")
	fs.StringVar(&o.Rootfs, "rootfs", "", "host path exposed read-only at the same path in the sandbox (default working directory)")
	fs.StringVar(&o.PersistDir, "persist-dir", "", "host path mounted read-write for persistent state")
	fs.StringVar(&o.OverlayDir, "overlay-dir", "", "host path holding a writable overlay over --rootfs")
	fs.BoolVar(&o.Write, "write", false, "mount --rootfs and --mount paths read-write")
	fs.Var(mounts, "mount", "bind mount SRC:DST[:ro|rw] (repeatable)")
}

// finalizeRunOptions resolves paths against the host filesystem and applies
// the parsed --mount flags.
func finalizeRunOptions(o *RunOptions, mounts stringList) error {
	for _, m := range mounts {
		mount, err := ParseMount(m)
		if err != nil {
			return err
		}
		if _, err := os.Stat(mount.Source); err != nil {
			return fmt.Errorf("mount source: %w", err)
		}
		o.Mounts = append(o.Mounts, mount)
	}
	if o.Rootfs != "" {
		abs, err := filepath.Abs(o.Rootfs)
		if err != nil {
			return fmt.Errorf("resolve --rootfs: %w", err)
		}
		if abs == "/" {
			return errors.New("--rootfs / is not supported with Docker; mount a specific directory instead")
		}
		if _, err := os.Stat(abs); err != nil {
			return fmt.Errorf("--rootfs: %w", err)
		}
		o.Rootfs = abs
	}
	if err := ensureDir("--persist-dir", &o.PersistDir); err != nil {
		return err
	}
	return ensureDir("--overlay-dir", &o.OverlayDir)
}

// ensureDir absolutizes and creates a host directory named by an
// optional flag.
func ensureDir(name string, dir *string) error {
	if *dir == "" {
		return nil
	}
	abs, err := filepath.Abs(*dir)
	if err != nil {
		return fmt.Errorf("resolve %s: %w", name, err)
	}
	if err := os.MkdirAll(abs, 0o755); err != nil {
		return fmt.Errorf("%s: %w", name, err)
	}
	*dir = abs
	return nil
}

// splitDashDash splits args at the first standalone "--" so that the
// command part is never exposed to flag parsing.
func splitDashDash(args []string) (before, command []string) {
	for i, a := range args {
		if a == "--" {
			return args[:i], args[i+1:]
		}
	}
	return args, nil
}

// splitID pops a leading non-flag argument so that both
// `cmdsbx run ID --flag` and `cmdsbx run --flag ID` parse.
func splitID(args []string) (string, []string) {
	if len(args) > 0 && !strings.HasPrefix(args[0], "-") {
		return args[0], args[1:]
	}
	return "", args
}

// parseSession parses `<id> [flags] [-- command]` and
// `[flags] <id> [-- command]` argument forms.
func parseSession(fs *flag.FlagSet, args []string) (id string, command []string, exit int, ok bool) {
	before, tail := splitDashDash(args)
	id, rest := splitID(before)
	if err := fs.Parse(rest); err != nil {
		return "", nil, parseExit(err), false
	}
	rem := fs.Args()
	if id == "" {
		if len(rem) == 0 {
			fs.Usage()
			return "", nil, 2, false
		}
		id, rem = rem[0], rem[1:]
	}
	return id, slices.Concat(rem, tail), 0, true
}

func parseExit(err error) int {
	if errors.Is(err, flag.ErrHelp) {
		return 0
	}
	return 2
}

// buildAndRun is the shared tail of do/unsafe/run: build the docker
// argv and execute it. stdin says whether os.Stdin is attached.
func buildAndRun(o *RunOptions, stdin bool) int {
	dockerArgs, err := BuildRunArgs(o)
	if err != nil {
		return fail(err)
	}
	return runDocker(dockerArgs, stdin)
}

func fail(err error) int {
	fmt.Fprintf(os.Stderr, "cmdsbx: %v\n", err)
	return 2
}

// parseEphemeral parses `[options] -- <command...>` for do/unsafe.
func parseEphemeral(fs *flag.FlagSet, args []string, o *RunOptions) (exit int, ok bool) {
	before, tail := splitDashDash(args)
	if err := fs.Parse(before); err != nil {
		return parseExit(err), false
	}
	o.Command = slices.Concat(fs.Args(), tail)
	if len(o.Command) == 0 {
		fs.Usage()
		return 2, false
	}
	return 0, true
}

// cmdDo is the safe ephemeral runner: no mounts, no network, no
// passthrough. It must stay safe to allow unconditionally in agent
// permission settings; isolation-weakening flags belong to cmdUnsafe.
// The only host exposure it can request is --mount-cwd-ro, which never
// runs docker directly: the mount happens only when a broker validates
// the directory against its own allow-list.
func cmdDo(args []string) int {
	var o RunOptions
	var mountCwd bool
	fs := newFlagSet("cmdsbx do")
	fs.Usage = func() {
		fmt.Fprint(os.Stderr, "Usage: cmdsbx do [options] -- <command...>\n\nRuns the command with no mounts and no network. --mount-cwd-ro exposes\nthe current directory read-only at the same path, but only through a\nbroker that allows it. Anything more requires 'cmdsbx unsafe'. When a\nbroker daemon (cmdsbx broker) is reachable, the run is delegated to it.\n\nOptions:\n")
		fs.PrintDefaults()
	}
	addSafeFlags(fs, &o)
	fs.BoolVar(&o.Interactive, "i", false, "stream stdin into the sandbox (pipes, heredocs, -i -- bash)")
	fs.BoolVar(&mountCwd, "mount-cwd-ro", false, "mount the current directory read-only at the same path (requires a broker that allows it)")
	if exit, ok := parseEphemeral(fs, args, &o); !ok {
		return exit
	}
	if mountCwd {
		cwd, err := os.Getwd()
		if err != nil {
			return fail(err)
		}
		resolved, err := filepath.EvalSymlinks(cwd)
		if err != nil {
			return fail(err)
		}
		o.Rootfs = resolved
	}
	o.NoPull = true
	conn := dialBroker()
	if conn == nil {
		if mountCwd {
			return fail(errors.New("--mount-cwd-ro requires a running broker (ccwrap or cmdsbx broker -allow-rootfs)"))
		}
		return buildAndRun(&o, o.Interactive)
	}
	return runViaBroker(conn, &o)
}

func cmdUnsafe(args []string) int {
	var o RunOptions
	var mounts stringList
	fs := newFlagSet("cmdsbx unsafe")
	fs.Usage = func() {
		fmt.Fprint(os.Stderr, "Usage: cmdsbx unsafe [options] -- <command...>\n\nLike 'cmdsbx do' but accepts isolation-weakening options: host\nmounts, host writes, and network egress. Never allow this command\nunconditionally in agent permission settings.\n\nOptions:\n")
		fs.PrintDefaults()
	}
	addRunFlags(fs, &o, &mounts)
	if exit, ok := parseEphemeral(fs, args, &o); !ok {
		return exit
	}
	if err := finalizeRunOptions(&o, mounts); err != nil {
		return fail(err)
	}
	return buildAndRun(&o, true)
}

func cmdRun(args []string) int {
	var o RunOptions
	var mounts stringList
	fs := newFlagSet("cmdsbx run")
	fs.Usage = func() {
		fmt.Fprint(os.Stderr, "Usage: cmdsbx run <id> [options] [-- <command...>]\n\nOptions:\n")
		fs.PrintDefaults()
	}
	fs.BoolVar(&o.Detach, "detach", false, "run the session in the background")
	addRunFlags(fs, &o, &mounts)
	id, command, exit, ok := parseSession(fs, args)
	if !ok {
		return exit
	}
	o.ID = id
	o.Command = command
	if err := finalizeRunOptions(&o, mounts); err != nil {
		return fail(err)
	}
	return buildAndRun(&o, !o.Detach)
}

func cmdExec(args []string) int {
	var o ExecOptions
	fs := newFlagSet("cmdsbx exec")
	fs.Usage = func() {
		fmt.Fprint(os.Stderr, "Usage: cmdsbx exec <id> [options] -- <command...>\n\nOptions:\n")
		fs.PrintDefaults()
	}
	fs.StringVar(&o.Workdir, "workdir", "", "working directory")
	fs.Var((*stringList)(&o.Env), "env", "environment variable KEY=VALUE (repeatable)")
	id, command, exit, ok := parseSession(fs, args)
	if !ok {
		return exit
	}
	o.ID = id
	o.Command = command
	execArgs, err := BuildExecArgs(&o)
	if err != nil {
		return fail(err)
	}
	return runDocker(execArgs, true)
}

func cmdDelete(args []string) int {
	if len(args) == 0 {
		fmt.Fprintln(os.Stderr, "Usage: cmdsbx delete <id>...")
		return 2
	}
	rc := 0
	names := make([]string, 0, len(args))
	for _, id := range args {
		if err := validateID(id); err != nil {
			fmt.Fprintf(os.Stderr, "cmdsbx: %v\n", err)
			rc = 1
			continue
		}
		names = append(names, containerName(id))
	}
	if len(names) > 0 {
		if code := runDockerQuiet(append([]string{"rm", "-f", "-v"}, names...)); code != 0 {
			rc = 1
		}
	}
	return rc
}

func cmdList(args []string) int {
	if len(args) > 0 {
		fmt.Fprintln(os.Stderr, "Usage: cmdsbx list")
		return 2
	}
	out, code := dockerOutput([]string{
		"ps", "-a",
		"--filter", "label=" + sessionLabelKey,
		"--format", `{{.Label "` + sessionLabelKey + `"}}\t{{.Image}}\t{{.Status}}`,
	})
	if code != 0 {
		return code
	}
	w := tabwriter.NewWriter(os.Stdout, 2, 8, 2, ' ', 0)
	fmt.Fprintln(w, "ID\tIMAGE\tSTATUS")
	for line := range strings.Lines(out) {
		if line = strings.TrimRight(line, "\n"); line != "" {
			fmt.Fprintln(w, line)
		}
	}
	_ = w.Flush()
	return 0
}
