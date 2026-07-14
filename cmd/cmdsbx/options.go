package cmdsbx

import (
	"fmt"
	"path/filepath"
	"regexp"
	"strings"
)

// Mount is a bind mount from a host path into the sandbox.
type Mount struct {
	Source string
	Target string
	// Mode is "ro", "rw", or "" meaning "follow the --write flag".
	Mode string
}

// RunOptions configures `cmdsbx do` / `cmdsbx unsafe` (ID empty) and
// `cmdsbx run` (ID set).
type RunOptions struct {
	ID          string
	Detach      bool
	Image       string
	AllowEgress bool
	Rootfs      string
	Workdir     string
	PersistDir  string
	OverlayDir  string
	Write       bool
	// NoPull passes `--pull=never` so a missing image fails immediately
	// instead of triggering an implicit pull. Set by `cmdsbx do` because
	// that command is intended for unconditional agent allow-lists.
	NoPull bool
	// Interactive streams stdin into the sandbox (`do -i`), mirroring
	// docker run -i; without it stdin is closed immediately so runs
	// never block waiting on it.
	Interactive bool
	// Name names an ephemeral (ID-less) container so the broker can
	// force-remove it when a run times out: killing the docker CLI does
	// not stop the container it started.
	Name string
	// Memory and PidsLimit cap container resources; set by the broker
	// ("" / 0 disable).
	Memory    string
	PidsLimit int
	Mounts    []Mount
	Env       []string
	Command   []string
}

// ExecOptions configures `cmdsbx exec`.
type ExecOptions struct {
	ID      string
	Workdir string
	Env     []string
	Command []string
}

var idPattern = regexp.MustCompile(`^[a-zA-Z0-9][a-zA-Z0-9_.-]{0,63}$`)

func validateID(id string) error {
	if !idPattern.MatchString(id) {
		return fmt.Errorf("invalid sandbox id %q", id)
	}
	return nil
}

var imagePattern = regexp.MustCompile(`^[a-zA-Z0-9][a-zA-Z0-9._:/@-]*$`)

// validateImage rejects image strings that docker would parse as CLI
// flags (e.g. "--privileged") instead of an image reference: the image
// is the first positional argument of `docker run`, so a leading '-'
// would inject arbitrary run flags.
func validateImage(image string) error {
	if !imagePattern.MatchString(image) {
		return fmt.Errorf("invalid image %q", image)
	}
	return nil
}

// ParseMount parses SRC:DST[:ro|rw]. The source is made absolute; the
// target must already be absolute.
func ParseMount(s string) (Mount, error) {
	parts := strings.Split(s, ":")
	m := Mount{}
	switch len(parts) {
	case 2:
		m.Source, m.Target = parts[0], parts[1]
	case 3:
		if parts[2] != "ro" && parts[2] != "rw" {
			return Mount{}, fmt.Errorf("invalid mount mode %q in %q (want ro or rw)", parts[2], s)
		}
		m.Source, m.Target, m.Mode = parts[0], parts[1], parts[2]
	default:
		return Mount{}, fmt.Errorf("invalid mount %q (want SRC:DST[:ro|rw])", s)
	}
	if m.Source == "" || m.Target == "" {
		return Mount{}, fmt.Errorf("invalid mount %q (want SRC:DST[:ro|rw])", s)
	}
	if !filepath.IsAbs(m.Target) {
		return Mount{}, fmt.Errorf("mount target must be an absolute path: %q", s)
	}
	abs, err := filepath.Abs(m.Source)
	if err != nil {
		return Mount{}, fmt.Errorf("resolve mount source %q: %w", m.Source, err)
	}
	m.Source = abs
	return m, nil
}

// validateWorkdir requires an absolute path: docker rejects relative
// working directories only at container start, with an opaque error.
func validateWorkdir(dir string) error {
	if dir != "" && !filepath.IsAbs(dir) {
		return fmt.Errorf("--workdir must be an absolute path: %q", dir)
	}
	return nil
}

// validateEnv requires KEY=VALUE form: a bare KEY would make docker
// forward the host's value into the sandbox, silently leaking host state.
func validateEnv(env []string) error {
	for _, e := range env {
		if i := strings.IndexByte(e, '='); i <= 0 {
			return fmt.Errorf("invalid env %q (want KEY=VALUE)", e)
		}
	}
	return nil
}
