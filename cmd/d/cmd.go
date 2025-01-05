package d

import (
	_ "embed"

	"github.com/spf13/cobra"

	"github.com/akiym/akitools/internal/util"
)

// Requirements:
// - bash

//go:embed d.sh
var script string

var Cmd = &cobra.Command{
	Use:   "d",
	Short: "",
	RunE: func(cmd *cobra.Command, args []string) error {
		return run(args)
	},
	DisableFlagParsing: true,
}

func run(args []string) error {
	return util.ExecEmbeddedScript("bash", script, args)
}
