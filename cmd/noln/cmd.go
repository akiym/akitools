package noln

import (
	"bufio"
	"bytes"
	"io"
	"os"

	"github.com/spf13/cobra"
)

var Cmd = &cobra.Command{
	Use:   "noln",
	Short: "",
	RunE: func(cmd *cobra.Command, args []string) error {
		return run()
	},
}

func run() error {
	r := bufio.NewReader(os.Stdin)
	src, err := io.ReadAll(r)
	if err != nil {
		return err
	}

	src = bytes.TrimRight(src, "\r\n")
	_, _ = os.Stdout.Write(src)

	return nil
}
