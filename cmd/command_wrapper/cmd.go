package command_wrapper

import (
	"os"
	"syscall"

	"github.com/spf13/cobra"
)

var Cmd = &cobra.Command{
	Use:   "command-wrapper <command>",
	Short: "",
	RunE: func(cmd *cobra.Command, args []string) error {
		return run(args)
	},
}

var envs []string

func init() {
	Cmd.Flags().StringArrayVarP(&envs, "env", "e", nil, "Set environment variables")
}

func run(args []string) error {
	environ := append([]string{}, envs...)
	if e := os.Getenv("COMMAND_WRAPPER_ENV"); e != "" {
		environ = append(environ, e)
	}
	return syscall.Exec(args[0], args, environ)
}
