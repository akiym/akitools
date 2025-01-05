package jwt

import (
	_ "embed"
	"os"

	"github.com/spf13/cobra"

	"github.com/akiym/akitools/internal/util"
)

// Requirements:
// - python
// - step
// - jq

//go:embed jwt.py
var script string

var Cmd = &cobra.Command{
	Use:   "jwt",
	Short: "",
	RunE: func(cmd *cobra.Command, args []string) error {
		return run(args)
	},
	DisableFlagParsing: true,
}

func run(args []string) error {
	stdin, err := util.StdinOrClipboard()
	if err != nil {
		return err
	}
	os.Stdin = stdin

	return util.ExecEmbeddedScript("python3", script, args)
}
