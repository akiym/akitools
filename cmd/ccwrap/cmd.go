package ccwrap

import (
	"fmt"
	"net"
	"os"
	"os/exec"
	"os/signal"
	"path/filepath"
	"strings"
	"sync"
	"syscall"
	"time"

	"github.com/spf13/cobra"
)

var Cmd = &cobra.Command{
	Use:                "ccwrap",
	DisableFlagParsing: true,
	RunE: func(cmd *cobra.Command, args []string) error {
		if len(args) >= 1 && args[0] == "compress" {
			return runCompress()
		}
		exitCode, err := run(args)
		if err != nil {
			return err
		}
		os.Exit(exitCode)
		return nil
	},
}

func workspaceName() (string, error) {
	cwd, err := os.Getwd()
	if err != nil {
		return "", err
	}
	return strings.ReplaceAll(cwd, "/", "-"), nil
}

func dataDir() (string, error) {
	dir := os.Getenv("XDG_DATA_HOME")
	if dir == "" {
		home, err := os.UserHomeDir()
		if err != nil {
			return "", err
		}
		dir = filepath.Join(home, ".local", "share")
	}
	return filepath.Join(dir, "ccwrap"), nil
}

func findFreePort() (int, error) {
	l, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		return 0, err
	}
	defer l.Close()
	return l.Addr().(*net.TCPAddr).Port, nil
}

func waitForPort(port int, timeout time.Duration) error {
	deadline := time.Now().Add(timeout)
	for time.Now().Before(deadline) {
		conn, err := net.DialTimeout("tcp", fmt.Sprintf("127.0.0.1:%d", port), 100*time.Millisecond)
		if err == nil {
			conn.Close()
			return nil
		}
		time.Sleep(50 * time.Millisecond)
	}
	return fmt.Errorf("timeout waiting for port %d", port)
}

func filterEnv(env []string, key string) []string {
	prefix := key + "="
	filtered := make([]string, 0, len(env))
	for _, e := range env {
		if !strings.HasPrefix(e, prefix) {
			filtered = append(filtered, e)
		}
	}
	return filtered
}

func compressFile(file string) error {
	zstd := exec.Command("zstd", "--rm", "-f", "-q", file)
	zstd.Stderr = os.Stderr
	return zstd.Run()
}

func runCompress() error {
	base, err := dataDir()
	if err != nil {
		return fmt.Errorf("data dir: %w", err)
	}

	entries, err := os.ReadDir(base)
	if err != nil {
		return fmt.Errorf("read data dir: %w", err)
	}

	for _, entry := range entries {
		if !entry.IsDir() {
			continue
		}
		workspaceDir := filepath.Join(base, entry.Name())

		locks, _ := filepath.Glob(filepath.Join(workspaceDir, "*.lock"))
		lockedPrefixes := make(map[string]bool)
		for _, lock := range locks {
			info, err := os.Stat(lock)
			if err != nil {
				continue
			}
			if time.Since(info.ModTime()) > 10*24*time.Hour {
				os.Remove(lock)
				continue
			}
			prefix := strings.TrimSuffix(lock, ".lock")
			lockedPrefixes[prefix] = true
		}

		for _, pattern := range []string{"*.har", "*.mitm"} {
			files, err := filepath.Glob(filepath.Join(workspaceDir, pattern))
			if err != nil {
				return err
			}
			for _, file := range files {
				ext := filepath.Ext(file)
				prefix := strings.TrimSuffix(file, ext)
				if lockedPrefixes[prefix] {
					continue
				}
				info, err := os.Stat(file)
				if err != nil {
					continue
				}
				zstFile := file + ".zst"
				if _, err := os.Stat(zstFile); err == nil {
					zstInfo, err := os.Stat(zstFile)
					if err == nil && info.ModTime().Before(zstInfo.ModTime()) {
						continue
					}
				}
				fmt.Fprintf(os.Stderr, "compressing %s\n", file)
				if err := compressFile(file); err != nil {
					fmt.Fprintf(os.Stderr, "ccwrap: failed to compress %s: %v\n", file, err)
				}
			}
		}
	}

	return nil
}

