package ccwrap

import (
	"errors"
	"fmt"
	"net"
	"os"
	"os/exec"
	"os/signal"
	"path/filepath"
	"strconv"
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
	newDir := filepath.Join(dir, "akitools", "ccwrap")
	// 旧配置(<data>/ccwrap)からの移行
	oldDir := filepath.Join(dir, "ccwrap")
	if _, err := os.Stat(newDir); os.IsNotExist(err) {
		if _, err := os.Stat(oldDir); err == nil {
			if err := os.MkdirAll(filepath.Dir(newDir), 0o755); err != nil {
				return "", err
			}
			if err := os.Rename(oldDir, newDir); err != nil {
				return "", err
			}
		}
	}
	return newDir, nil
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

// needsHerdrAgentHint はHERDR_AGENTヒントを立てる必要があるかを返す。
// herdrのpane内(HERDR_PANE_IDが継承されている)でだけ意味があり、
// 呼び出し側が明示的にHERDR_AGENTを指定していればそちらを尊重する
func needsHerdrAgentHint() bool {
	return os.Getenv("HERDR_PANE_ID") != "" && os.Getenv("HERDR_AGENT") == ""
}

// applyHerdrAgentHint はherdrに「このpaneのagentはclaudeだ」と伝える。
// ccwrapはosc8wrap(内部でptyを張る)越しにclaudeを起動するため、
// host側のherdrから見えるpaneのforeground processはccwrapとosc8wrapだけで、
// claude本体は内側のptyに隠れて検出できない。
//
// HERDR_AGENTはプロセスの環境ブロックから読まれるが、これはexec時点で固定され
// os.Setenvでは書き換えられない(Linuxの/proc/<pid>/environもmacOSの
// KERN_PROCARGS2も反映しない)。そのため自プロセスをexecし直して環境ごと
// 差し替える。execした環境はosc8wrapとclaudeにもそのまま継承される。
//
// 成功時はプロセスイメージが置き換わるので戻らない。
func applyHerdrAgentHint() error {
	if !needsHerdrAgentHint() {
		return nil
	}
	exe, err := os.Executable()
	if err != nil {
		return err
	}
	env := append(filterEnv(os.Environ(), "HERDR_AGENT"), "HERDR_AGENT=claude")
	return syscall.Exec(exe, os.Args, env)
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
				if zstInfo, err := os.Stat(zstFile); err == nil && info.ModTime().Before(zstInfo.ModTime()) {
					continue
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
	if err := applyHerdrAgentHint(); err != nil {
		fmt.Fprintf(os.Stderr, "ccwrap: herdr agent hint disabled: %v\n", err)
	}

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

	cwd, err := os.Getwd()
	if err != nil {
		return 1, fmt.Errorf("getwd: %w", err)
	}
	timestamp := time.Now().Format("20060102-150405")
	brokerSocket, stopBroker := startBroker(cwd, timestamp)
	if stopBroker != nil {
		defer stopBroker()
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
	// mitm/harダンプにはAnthropicの認証ヘッダと会話全文が入るので、
	// 他ユーザーから読めない権限で作る(brokerログと同じ扱い)
	if err := os.MkdirAll(logDir, 0o700); err != nil {
		return 1, fmt.Errorf("create log dir: %w", err)
	}
	if err := os.Chmod(logDir, 0o700); err != nil {
		return 1, fmt.Errorf("tighten log dir: %w", err)
	}

	port, err := findFreePort()
	if err != nil {
		return 1, fmt.Errorf("find free port: %w", err)
	}

	mitmFile := filepath.Join(logDir, timestamp+".mitm")
	harFile := filepath.Join(logDir, timestamp+".har")
	lockFile := filepath.Join(logDir, timestamp+".lock")

	// mitmdumpは自身のumaskでダンプを作るため、先に0600で作っておく。
	// 後続のO_TRUNC書き込みはmodeを変えないので権限は維持される
	if f, err := os.OpenFile(mitmFile, os.O_CREATE|os.O_WRONLY, 0o600); err != nil {
		return 1, fmt.Errorf("create mitm file: %w", err)
	} else {
		f.Close()
	}

	if err := os.WriteFile(lockFile, nil, 0o600); err != nil {
		return 1, fmt.Errorf("create lock file: %w", err)
	}
	defer os.Remove(lockFile)

	mitmdump := exec.Command("mitmdump",
		"--quiet",
		"--mode", "reverse:https://api.anthropic.com",
		"--listen-host", "127.0.0.1",
		"--listen-port", strconv.Itoa(port),
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
			_ = mitmdump.Process.Signal(syscall.SIGTERM)
			_ = mitmdump.Wait()
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
	env := append(filterEnv(os.Environ(), "ANTHROPIC_BASE_URL"),
		fmt.Sprintf("ANTHROPIC_BASE_URL=http://127.0.0.1:%d", port),
	)
	if brokerSocket != "" {
		env = append(filterEnv(env, "SANDBOX_BROKER_SOCKET"), "SANDBOX_BROKER_SOCKET="+brokerSocket)
	}
	claude.Env = env

	if err := claude.Start(); err != nil {
		cleanupMitmdump()
		return 1, fmt.Errorf("start claude: %w", err)
	}

	signal.Ignore(syscall.SIGINT)

	sigCh := make(chan os.Signal, 1)
	signal.Notify(sigCh, syscall.SIGTERM)
	go func() {
		for range sigCh {
			_ = claude.Process.Signal(syscall.SIGTERM)
		}
	}()

	claudeErr := claude.Wait()
	signal.Stop(sigCh)
	close(sigCh)

	cleanupMitmdump()

	if fi, err := os.Stat(mitmFile); err == nil && fi.Size() > 0 {
		// harもmitmと同じく認証ヘッダを含む。変換が終わってからchmodすると
		// 書き出している間ずっと他ユーザーから読めるので、先に0600で作る
		if f, err := os.OpenFile(harFile, os.O_CREATE|os.O_WRONLY, 0o600); err != nil {
			fmt.Fprintf(os.Stderr, "ccwrap: failed to create HAR file: %v\n", err)
		} else {
			f.Close()
		}
		conv := exec.Command("mitmdump",
			"-nr", mitmFile,
			"--set", "hardump="+harFile,
		)
		conv.Stderr = os.Stderr
		if err := conv.Run(); err != nil {
			fmt.Fprintf(os.Stderr, "ccwrap: failed to convert to HAR: %v\n", err)
		}
		if err := os.Chmod(harFile, 0o600); err != nil && !os.IsNotExist(err) {
			fmt.Fprintf(os.Stderr, "ccwrap: failed to tighten HAR permissions: %v\n", err)
		}
	}

	if claudeErr != nil {
		var exitErr *exec.ExitError
		if errors.As(claudeErr, &exitErr) {
			return exitErr.ExitCode(), nil
		}
		return 1, fmt.Errorf("claude: %w", claudeErr)
	}

	return 0, nil
}
