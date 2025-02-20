package wfind

import (
	"fmt"

	"github.com/spf13/cobra"

	"github.com/akiym/akitools/internal/w3m"
)

// Requirements:
// - rg

// Original code:
// https://shinh.hatenablog.com/entry/20070429/1177827792

var Cmd = &cobra.Command{
	Use:   "wfind",
	Short: "",
	RunE: func(cmd *cobra.Command, args []string) error {
		return run(args)
	},
}

const lineMax = 10000

func run(args []string) error {
	return w3m.W3mWrapEach("find", args, lineMax, func(line string) string {
		return fmt.Sprintf(
			`<a href="%s">%[1]s</a>`,
			line,
		)
	})
}
