package cmdsbx

import (
	"os"

	"github.com/spf13/cobra"
)

// Requirements:
// - docker

var Cmd = &cobra.Command{
	Use:                "cmdsbx <command>",
	Short:              "Run commands in disposable Docker containers, isolated from the host",
	DisableFlagParsing: true,
	RunE: func(cmd *cobra.Command, args []string) error {
		os.Exit(Main(args))
		return nil
	},
}
