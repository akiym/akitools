package cmdsbx

import (
	"bytes"
	"context"
	"encoding/binary"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"log/slog"
	"net"
	"os"
	"os/exec"
	"os/signal"
	"path/filepath"
	"strings"
	"sync"
	"sync/atomic"
	"syscall"
	"time"
)

// The broker daemon executes `cmdsbx do`-equivalent runs on behalf of
// clients that must not reach the Docker socket themselves (e.g. sandboxed
// agents that are only allowed to connect to the broker socket). The wire
// protocol cannot express mounts, network access, or any other
// isolation-weakening option:
//
//	client → server: uint32(BE) request length + JSON brokerRequest,
//	                 then raw stdin bytes until the client half-closes
//	server → client: frames of [type:1][length:4 BE][payload], carrying
//	                 stdout/stderr data and a final exit frame

const (
	frameStdout byte = 1
	frameStderr byte = 2
	frameExit   byte = 3

	maxRequestSize = 1 << 20
)

// brokerRequest is the only shape the broker accepts. Adding a field here
// widens what sandboxed clients can request; anything isolation-weakening
// belongs to `cmdsbx unsafe`, which the broker deliberately cannot run.
// Rootfs is the one concession: it is honored only when the broker was
// started with an allowed directory, is confined to that directory's
// subtree, and always mounts read-only.
type brokerRequest struct {
	Image   string   `json:"image"`
	Command []string `json:"command"`
	Env     []string `json:"env,omitempty"`
	Workdir string   `json:"workdir,omitempty"`
	Rootfs  string   `json:"rootfs,omitempty"`
}

type brokerExit struct {
	Code  int    `json:"code"`
	Error string `json:"error,omitempty"`
}

type brokerConfig struct {
	timeout       time.Duration
	memory        string
	pidsLimit     int
	maxConcurrent int
	// allowRootfs is the symlink-resolved directory whose subtree clients
	// may request as a read-only rootfs mount; empty rejects all requests.
	allowRootfs string
	logger      *slog.Logger
}

// BrokerOptions configures a broker embedded via ListenBroker/Serve.
type BrokerOptions struct {
	Timeout       time.Duration
	Memory        string // "" disables the memory limit
	PidsLimit     int    // 0 disables the pids limit
	MaxConcurrent int
	// AllowRootfs is a symlink-resolved absolute directory whose subtree
	// clients may request as a read-only rootfs mount ("" rejects all).
	AllowRootfs string
	Logger      *slog.Logger
}

// DefaultBrokerOptions returns the limits `cmdsbx broker` defaults to.
func DefaultBrokerOptions() BrokerOptions {
	return BrokerOptions{
		Timeout:       5 * time.Minute,
		Memory:        "2g",
		PidsLimit:     1024,
		MaxConcurrent: 8,
	}
}

func xdgStateDir() string {
	if dir := os.Getenv("XDG_STATE_HOME"); dir != "" {
		return dir
	}
	home, err := os.UserHomeDir()
	if err != nil {
		return ""
	}
	return filepath.Join(home, ".local", "state")
}

// brokerSocketPath places the socket under $XDG_RUNTIME_DIR, falling
// back to the XDG state dir on systems without one (macOS): the
// rendezvous path must be stable across sessions, which rules out
// $TMPDIR-style per-session directories.
func brokerSocketPath() string {
	if v := os.Getenv("SANDBOX_BROKER_SOCKET"); v != "" {
		return v
	}
	dir := os.Getenv("XDG_RUNTIME_DIR")
	if dir == "" {
		dir = xdgStateDir()
	}
	if dir == "" {
		return ""
	}
	return filepath.Join(dir, "akitools", "cmdsbx", "broker.sock")
}

// brokerLogPath places the log under the XDG state dir, which the spec
// designates for logs and history.
func brokerLogPath() string {
	dir := xdgStateDir()
	if dir == "" {
		return ""
	}
	return filepath.Join(dir, "akitools", "cmdsbx", "broker.log")
}

