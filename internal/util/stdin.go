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

		if _, err := w.Write(output); err != nil {
			return nil, err
		}
		_ = w.Close()

		return r, nil
	} else {
		return os.Stdin, nil
	}
}
