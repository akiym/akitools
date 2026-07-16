package gistwrapper

import (
	"bytes"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"os"
	"os/exec"
	"path/filepath"
	"regexp"
	"strings"

	"github.com/spf13/cobra"
)

// Requirements:
// - gh
// - op

var Cmd = &cobra.Command{
	Use:   "gistwrapper <filenames>",
	Short: "Upload files to a secret gist via gh(1)",
	Args:  cobra.MinimumNArgs(1),
	RunE: func(cmd *cobra.Command, args []string) error {
		return run(args)
	},
}

const (
	gistDigest     = ".gistdigest"
	opGistTokenRef = "op://Personal/gist/token"
)

var re = regexp.MustCompile(`/(\w+)$`)

func run(filenames []string) error {
	token, err := readToken()
	if err != nil {
		return err
	}
	env := append(os.Environ(), "GH_TOKEN="+token)

	var url string
	b, err := os.ReadFile(gistDigest)
	switch {
	case err == nil:
		digest := strings.TrimSpace(string(b))
		if err := updateGist(env, digest, filenames); err != nil {
			return err
		}
		url = "https://gist.github.com/" + digest
	case errors.Is(err, os.ErrNotExist):
		url, err = createGist(env, filenames)
		if err != nil {
			return err
		}
		m := re.FindStringSubmatch(url)
		if len(m) != 2 {
			return fmt.Errorf("invalid url: %s", url)
		}
		if err := os.WriteFile(gistDigest, []byte(m[1]), 0o644); err != nil {
			return err
		}
	default:
		return err
	}

	fmt.Println(url)
	pbcopy(url)

	return nil
}

func createGist(env []string, filenames []string) (string, error) {
	cwd, err := os.Getwd()
	if err != nil {
		return "", err
	}
	return gh(env, nil, append([]string{"gist", "create", "-d", filepath.Base(cwd)}, filenames...)...)
}

func updateGist(env []string, digest string, filenames []string) error {
	files := make(map[string]map[string]string, len(filenames))
	for _, filename := range filenames {
		content, err := os.ReadFile(filename)
		if err != nil {
			return err
		}
		files[filepath.Base(filename)] = map[string]string{"content": string(content)}
	}
	body, err := json.Marshal(map[string]any{"files": files})
	if err != nil {
		return err
	}
	_, err = gh(env, bytes.NewReader(body), "api", "--silent", "-X", "PATCH", "--input", "-", "gists/"+digest)
	return err
}

func readToken() (string, error) {
	cmd := exec.Command("op", "read", opGistTokenRef)
	cmd.Stderr = os.Stderr
	output, err := cmd.Output()
	if err != nil {
		return "", err
	}
	token := strings.TrimSpace(string(output))
	return token, nil
}

func gh(env []string, stdin io.Reader, args ...string) (string, error) {
	cmd := exec.Command("gh", args...)
	cmd.Env = env
	cmd.Stdin = stdin
	cmd.Stderr = os.Stderr
	output, err := cmd.Output()
	if err != nil {
		return "", err
	}
	return strings.TrimSpace(string(output)), nil
}

func pbcopy(s string) {
	cmd := exec.Command("pbcopy")
	cmd.Stdin = strings.NewReader(s)
	_ = cmd.Run()
}