// execCommandContext is swapped out by tests, like execCommand.
var execCommandContext = exec.CommandContext

func cmdBroker(args []string) int {
	opts := DefaultBrokerOptions()
	fs := newFlagSet("cmdsbx broker")
	socket := fs.String("socket", brokerSocketPath(), "unix socket to listen on (env: SANDBOX_BROKER_SOCKET)")
	logPath := fs.String("log", brokerLogPath(), "request log file (empty to log to stderr)")
	fs.DurationVar(&opts.Timeout, "timeout", opts.Timeout, "hard timeout per command")
	fs.StringVar(&opts.Memory, "memory", opts.Memory, "container memory limit (empty to disable)")
	fs.IntVar(&opts.PidsLimit, "pids-limit", opts.PidsLimit, "container pids limit (0 to disable)")
	fs.IntVar(&opts.MaxConcurrent, "max-concurrent", opts.MaxConcurrent, "max concurrent commands")
	allowRootfs := fs.String("allow-rootfs", "", "host directory whose subtree clients may request as a read-only rootfs mount")
	fs.Usage = func() {
		fmt.Fprint(os.Stderr, "Usage: cmdsbx broker [options]\n\nServe 'cmdsbx do' requests over a unix socket so that sandboxed\nclients can run disposable containers without access to the Docker\nsocket. Only no-mount, no-network, no-pull runs can be requested,\nplus read-only rootfs mounts under -allow-rootfs when set.\n\nOptions:\n")
		fs.PrintDefaults()
	}
	if err := fs.Parse(args); err != nil {
		return parseExit(err)
	}
	if fs.NArg() > 0 {
		fs.Usage()
		return 2
	}
	if *socket == "" {
		return fail(errors.New("cannot determine broker socket path"))
	}
	if opts.MaxConcurrent < 1 {
		return fail(errors.New("--max-concurrent must be at least 1"))
	}
	if *allowRootfs != "" {
		abs, err := filepath.Abs(*allowRootfs)
		if err != nil {
			return fail(fmt.Errorf("-allow-rootfs: %w", err))
		}
		resolved, err := filepath.EvalSymlinks(abs)
		if err != nil {
			return fail(fmt.Errorf("-allow-rootfs: %w", err))
		}
		opts.AllowRootfs = resolved
	}
	logDest := io.Writer(os.Stderr)
	if *logPath != "" {
		if err := os.MkdirAll(filepath.Dir(*logPath), 0o700); err != nil {
			return fail(err)
		}
		logFile, err := os.OpenFile(*logPath, os.O_CREATE|os.O_WRONLY|os.O_APPEND, 0o600)
		if err != nil {
			return fail(err)
		}
		defer logFile.Close()
		logDest = logFile
	}
	opts.Logger = slog.New(slog.NewJSONHandler(logDest, nil))
	// Mask group/other bits so nothing the daemon creates (the socket
	// included, even in the window before ListenBroker's chmod) is ever
	// accessible to other users.
	syscall.Umask(0o077)
	b, err := ListenBroker(*socket)
	if err != nil {
		return fail(err)
	}
	ctx, stop := signal.NotifyContext(context.Background(), os.Interrupt, syscall.SIGTERM)
	defer stop()
	b.Serve(ctx, opts)
	return 0
}

// Broker is a listening broker socket ready to Serve.
type Broker struct {
	ln     net.Listener
	socket string
}

// ListenBroker prepares the socket directory and binds the broker
// socket, so that startup failures (another live broker, an unwritable
// directory) surface before Serve is spawned in the background. The
// directory is created (and, when pre-existing, tightened) to 0700 so
// other users cannot reach the socket even before its own chmod.
func ListenBroker(socket string) (*Broker, error) {
	if err := os.MkdirAll(filepath.Dir(socket), 0o700); err != nil {
		return nil, err
	}
	if err := os.Chmod(filepath.Dir(socket), 0o700); err != nil {
		return nil, err
	}
	if _, err := os.Stat(socket); err == nil {
		if conn, err := net.Dial("unix", socket); err == nil {
			conn.Close()
			return nil, fmt.Errorf("broker already running on %s", socket)
		}
		if err := os.Remove(socket); err != nil {
			return nil, err
		}
	}
	ln, err := net.Listen("unix", socket)
	if err != nil {
		return nil, err
	}
	if err := os.Chmod(socket, 0o600); err != nil {
		ln.Close()
		return nil, err
	}
	return &Broker{ln: ln, socket: socket}, nil
}

