package ccwrap

import (
	"bufio"
	"errors"
	"fmt"
	"io"
	"os"
	"os/exec"
	"os/signal"
	"path/filepath"
	"regexp"
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

// listenLine はmitmdumpがbindを終えたときにtermlogへ出す行
// ("... listening at 127.0.0.1:12345.")。ポートはgetsockname由来なので
// listen-port 0 で起動してもmitmdumpが実際に確保したポートが得られる
var listenLine = regexp.MustCompile(`listening at 127\.0\.0\.1:(\d+)\b`)

// waitForListen はmitmdumpのstdoutからbind済みポートを読み取る。
// 自分で空きポートを選んでから渡す方式だと、closeからmitmdumpのbindまでの
// 間に同じホストの別プロセスがそのポートを奪え、claudeは認証ヘッダごと
// そこへ送ってしまう。mitmdump自身が報告したポートなら所有者は確定する。
// 行が来る前にstdoutが閉じたらmitmdumpは起動に失敗している。
// 読み取り後もEOFまで捨て続け、パイプが詰まってmitmdumpが止まらないようにする
func waitForListen(r io.Reader, timeout time.Duration) (int, error) {
	type result struct {
		port int
		err  error
	}
	ch := make(chan result, 1)
	go func() {
		scanner := bufio.NewScanner(r)
		found := false
		for scanner.Scan() {
			if found {
				continue
			}
			if m := listenLine.FindStringSubmatch(scanner.Text()); m != nil {
				port, err := strconv.Atoi(m[1])
				ch <- result{port, err}
				found = true
			}
		}
		if !found {
			ch <- result{0, errors.New("mitmdump exited before listening")}
		}
	}()
	select {
	case res := <-ch:
		return res.port, res.err
	case <-time.After(timeout):
		return 0, errors.New("timeout waiting for mitmdump to listen")
	}
}

type mitmdumpProc struct {
	cmd  *exec.Cmd
	port int
	once sync.Once
}

func (p *mitmdumpProc) stop() {
	p.once.Do(func() {
		_ = p.cmd.Process.Signal(syscall.SIGTERM)
		_ = p.cmd.Wait()
	})
}

// startMitmdump はreverse proxyをOS任せのポートで起動し、mitmdumpが
// 報告したポートを返す
func startMitmdump(mitmFile string) (*mitmdumpProc, error) {
	cmd := exec.Command("mitmdump",
		"--mode", "reverse:https://api.anthropic.com",
		"--listen-host", "127.0.0.1",
		"--listen-port", "0",
		"--set", "flow_detail=0",
		"--set", "stream_large_bodies=1",
		"--store-streamed-bodies",
		"-w", mitmFile,
	)
	cmd.SysProcAttr = &syscall.SysProcAttr{
		Setpgid: true,
	}
	// stdoutがパイプだとPythonはブロックバッファリングするので、
	// listening行が即座に届くよう無効化する
	cmd.Env = append(os.Environ(), "PYTHONUNBUFFERED=1")
	cmd.Stderr = os.Stderr
	stdout, err := cmd.StdoutPipe()
	if err != nil {
		return nil, err
	}
	if err := cmd.Start(); err != nil {
		return nil, fmt.Errorf("start mitmdump: %w", err)
	}
	p := &mitmdumpProc{cmd: cmd}
	port, err := waitForListen(stdout, 5*time.Second)
	if err != nil {
		p.stop()
		return nil, fmt.Errorf("mitmdump not ready: %w", err)
	}
	p.port = port
	return p, nil
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

	mitmdump, err := startMitmdump(mitmFile)
	if err != nil {
		return 1, err
	}
	cleanupMitmdump := mitmdump.stop
	port := mitmdump.port

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
