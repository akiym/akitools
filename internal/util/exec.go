package util

import (
	"os"
	"os/exec"
)

func ExecEmbeddedScript(command, embeddedScript string, args []string) error {
	tmpfile, err := os.CreateTemp("", "akitools-*")
	if err != nil {
		return err
	}
	defer tmpfile.Close()
	// 消さないと呼び出しのたびにスクリプトが $TMPDIR に溜まり続ける
	defer os.Remove(tmpfile.Name())

	if _, err := tmpfile.WriteString(embeddedScript); err != nil {
		return err
	}
	if err := tmpfile.Close(); err != nil {
		return err
	}
	cmd := exec.Command(
		command,
		append([]string{tmpfile.Name()}, args...)...,
	)
	cmd.Stdin = os.Stdin
	cmd.Stdout = os.Stdout
	cmd.Stderr = os.Stderr
	return cmd.Run()
}