// Serve accepts requests until ctx is cancelled, then removes the
// socket. MaxConcurrent must be at least 1.
func (b *Broker) Serve(ctx context.Context, opts BrokerOptions) {
	logger := opts.Logger
	if logger == nil {
		logger = slog.New(slog.DiscardHandler)
	}
	go func() {
		<-ctx.Done()
		b.ln.Close()
	}()
	logger.Info("listening", "socket", b.socket)
	serveListener(ctx, b.ln, brokerConfig{
		timeout:       opts.Timeout,
		memory:        opts.Memory,
		pidsLimit:     opts.PidsLimit,
		maxConcurrent: opts.MaxConcurrent,
		allowRootfs:   opts.AllowRootfs,
		logger:        logger,
	})
	logger.Info("shutting down")
	os.Remove(b.socket)
}

func serveListener(ctx context.Context, ln net.Listener, cfg brokerConfig) {
	sem := make(chan struct{}, cfg.maxConcurrent)
	var wg sync.WaitGroup
	defer wg.Wait()
	for {
		conn, err := ln.Accept()
		if err != nil {
			if ctx.Err() != nil || errors.Is(err, net.ErrClosed) {
				return
			}
			// Transient failures (EMFILE, ECONNABORTED) must not kill
			// the daemon; back off and keep accepting.
			cfg.logger.Error("accept failed", "error", err)
			time.Sleep(time.Second)
			continue
		}
		wg.Add(1)
		go func() {
			defer wg.Done()
			sem <- struct{}{}
			defer func() { <-sem }()
			serveConn(ctx, conn, cfg)
		}()
	}
}

var brokerRunSeq atomic.Int64

func brokerContainerName() string {
	return fmt.Sprintf("sandbox-broker-%d-%d", os.Getpid(), brokerRunSeq.Add(1))
}

func serveConn(ctx context.Context, conn net.Conn, cfg brokerConfig) {
	defer conn.Close()
	fw := &frameWriter{w: conn}
	// Bound how long a connection may sit without a request: it holds a
	// concurrency slot, and the shutdown path waits for it via wg.Wait.
	_ = conn.SetReadDeadline(time.Now().Add(10 * time.Second))
	req, err := readRequest(conn)
	if err != nil {
		writeExit(fw, 2, fmt.Sprintf("bad request: %v", err))
		return
	}
	_ = conn.SetReadDeadline(time.Time{})
	rootfs := ""
	if req.Rootfs != "" {
		rootfs, err = allowedRootfs(req.Rootfs, cfg.allowRootfs)
		if err != nil {
			writeExit(fw, 2, err.Error())
			return
		}
	}
	o := &RunOptions{
		Image:     req.Image,
		Rootfs:    rootfs,
		Workdir:   req.Workdir,
		Env:       req.Env,
		Command:   req.Command,
		NoPull:    true,
		Name:      brokerContainerName(),
		Memory:    cfg.memory,
		PidsLimit: cfg.pidsLimit,
	}
	dockerArgs, err := BuildRunArgs(o)
	if err != nil {
		writeExit(fw, 2, err.Error())
		return
	}
	cfg.logger.Info("run", "image", req.Image, "command", req.Command, "rootfs", rootfs)

	runCtx, cancel := context.WithTimeout(ctx, cfg.timeout)
	defer cancel()
	cmd := execCommandContext(runCtx, "docker", dockerArgs...)
	stdin, err := cmd.StdinPipe()
	if err != nil {
		writeExit(fw, 125, err.Error())
		return
	}
	cmd.Stdout = streamWriter{fw, frameStdout, cancel}
	cmd.Stderr = streamWriter{fw, frameStderr, cancel}
	// Straggler guard for the stdout/stderr copiers after exit or kill.
	cmd.WaitDelay = 2 * time.Second
	// Pump client stdin in a goroutine that Wait does not track: with
	// cmd.Stdin = conn, Wait would block until the pump's conn.Read
	// returned, holding every run open until the client sent data or
	// closed its end. Like docker's own CLI, the run finishes with the
	// process and the pump is unblocked by the deferred conn.Close.
	go func() {
		_, _ = io.Copy(stdin, conn)
		stdin.Close()
	}()
	err = cmd.Run()
	// Only a failed Run can be blamed on the context: a run that
	// finished successfully right at the deadline is still a success.
	if err != nil && runCtx.Err() != nil {
		removeContainer(o.Name)
		msg := "cancelled"
		if errors.Is(runCtx.Err(), context.DeadlineExceeded) {
			msg = fmt.Sprintf("timed out after %s", cfg.timeout)
		}
		writeExit(fw, 124, msg)
		return
	}
	var ee *exec.ExitError
	switch {
	case err == nil:
		writeExit(fw, 0, "")
	case errors.As(err, &ee):
		writeExit(fw, ee.ExitCode(), "")
	case errors.Is(err, exec.ErrWaitDelay):
		writeExit(fw, cmd.ProcessState.ExitCode(), "")
	default:
		writeExit(fw, 125, err.Error())
	}
}

