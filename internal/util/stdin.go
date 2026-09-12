package util

import (
	"os"
	"os/exec"

	"github.com/mattn/go-isatty"
)

func StdinOrClipboard() (r *os.File, err error) {
	if isatty.IsTerminal(os.Stdin.Fd()) {
		output, err := exec.Command("pbpaste").Output()
		if err != nil {
			return os.Stdin, nil // ignore
		}

		r, w, err := os.Pipe()
		if err != nil {
			return nil, err
		}

		// A clipboard larger than the pipe buffer (64KB on Linux, less on
		// macOS) would block forever here: nothing reads the other end
		// until the caller hands it to a child process.
		go func() {
			defer w.Close()
			_, _ = w.Write(output)
		}()

		return r, nil
	} else {
		return os.Stdin, nil
	}
}
