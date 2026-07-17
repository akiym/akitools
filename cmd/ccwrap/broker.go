package ccwrap

import (
	"context"
	"crypto/rand"
	"fmt"
	"log/slog"
	"net"
	"os"
	"path/filepath"
	"strings"

	"github.com/akiym/akitools/cmd/cmdsbx"
)

const ccwrapDirRel = ".claude/.ccwrap"

// brokerLogPath はbrokerのログをXDG state配下のworkspace別ディレクトリ
// (dataDirのmitm/harログと同じcwd由来の名前)に置く。.claude以下には
// socketだけを置き、ログでプロジェクトを汚さない。timestampはrun()の
// mitm/harログと同じ値で、セッションの突き合わせができる
func brokerLogPath(cwd, timestamp string) (string, error) {
	base, err := xdgStateBase()
	if err != nil {
		return "", err
	}
	return filepath.Join(base, "akitools", "cmdsbx", strings.ReplaceAll(cwd, "/", "-"), timestamp+".log"), nil
}

// brokerSocketName は同一プロジェクトで並行するccwrapセッションが
// 衝突しないようsocket名をランダムにする
func brokerSocketName() (string, error) {
	var buf [4]byte
	if _, err := rand.Read(buf[:]); err != nil {
		return "", err
	}
	return fmt.Sprintf("broker-%x.sock", buf), nil
}

// cleanStaleSockets は異常終了したセッションが残したsocketを削除する。
// 生きているbroker(dialが通るもの)は並行セッションのものなので残す
func cleanStaleSockets(sockDir string) {
	matches, _ := filepath.Glob(filepath.Join(sockDir, "broker-*.sock"))
	for _, m := range matches {
		if conn, err := net.Dial("unix", m); err == nil {
			conn.Close()
			continue
		}
		os.Remove(m)
	}
}

// startBroker starts an in-process cmdsbx broker that allows read-only
// rootfs mounts inside cwd, so `cmdsbx do --mount-cwd-ro` works for the
// wrapped claude without weakening `do` anywhere else. The broker lives
// and dies with ccwrap. Failures to start are reported but never fatal:
// ccwrap still runs, just without a broker.
func startBroker(cwd, timestamp string) (socket string, stop func()) {
	warn := func(err error) {
		fmt.Fprintf(os.Stderr, "ccwrap: broker disabled: %v\n", err)
	}
	name, err := brokerSocketName()
	if err != nil {
		warn(err)
		return "", nil
	}
	sock := filepath.Join(cwd, ccwrapDirRel, name)
	// sun_path is capped at 104 bytes on macOS (108 on Linux)
	if len(sock) >= 104 {
		warn(fmt.Errorf("socket path too long: %s", sock))
		return "", nil
	}
	allowRootfs, err := filepath.EvalSymlinks(cwd)
	if err != nil {
		warn(err)
		return "", nil
	}
	sockDir := filepath.Join(cwd, ccwrapDirRel)
	if err := os.MkdirAll(sockDir, 0o700); err != nil {
		warn(err)
		return "", nil
	}
	cleanStaleSockets(sockDir)
	gitignore := filepath.Join(sockDir, ".gitignore")
	if _, err := os.Stat(gitignore); os.IsNotExist(err) {
		if err := os.WriteFile(gitignore, []byte("*\n"), 0o600); err != nil {
			warn(err)
			return "", nil
		}
	}
	logPath, err := brokerLogPath(cwd, timestamp)
	if err != nil {
		warn(err)
		return "", nil
	}
	if err := os.MkdirAll(filepath.Dir(logPath), 0o700); err != nil {
		warn(err)
		return "", nil
	}
	logFile, err := os.OpenFile(logPath, os.O_CREATE|os.O_WRONLY|os.O_APPEND, 0o600)
	if err != nil {
		warn(err)
		return "", nil
	}
	b, err := cmdsbx.ListenBroker(sock)
	if err != nil {
		logFile.Close()
		warn(err)
		return "", nil
	}
	opts := cmdsbx.DefaultBrokerOptions()
	opts.AllowRootfs = allowRootfs
	opts.Logger = slog.New(slog.NewJSONHandler(logFile, nil))
	ctx, cancel := context.WithCancel(context.Background())
	done := make(chan struct{})
	go func() {
		defer close(done)
		b.Serve(ctx, opts)
	}()
	return sock, func() {
		cancel()
		<-done
		logFile.Close()
	}
}
