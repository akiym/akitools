package shellcode

import (
	_ "embed"
	"os"
	"syscall"

	"github.com/spf13/cobra"
)

//go:embed shellcode
var binary []byte

var Cmd = &cobra.Command{
	Use:   "shellcode",
	Short: "",
	RunE: func(cmd *cobra.Command, args []string) error {
		return run(args)
	},
	DisableFlagParsing: true,
}

func run(args []string) error {
	tmpfile, err := os.CreateTemp("", "")
	if err != nil {
		return err
	}
	defer tmpfile.Close()

	if err := os.WriteFile(tmpfile.Name(), binary, 0); err != nil {
		return err
	}
	if err := os.Chmod(tmpfile.Name(), 0755); err != nil {
		return err
	}
	return syscall.Exec(tmpfile.Name(), append([]string{tmpfile.Name()}, args...), os.Environ())
}
