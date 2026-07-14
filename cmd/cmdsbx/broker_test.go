package cmdsbx

import (
	"bytes"
	"context"
	"encoding/binary"
	"encoding/json"
	"io"
	"log/slog"
	"net"
	"os"
	"os/exec"
	"path/filepath"
	"slices"
	"strings"
	"sync"
	"testing"
	"time"
)

// noBroker points `do` at a nonexistent broker socket so tests exercise
// the direct-docker path even when a real daemon is running.
func noBroker(t *testing.T) {
	t.Helper()
	t.Setenv("SANDBOX_BROKER_SOCKET", filepath.Join(t.TempDir(), "absent.sock"))
}

type callRecorder struct {
	mu    sync.Mutex
	calls [][]string
}

func (c *callRecorder) add(call []string) {
	c.mu.Lock()
	defer c.mu.Unlock()
	c.calls = append(c.calls, call)
}

func (c *callRecorder) all() [][]string {
	c.mu.Lock()
	defer c.mu.Unlock()
	return slices.Clone(c.calls)
}

// stubExecCommandContext records docker invocations from the broker and
// runs the given shell script in their place.
func stubExecCommandContext(t *testing.T, rec *callRecorder, script string) {
	t.Helper()
	orig := execCommandContext
	execCommandContext = func(ctx context.Context, name string, args ...string) *exec.Cmd {
		rec.add(append([]string{name}, args...))
		return exec.CommandContext(ctx, "sh", "-c", script)
	}
	t.Cleanup(func() { execCommandContext = orig })
}

func stubExecCommandRecorder(t *testing.T, rec *callRecorder) {
	t.Helper()
	orig := execCommand
	execCommand = func(name string, args ...string) *exec.Cmd {
		rec.add(append([]string{name}, args...))
		return exec.Command("true")
	}
	t.Cleanup(func() { execCommand = orig })
}

// pipeConn is an in-memory net.Conn with unix-socket-like half close,
// usable where the test sandbox forbids binding real unix sockets.
type pipeConn struct {
	r *os.File
	w *os.File
}

func (c *pipeConn) Read(p []byte) (int, error)       { return c.r.Read(p) }
func (c *pipeConn) Write(p []byte) (int, error)      { return c.w.Write(p) }
func (c *pipeConn) CloseWrite() error                { return c.w.Close() }
func (c *pipeConn) LocalAddr() net.Addr              { return &net.UnixAddr{Net: "unix"} }
func (c *pipeConn) RemoteAddr() net.Addr             { return &net.UnixAddr{Net: "unix"} }
func (c *pipeConn) SetDeadline(time.Time) error      { return nil }
func (c *pipeConn) SetReadDeadline(time.Time) error  { return nil }
func (c *pipeConn) SetWriteDeadline(time.Time) error { return nil }

func (c *pipeConn) Close() error {
	c.r.Close()
	return c.w.Close()
}

func connPair(t *testing.T) (client, server net.Conn) {
	t.Helper()
	cr, sw, err := os.Pipe()
	if err != nil {
		t.Fatal(err)
	}
	sr, cw, err := os.Pipe()
	if err != nil {
		t.Fatal(err)
	}
	client = &pipeConn{r: cr, w: cw}
	server = &pipeConn{r: sr, w: sw}
	t.Cleanup(func() {
		client.Close()
		server.Close()
	})
	return client, server
}

var testBrokerConfig = brokerConfig{
	timeout:       time.Minute,
	memory:        "1g",
	pidsLimit:     64,
	maxConcurrent: 2,
	logger:        slog.New(slog.DiscardHandler),
}