// allowedRootfs validates a client-requested read-only rootfs mount:
// the request must be an absolute path that resolves (symlinks
// included) to the broker's allowed directory or below it.
func allowedRootfs(requested, allowed string) (string, error) {
	if allowed == "" {
		return "", errors.New("broker does not allow rootfs mounts")
	}
	if !filepath.IsAbs(requested) {
		return "", fmt.Errorf("rootfs must be an absolute path: %q", requested)
	}
	resolved, err := filepath.EvalSymlinks(requested)
	if err != nil {
		return "", fmt.Errorf("rootfs: %w", err)
	}
	rel, err := filepath.Rel(allowed, resolved)
	if err != nil || rel == ".." || strings.HasPrefix(rel, ".."+string(filepath.Separator)) {
		return "", fmt.Errorf("rootfs %q is outside %s", requested, allowed)
	}
	return resolved, nil
}

// removeContainer force-removes a named container after a cancelled run:
// killing the docker CLI does not stop the container it started.
func removeContainer(name string) {
	cmd := execCommand("docker", "rm", "-f", name)
	cmd.Stdout = io.Discard
	cmd.Stderr = io.Discard
	_ = cmd.Run()
}

func readRequest(r io.Reader) (*brokerRequest, error) {
	var lenBuf [4]byte
	if _, err := io.ReadFull(r, lenBuf[:]); err != nil {
		return nil, err
	}
	n := binary.BigEndian.Uint32(lenBuf[:])
	if n == 0 || n > maxRequestSize {
		return nil, fmt.Errorf("invalid request size %d", n)
	}
	buf := make([]byte, n)
	if _, err := io.ReadFull(r, buf); err != nil {
		return nil, err
	}
	dec := json.NewDecoder(bytes.NewReader(buf))
	dec.DisallowUnknownFields()
	var req brokerRequest
	if err := dec.Decode(&req); err != nil {
		return nil, err
	}
	return &req, nil
}

type frameWriter struct {
	mu sync.Mutex
	w  io.Writer
}

func (fw *frameWriter) writeFrame(typ byte, payload []byte) error {
	fw.mu.Lock()
	defer fw.mu.Unlock()
	buf := make([]byte, 5+len(payload))
	buf[0] = typ
	binary.BigEndian.PutUint32(buf[1:5], uint32(len(payload)))
	copy(buf[5:], payload)
	_, err := fw.w.Write(buf)
	return err
}

