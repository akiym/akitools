package o

import (
	"os/exec"

	"github.com/spf13/cobra"
)

var Cmd = &cobra.Command{
	Use:   "o",
	Short: "A wrapper for shorthand of open(1)",
	RunE: func(cmd *cobra.Command, args []string) error {
		return run(args)
	},
}

func run(args []string) error {
	if len(args) == 0 {
		return exec.Command("open", ".").Run()
	} else {
		return exec.Command("open", args...).Run()
	}
}