// serveOneRequest runs serveConn against an in-memory connection and
// speaks the client side of the protocol on it.
func serveOneRequest(t *testing.T, cfg brokerConfig, req, stdin []byte) (stdout, stderr []byte, exit brokerExit) {
	t.Helper()
	client, server := connPair(t)
	done := make(chan struct{})
	go func() {
		defer close(done)
		serveConn(context.Background(), server, cfg)
	}()
	defer func() { <-done }()

	var lenBuf [4]byte
	binary.BigEndian.PutUint32(lenBuf[:], uint32(len(req)))
	if _, err := client.Write(append(lenBuf[:], req...)); err != nil {
		t.Fatal(err)
	}
	if len(stdin) > 0 {
		if _, err := client.Write(stdin); err != nil {
			t.Fatal(err)
		}
	}
	if err := client.(*pipeConn).CloseWrite(); err != nil {
		t.Fatal(err)
	}
	var outBuf, errBuf bytes.Buffer
	for {
		typ, payload, err := readFrame(client)
		if err != nil {
			t.Fatalf("readFrame: %v (stdout=%q stderr=%q)", err, outBuf.String(), errBuf.String())
		}
		switch typ {
		case frameStdout:
			outBuf.Write(payload)
		case frameStderr:
			errBuf.Write(payload)
		case frameExit:
			if err := json.Unmarshal(payload, &exit); err != nil {
				t.Fatal(err)
			}
			return outBuf.Bytes(), errBuf.Bytes(), exit
		default:
			t.Fatalf("unknown frame type %d", typ)
		}
	}
}

func TestBrokerRunsCommand(t *testing.T) {
	var rec callRecorder
	stubExecCommandContext(t, &rec, "tr a-z A-Z; echo oops >&2; exit 7")

	req := []byte(`{"image":"alpine:3","command":["tr","a-z","A-Z"],"env":["A=1"],"workdir":"/w"}`)
	stdout, stderr, exit := serveOneRequest(t, testBrokerConfig, req, []byte("hello\n"))
	if exit.Code != 7 || exit.Error != "" {
		t.Errorf("exit = %+v, want code 7", exit)
	}
	if string(stdout) != "HELLO\n" {
		t.Errorf("stdout = %q, want %q", stdout, "HELLO\n")
	}
	if string(stderr) != "oops\n" {
		t.Errorf("stderr = %q, want %q", stderr, "oops\n")
	}

	calls := rec.all()
	if len(calls) != 1 {
		t.Fatalf("docker invocations = %d, want 1", len(calls))
	}
	got := calls[0]
	name := ""
	if i := slices.Index(got, "--name"); i >= 0 && i+1 < len(got) {
		name = got[i+1]
	}
	if !strings.HasPrefix(name, "sandbox-broker-") {
		t.Errorf("missing broker container name: %q", got)
	}
	want := []string{
		"docker", "run", "--init", "--label", "sandbox.managed=1", "--rm", "-i",
		"--name", name,
		"--network", "none", "--pull", "never",
		"--memory", "1g", "--pids-limit", "64",
		"-w", "/w", "-e", "A=1",
		"alpine:3", "tr", "a-z", "A-Z",
	}
	if !slices.Equal(got, want) {
		t.Errorf("got  %q\nwant %q", got, want)
	}
}

func TestBrokerRejectsUnknownFields(t *testing.T) {
	var rec callRecorder
	stubExecCommandContext(t, &rec, "true")

	req := []byte(`{"image":"alpine:3","command":["true"],"mounts":["/:/host"]}`)
	_, _, exit := serveOneRequest(t, testBrokerConfig, req, nil)
	if exit.Code != 2 || !strings.Contains(exit.Error, "unknown field") {
		t.Errorf("exit = %+v, want code 2 with unknown field error", exit)
	}
	if len(rec.all()) != 0 {
		t.Errorf("docker must not run for a rejected request: %q", rec.all())
	}
}

func TestBrokerRejectsFlagInjectionImage(t *testing.T) {
	var rec callRecorder
	stubExecCommandContext(t, &rec, "true")

	req := []byte(`{"image":"--privileged","command":["true"]}`)
	_, _, exit := serveOneRequest(t, testBrokerConfig, req, nil)
	if exit.Code != 2 || !strings.Contains(exit.Error, "invalid image") {
		t.Errorf("exit = %+v, want code 2 with invalid image error", exit)
	}
	if len(rec.all()) != 0 {
		t.Errorf("docker must not run for a rejected request: %q", rec.all())
	}
}