func run(args []string) (int, error) {
	// 先にsettings.local.jsonを最新のcwdに揃えてから検査する
	if err := ensureLocalSandboxSettings(); err != nil {
		return 1, fmt.Errorf("setup sandbox settings: %w", err)
	}

	ok, err := confirmSettings()
	if err != nil {
		return 1, fmt.Errorf("check settings: %w", err)
	}
	if !ok {
		fmt.Fprintln(os.Stderr, "ccwrap: aborted")
		return 1, nil
	}

	workspace, err := workspaceName()
	if err != nil {
		return 1, fmt.Errorf("workspace: %w", err)
	}

	base, err := dataDir()
	if err != nil {
		return 1, fmt.Errorf("data dir: %w", err)
	}

	logDir := filepath.Join(base, workspace)
	if err := os.MkdirAll(logDir, 0o755); err != nil {
		return 1, fmt.Errorf("create log dir: %w", err)
	}

	port, err := findFreePort()
	if err != nil {
		return 1, fmt.Errorf("find free port: %w", err)
	}

	timestamp := time.Now().Format("20060102-150405")
	mitmFile := filepath.Join(logDir, timestamp+".mitm")
	harFile := filepath.Join(logDir, timestamp+".har")
	lockFile := filepath.Join(logDir, timestamp+".lock")

	if err := os.WriteFile(lockFile, nil, 0o644); err != nil {
		return 1, fmt.Errorf("create lock file: %w", err)
	}
	defer os.Remove(lockFile)

	mitmdump := exec.Command("mitmdump",
		"--quiet",
		"--mode", "reverse:https://api.anthropic.com",
		"--listen-host", "127.0.0.1",
		"--listen-port", fmt.Sprintf("%d", port),
		"--set", "stream_large_bodies=1",
		"--store-streamed-bodies",
		"-w", mitmFile,
	)
	mitmdump.SysProcAttr = &syscall.SysProcAttr{
		Setpgid: true,
	}
	mitmdump.Stderr = os.Stderr
	if err := mitmdump.Start(); err != nil {
		return 1, fmt.Errorf("start mitmdump: %w", err)
	}
	var cleanupOnce sync.Once
	cleanupMitmdump := func() {
		cleanupOnce.Do(func() {
			mitmdump.Process.Signal(syscall.SIGTERM)
			mitmdump.Wait()
		})
	}

	if err := waitForPort(port, 5*time.Second); err != nil {
		cleanupMitmdump()
		return 1, fmt.Errorf("mitmdump not ready: %w", err)
	}

	claudeArgs := append([]string{"--scheme=intellij://idea", "claude"}, args...)
	claude := exec.Command("osc8wrap", claudeArgs...)
	claude.Stdin = os.Stdin
	claude.Stdout = os.Stdout
	claude.Stderr = os.Stderr
	claude.Env = append(filterEnv(os.Environ(), "ANTHROPIC_BASE_URL"),
		fmt.Sprintf("ANTHROPIC_BASE_URL=http://127.0.0.1:%d", port),
	)

	if err := claude.Start(); err != nil {
		cleanupMitmdump()
		return 1, fmt.Errorf("start claude: %w", err)
	}

	signal.Ignore(syscall.SIGINT)

	sigCh := make(chan os.Signal, 1)
	signal.Notify(sigCh, syscall.SIGTERM)
	go func() {
		for range sigCh {
			claude.Process.Signal(syscall.SIGTERM)
		}
	}()

	claudeErr := claude.Wait()
	signal.Stop(sigCh)
	close(sigCh)

	cleanupMitmdump()

	if fi, err := os.Stat(mitmFile); err == nil && fi.Size() > 0 {
		conv := exec.Command("mitmdump",
			"-nr", mitmFile,
			"--set", "hardump="+harFile,
		)
		conv.Stderr = os.Stderr
		if err := conv.Run(); err != nil {
			fmt.Fprintf(os.Stderr, "ccwrap: failed to convert to HAR: %v\n", err)
		}
	}

	if claudeErr != nil {
		if exitErr, ok := claudeErr.(*exec.ExitError); ok {
			return exitErr.ExitCode(), nil
		}
		return 1, fmt.Errorf("claude: %w", claudeErr)
	}

	return 0, nil
}
