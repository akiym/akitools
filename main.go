package main

import (
	"fmt"
	"os"
	"path/filepath"

	"github.com/spf13/cobra"

	"github.com/akiym/akitools/cmd/binary2png"
	"github.com/akiym/akitools/cmd/command_wrapper"
	"github.com/akiym/akitools/cmd/d"
	"github.com/akiym/akitools/cmd/gadgets"
	"github.com/akiym/akitools/cmd/gistwrapper"
	"github.com/akiym/akitools/cmd/git_branch_recent"
	"github.com/akiym/akitools/cmd/libc_offsets"
	"github.com/akiym/akitools/cmd/noln"
	"github.com/akiym/akitools/cmd/o"
	"github.com/akiym/akitools/cmd/random_string"
	"github.com/akiym/akitools/cmd/rotn"
	"github.com/akiym/akitools/cmd/shellcode"
	"github.com/akiym/akitools/cmd/tobin"
	"github.com/akiym/akitools/cmd/tohex"
)

var version = "0.0.1"
var revision = "HEAD"

func main() {
	rootCmd := &cobra.Command{
		Use:          "akitools <command>",
		Version:      fmt.Sprintf("%s (revision:%s)", version, revision),
		SilenceUsage: true,
	}

	rootCmd.AddCommand(binary2png.Cmd)
	rootCmd.AddCommand(command_wrapper.Cmd)
	rootCmd.AddCommand(d.Cmd)
	rootCmd.AddCommand(gadgets.Cmd)
	rootCmd.AddCommand(gistwrapper.Cmd)
	rootCmd.AddCommand(git_branch_recent.Cmd)
	rootCmd.AddCommand(libc_offsets.Cmd)
	rootCmd.AddCommand(noln.Cmd)
	rootCmd.AddCommand(o.Cmd)
	rootCmd.AddCommand(random_string.Cmd)
	rootCmd.AddCommand(rotn.Cmd)
	rootCmd.AddCommand(shellcode.Cmd)
	rootCmd.AddCommand(tobin.Cmd)
	rootCmd.AddCommand(tohex.Cmd)

	argv0 := filepath.Base(os.Args[0])
	for _, cmd := range rootCmd.Commands() {
		if argv0 == cmd.Name() {
			rootCmd.SetArgs(append([]string{argv0}, os.Args[1:]...))
			break
		}
	}

	if err := rootCmd.Execute(); err != nil {
		os.Exit(1)
	}
}
