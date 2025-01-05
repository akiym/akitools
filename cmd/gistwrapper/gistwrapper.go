package gistwrapper

import (
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"regexp"
	"strings"

	"github.com/spf13/cobra"
)

// Requirements:
// - gist

const gist = "gist"
const gistDigest = ".gistdigest"

var re = regexp.MustCompile(`/(\w+)$`)

var Cmd = &cobra.Command{
	Use:   "gistwrapper <filenames>",
	Short: "A wrapper of gist(1)",
	RunE: func(cmd *cobra.Command, args []string) error {
		return run(args)
	},
}

func run(args []string) error {
	cwd, err := os.Getwd()
	if err != nil {
		return err
	}
	opt := []string{
		// private
		"-p",
		// description
		"-d", filepath.Base(cwd),
	}

	if _, err := os.Stat(gistDigest); err == nil {
		digest, err := os.ReadFile(gistDigest)
		if err != nil {
			return err
		}
		opt = append(opt, "-u", string(digest))
		opt = append(opt, args...)
		cmd := exec.Command(gist, opt...)
		cmd.Stdout = os.Stdout
		cmd.Stderr = os.Stderr
		return cmd.Run()
	} else {
		output, err := exec.Command(gist, append(opt, args...)...).Output()
		if err != nil {
			return err
		}
		fmt.Printf("%s", output)
		url := strings.TrimSuffix(string(output), "\n")
		m := re.FindStringSubmatch(url)
		if len(m) != 2 {
			return fmt.Errorf("invalid url: %s", url)
		}
		digest := m[1]
		if err := os.WriteFile(gistDigest, []byte(digest), 0644); err != nil {
			return err
		}
	}

	return nil
}
