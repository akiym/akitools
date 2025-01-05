package util

import (
	"os"
	"os/exec"
)

func ExecEmbeddedScript(command, embeddedScript string, args []string) error {
	tmpfile, err := os.CreateTemp("", "")
	if err != nil {
		return err
	}
	defer tmpfile.Close()

	if err := os.WriteFile(tmpfile.Name(), []byte(embeddedScript), 0); err != nil {
		return err
	}
	cmd := exec.Command(
		command,
		append([]string{tmpfile.Name()}, args...)...,
	)
	cmd.Stdout = os.Stdout
	cmd.Stderr = os.Stderr
	return cmd.Run()
}
