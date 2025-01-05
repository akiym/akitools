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

var Cmd = &cobra.Command{
	Use:   "gistwrapper <filenames>",
	Short: "A wrapper of gist(1)",
	RunE: func(cmd *cobra.Command, args []string) error {
		return run(args)
	},
}

const gist = "gist"
const gistDigest = ".gistdigest"

var re = regexp.MustCompile(`/(\w+)$`)

func run(filenames []string) error {
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
		url, err := uploadGist(opt, filenames)
		if err != nil {
			return err
		}
		fmt.Println(url)
		pbcopy(url)
	} else {
		url, err := uploadGist(opt, filenames)
		if err != nil {
			return err
		}
		fmt.Println(url)
		m := re.FindStringSubmatch(url)
		if len(m) != 2 {
			return fmt.Errorf("invalid url: %s", url)
		}
		digest := m[1]
		if err := os.WriteFile(gistDigest, []byte(digest), 0644); err != nil {
			return err
		}
		pbcopy(url)
	}

	return nil
}

func uploadGist(opt []string, filenames []string) (string, error) {
	output, err := exec.Command(gist, append(opt, filenames...)...).Output()
	if err != nil {
		_, _ = fmt.Fprintf(os.Stderr, "%s", output)
		return "", err
	}
	return strings.TrimSuffix(string(output), "\n"), nil
}

func pbcopy(s string) {
	cmd := exec.Command("pbcopy")
	cmd.Stdin = strings.NewReader(s)
	_ = cmd.Run()
}