func writeExit(fw *frameWriter, code int, msg string) {
	payload, _ := json.Marshal(brokerExit{Code: code, Error: msg})
	_ = fw.writeFrame(frameExit, payload)
}

// streamWriter frames one output stream of the sandboxed command. A
// failed frame write means the client is gone, so it cancels the run
// instead of letting the container occupy a slot until the timeout.
type streamWriter struct {
	fw     *frameWriter
	typ    byte
	cancel context.CancelFunc
}

func (sw streamWriter) Write(p []byte) (int, error) {
	if len(p) == 0 {
		return 0, nil
	}
	if err := sw.fw.writeFrame(sw.typ, p); err != nil {
		sw.cancel()
		return 0, err
	}
	return len(p), nil
}

func readFrame(r io.Reader) (byte, []byte, error) {
	var hdr [5]byte
	if _, err := io.ReadFull(r, hdr[:]); err != nil {
		return 0, nil, err
	}
	n := binary.BigEndian.Uint32(hdr[1:])
	if n > maxRequestSize {
		return 0, nil, fmt.Errorf("frame too large (%d bytes)", n)
	}
	payload := make([]byte, n)
	if _, err := io.ReadFull(r, payload); err != nil {
		return 0, nil, err
	}
	return hdr[0], payload, nil
}

func closeWrite(conn net.Conn) {
	if cw, ok := conn.(interface{ CloseWrite() error }); ok {
		cw.CloseWrite()
	}
}

// dialBroker connects to the broker daemon when its socket is present.
// A nil connection means `do` should run docker directly.
func dialBroker() net.Conn {
	path := brokerSocketPath()
	if path == "" {
		return nil
	}
	if _, err := os.Stat(path); err != nil {
		return nil
	}
	conn, err := net.Dial("unix", path)
	if err != nil {
		fmt.Fprintf(os.Stderr, "cmdsbx: broker socket %s not reachable (%v); running docker directly\n", path, err)
		return nil
	}
	return conn
}

// runViaBroker submits a `do` run to the broker daemon and relays
// stdin/stdout/stderr, returning the exit code to propagate.
func runViaBroker(conn net.Conn, o *RunOptions) int {
	defer conn.Close()
	req, err := json.Marshal(brokerRequest{
		Image:   o.Image,
		Command: o.Command,
		Env:     o.Env,
		Workdir: o.Workdir,
		Rootfs:  o.Rootfs,
	})
	if err != nil {
		return fail(err)
	}
	var lenBuf [4]byte
	binary.BigEndian.PutUint32(lenBuf[:], uint32(len(req)))
	if _, err := conn.Write(append(lenBuf[:], req...)); err != nil {
		return fail(err)
	}
	// docker run -i does not exit until its stdin closes, even after the
	// container has exited, so streaming stdin unconditionally would
	// hold every run open until it saw input. Stream only under `do -i`,
	// mirroring docker; otherwise half-close right away.
	if o.Interactive {
		go func() {
			_, _ = io.Copy(conn, os.Stdin)
			closeWrite(conn)
		}()
	} else {
		closeWrite(conn)
	}
	for {
		typ, payload, err := readFrame(conn)
		if err != nil {
			fmt.Fprintf(os.Stderr, "cmdsbx: broker: %v\n", err)
			return 125
		}
		switch typ {
		case frameStdout:
			_, _ = os.Stdout.Write(payload)
		case frameStderr:
			_, _ = os.Stderr.Write(payload)
		case frameExit:
			var exit brokerExit
			if err := json.Unmarshal(payload, &exit); err != nil {
				fmt.Fprintf(os.Stderr, "cmdsbx: broker: %v\n", err)
				return 125
			}
			if exit.Error != "" {
				fmt.Fprintf(os.Stderr, "cmdsbx: %s\n", exit.Error)
			}
			return exit.Code
		default:
			fmt.Fprintf(os.Stderr, "cmdsbx: broker: unknown frame type %d\n", typ)
			return 125
		}
	}
}