func TestBrokerTimeoutRemovesContainer(t *testing.T) {
	var runs, rms callRecorder
	stubExecCommandContext(t, &runs, "exec sleep 10")
	stubExecCommandRecorder(t, &rms)
	cfg := testBrokerConfig
	cfg.timeout = 100 * time.Millisecond

	req := []byte(`{"image":"alpine:3","command":["sleep","10"]}`)
	_, _, exit := serveOneRequest(t, cfg, req, nil)
	if exit.Code != 124 || !strings.Contains(exit.Error, "timed out") {
		t.Errorf("exit = %+v, want code 124 with timeout error", exit)
	}
	calls := rms.all()
	if len(calls) != 1 || len(calls[0]) != 4 ||
		calls[0][1] != "rm" || calls[0][2] != "-f" ||
		!strings.HasPrefix(calls[0][3], "sandbox-broker-") {
		t.Errorf("expected docker rm -f of the broker container, got %q", calls)
	}
}

// startBroker listens on a real unix socket; skipped where the test
// sandbox forbids binding one.
func startBroker(t *testing.T, cfg brokerConfig) string {
	t.Helper()
	sock := filepath.Join(t.TempDir(), "broker.sock")
	ln, err := net.Listen("unix", sock)
	if err != nil {
		t.Skipf("cannot bind unix socket: %v", err)
	}
	ctx, cancel := context.WithCancel(context.Background())
	done := make(chan struct{})
	go func() {
		defer close(done)
		serveListener(ctx, ln, cfg)
	}()
	t.Cleanup(func() {
		cancel()
		ln.Close()
		<-done
	})
	return sock
}

func TestMainDoUsesBroker(t *testing.T) {
	var rec callRecorder
	stubExecCommandContext(t, &rec, "echo hi")
	sock := startBroker(t, testBrokerConfig)
	t.Setenv("SANDBOX_BROKER_SOCKET", sock)

	devnull, err := os.Open(os.DevNull)
	if err != nil {
		t.Fatal(err)
	}
	defer devnull.Close()
	r, w, err := os.Pipe()
	if err != nil {
		t.Fatal(err)
	}
	origIn, origOut := os.Stdin, os.Stdout
	os.Stdin, os.Stdout = devnull, w
	code := Main([]string{"do", "--image", "alpine:3", "--", "echo", "hi"})
	os.Stdin, os.Stdout = origIn, origOut
	w.Close()
	out, err := io.ReadAll(r)
	if err != nil {
		t.Fatal(err)
	}
	if code != 0 {
		t.Fatalf("exit = %d, want 0", code)
	}
	if string(out) != "hi\n" {
		t.Errorf("stdout = %q, want %q", out, "hi\n")
	}
	calls := rec.all()
	if len(calls) != 1 {
		t.Fatalf("docker invocations = %d, want 1", len(calls))
	}
	if !containsPair(calls[0], "--pull", "never") {
		t.Errorf("broker run must pass --pull never: %q", calls[0])
	}
}

func TestBuildRunArgsBrokerFields(t *testing.T) {
	args, err := BuildRunArgs(&RunOptions{
		Image:     "alpine:3",
		Command:   []string{"true"},
		NoPull:    true,
		Name:      "sandbox-broker-1-1",
		Memory:    "2g",
		PidsLimit: 100,
	})
	if err != nil {
		t.Fatal(err)
	}
	want := []string{
		"run", "--init", "--label", "sandbox.managed=1", "--rm", "-i",
		"--name", "sandbox-broker-1-1",
		"--network", "none", "--pull", "never",
		"--memory", "2g", "--pids-limit", "100",
		"alpine:3", "true",
	}
	if !slices.Equal(args, want) {
		t.Errorf("got  %q\nwant %q", args, want)
	}
}

func TestValidateImage(t *testing.T) {
	for _, image := range []string{"x", "ubuntu:24.04", "python:3.12-slim", "ghcr.io/foo/bar@sha256:abc", "registry:5000/a/b:tag"} {
		if err := validateImage(image); err != nil {
			t.Errorf("validateImage(%q) = %v", image, err)
		}
	}
	for _, image := range []string{"", "-x", "--privileged", "a b", "a\nb"} {
		if err := validateImage(image); err == nil {
			t.Errorf("validateImage(%q) should fail", image)
		}
	}
}
