package cmdsbx

import (
	"errors"
	"fmt"
	"io"
	"os"
	"os/exec"
	"strings"
)

const (
	namePrefix      = "sandbox-"
	managedLabel    = "sandbox.managed=1"
	sessionLabelKey = "sandbox.session"
	overlayBase     = "/.sandbox/overlay"
	keepAliveShell  = "while :; do sleep 3600; done"
)

func containerName(id string) string {
	return namePrefix + id
}

// BuildRunArgs builds the docker CLI arguments for `cmdsbx do`,
// `cmdsbx unsafe`, and `cmdsbx run`. All paths in o must already be
// absolute; only `unsafe` and `run` can populate the isolation-weakening
// fields.
func BuildRunArgs(o *RunOptions) ([]string, error) {
	if o.Image == "" {
		return nil, errors.New("no image specified")
	}
	if o.ID == "" && len(o.Command) == 0 {
		return nil, errors.New("no command specified")
	}
	// docker's -v syntax splits on ':', so colon-containing paths cannot
	// be expressed; reject them upfront instead of surfacing docker's
	// "invalid volume specification".
	for _, p := range []struct{ name, path string }{
		{"--rootfs", o.Rootfs},
		{"--persist-dir", o.PersistDir},
		{"--overlay-dir", o.OverlayDir},
	} {
		if strings.Contains(p.path, ":") {
			return nil, fmt.Errorf("%s path %q must not contain ':'", p.name, p.path)
		}
	}
	if o.OverlayDir != "" {
		if o.Rootfs == "" {
			return nil, errors.New("--overlay-dir requires --rootfs")
		}
		if o.Write {
			return nil, errors.New("--overlay-dir and --write are mutually exclusive")
		}
		// These characters cannot be escaped portably in overlayfs mount options.
		if strings.ContainsAny(o.Rootfs, ",'") {
			return nil, fmt.Errorf("rootfs path %q must not contain ',' or ''' when using --overlay-dir", o.Rootfs)
		}
	}
	if err := validateEnv(o.Env); err != nil {
		return nil, err
	}
	if err := validateWorkdir(o.Workdir); err != nil {
		return nil, err
	}

	args := []string{"run", "--init", "--label", managedLabel}
	if o.ID == "" {
		args = append(args, "--rm", "-i")
	} else {
		if err := validateID(o.ID); err != nil {
			return nil, err
		}
		args = append(args, "--name", containerName(o.ID), "--label", sessionLabelKey+"="+o.ID)
		if o.Detach {
			args = append(args, "-d")
		} else {
			args = append(args, "-i")
		}
	}
	if !o.AllowEgress {
		args = append(args, "--network", "none")
	}
	if o.NoPull {
		args = append(args, "--pull", "never")
	}

	mountMode := "ro"
	if o.Write {
		mountMode = "rw"
	}
	if o.Rootfs != "" {
		args = append(args, "-v", o.Rootfs+":"+o.Rootfs+":"+mountMode)
	}
	for _, m := range o.Mounts {
		mode := m.Mode
		if mode == "" {
			mode = mountMode
		}
		args = append(args, "-v", m.Source+":"+m.Target+":"+mode)
	}
	if o.PersistDir != "" {
		args = append(args, "-v", o.PersistDir+":"+o.PersistDir+":rw")
	}
	if o.OverlayDir != "" {
		args = append(args,
			"-v", o.OverlayDir+":"+overlayBase+":rw",
			"--cap-add", "SYS_ADMIN",
			"--security-opt", "apparmor=unconfined",
		)
	}

	workdir := o.Workdir
	if workdir == "" && o.Rootfs != "" {
		workdir = o.Rootfs
	}
	if workdir != "" {
		args = append(args, "-w", workdir)
	}
	for _, e := range o.Env {
		args = append(args, "-e", e)
	}
	args = append(args, o.Image)

	command := o.Command
	if len(command) == 0 {
		command = []string{"/bin/sh", "-c", keepAliveShell}
	}
	if o.OverlayDir != "" {
		command = append([]string{"/bin/sh", "-c", overlayScript(o.Rootfs), "sandbox"}, command...)
	}
	return append(args, command...), nil
}

// overlayScript mounts an overlayfs over the rootfs path so that writes
// land in the overlay directory instead of the host, then execs the
// sandboxed command. The cwd set by docker -w still references the
// pre-mount read-only bind mount, so re-enter it to resolve into the
// overlay before exec.
func overlayScript(rootfs string) string {
	q := shellQuote(rootfs)
	return fmt.Sprintf(
		"set -e; mkdir -p %[1]s/upper %[1]s/work; "+
			"mount -t overlay overlay -o lowerdir=%[2]s,upperdir=%[1]s/upper,workdir=%[1]s/work %[2]s; "+
			`cd "$PWD"; exec "$@"`,
		overlayBase, q)
}

func shellQuote(s string) string {
	return "'" + strings.ReplaceAll(s, "'", `'\''`) + "'"
}

// BuildExecArgs builds the docker CLI arguments for `cmdsbx exec`.
func BuildExecArgs(o *ExecOptions) ([]string, error) {
	if err := validateID(o.ID); err != nil {
		return nil, err
	}
	if len(o.Command) == 0 {
		return nil, errors.New("no command specified")
	}
	if err := validateEnv(o.Env); err != nil {
		return nil, err
	}
	if err := validateWorkdir(o.Workdir); err != nil {
		return nil, err
	}
	args := []string{"exec", "-i"}
	if o.Workdir != "" {
		args = append(args, "-w", o.Workdir)
	}
	for _, e := range o.Env {
		args = append(args, "-e", e)
	}
	args = append(args, containerName(o.ID))
	return append(args, o.Command...), nil
}

// execCommand is swapped out by tests; the docker binary is deliberately
// not configurable at runtime.
var execCommand = exec.Command

// runDocker runs docker with the given arguments, wiring stdio through,
// and returns the exit code to propagate.
func runDocker(args []string, stdin bool) int {
	cmd := execCommand("docker", args...)
	cmd.Stdout = os.Stdout
	cmd.Stderr = os.Stderr
	if stdin {
		cmd.Stdin = os.Stdin
	}
	return exitCode(cmd.Run())
}

// runDockerQuiet runs docker discarding stdout (e.g. `docker rm` echoes
// container names).
func runDockerQuiet(args []string) int {
	cmd := execCommand("docker", args...)
	cmd.Stdout = io.Discard
	cmd.Stderr = os.Stderr
	return exitCode(cmd.Run())
}

func dockerOutput(args []string) (string, int) {
	cmd := execCommand("docker", args...)
	cmd.Stderr = os.Stderr
	out, err := cmd.Output()
	return string(out), exitCode(err)
}

func exitCode(err error) int {
	if err == nil {
		return 0
	}
	var ee *exec.ExitError
	if errors.As(err, &ee) {
		return ee.ExitCode()
	}
	fmt.Fprintf(os.Stderr, "cmdsbx: %v\n", err)
	return 125
}
