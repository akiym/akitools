package git_sign

import (
	"fmt"
	"os"
	"os/exec"
	"strings"

	"github.com/spf13/cobra"
)

var Cmd = &cobra.Command{
	Use:   "git-sign",
	Short: "Sign the latest commit if it is unsigned and authored by you",
	RunE: func(cmd *cobra.Command, args []string) error {
		return run()
	},
}

func indent(s, prefix string) string {
	lines := strings.Split(s, "\n")
	for i, line := range lines {
		if line != "" {
			lines[i] = prefix + line
		}
	}
	return strings.Join(lines, "\n")
}

func gitConfig(key string) (string, error) {
	out, err := exec.Command("git", "config", key).Output()
	if err != nil {
		return "", fmt.Errorf("failed to get git config %s: %w", key, err)
	}
	return strings.TrimSpace(string(out)), nil
}

func run() error {
	userName, err := gitConfig("user.name")
	if err != nil {
		return err
	}
	userEmail, err := gitConfig("user.email")
	if err != nil {
		return err
	}

	// Get the latest commit info: hash, author name, author email, date, body, signature status
	out, err := exec.Command("git", "log", "-1", "--format=%H%n%an%n%ae%n%aD%n%G?%n%B").Output()
	if err != nil {
		return fmt.Errorf("failed to get latest commit: %w", err)
	}

	lines := strings.SplitN(string(out), "\n", 6)
	if len(lines) < 6 {
		return fmt.Errorf("unexpected git log output")
	}

	hash := lines[0]
	authorName := lines[1]
	authorEmail := lines[2]
	authorDate := lines[3]
	sigStatus := lines[4]
	body := strings.TrimRight(lines[5], "\n")

	shortHash := hash[:7]

	if authorName != userName || authorEmail != userEmail {
		fmt.Fprintf(os.Stderr, "skip: commit %s is not yours (%s <%s>)\n", shortHash, authorName, authorEmail)
		return nil
	}

	// N = no signature, other values (G, B, U, X, Y, R, E) indicate some form of signature
	if sigStatus != "N" && sigStatus != "" {
		fmt.Fprintf(os.Stderr, "skip: commit %s is already signed\n", shortHash)
		return nil
	}

	fmt.Fprintf(os.Stderr, "signing commit %s\n", shortHash)
	fmt.Fprintf(os.Stderr, "Author: %s <%s>\n", authorName, authorEmail)
	fmt.Fprintf(os.Stderr, "Date:   %s\n", authorDate)
	fmt.Fprintf(os.Stderr, "\n%s\n\n", indent(body, "    "))

	amend := exec.Command("git", "commit", "--amend", "--no-edit", "-S")
	amend.Stdin = os.Stdin
	amend.Stdout = os.Stdout
	amend.Stderr = os.Stderr
	return amend.Run()
}
