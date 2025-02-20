package wag

import (
	"fmt"
	"strings"

	"github.com/spf13/cobra"

	"github.com/akiym/akitools/internal/w3m"
)

// Requirements:
// - rg

// Original code:
// https://shinh.hatenablog.com/entry/20070429/1177827792

var Cmd = &cobra.Command{
	// For historical reasons, I use the alias "ag" instead of "rg". w3m + ag = wag
	Use:   "wag",
	Short: "",
	RunE: func(cmd *cobra.Command, args []string) error {
		return run(args)
	},
}

const lineMax = 10000

func run(args []string) error {
	rgOptions := []string{
		"--no-heading",
		"--line-number",
		"--color=always",
		"--colors=path:none",
		"--colors=line:none",
	}
	return w3m.W3mWrapEach("rg", append(rgOptions, args...), lineMax, func(line string) string {
		// Remove ANSI escape sequences from path and line
		line = strings.Replace(line, "\x1b[0m", "", 4)

		line = strings.ReplaceAll(line, "\x1b[0m\x1b[1m\x1b[31m", "<b>")
		line = strings.ReplaceAll(line, "\x1b[0m", "</b>")

		if strings.Contains(line, ":") {
			parts := strings.SplitN(line, ":", 3)
			if len(parts) >= 2 {
				return fmt.Sprintf(
					`<a href="%s">%s</a>:%s`,
					parts[0]+"#"+parts[1],
					parts[0]+":"+parts[1],
					parts[2],
				)
			}
		}
		return line
	})
}
