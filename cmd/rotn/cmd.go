package rotn

import (
	_ "embed"

	"github.com/spf13/cobra"

	"github.com/akiym/akitools/internal/util"
)

// Requirements:
// - perl

//go:embed rotn.pl
var script string

var Cmd = &cobra.Command{
	Use:   "rotn",
	Short: "",
	RunE: func(cmd *cobra.Command, args []string) error {
		return run(args)
	},
	DisableFlagParsing: true,
}

func run(args []string) error {
	return util.ExecEmbeddedScript("perl", script, args)
}
