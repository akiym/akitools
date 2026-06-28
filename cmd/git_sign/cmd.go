package git_sign

import (
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strings"

	"github.com/spf13/cobra"
)

var headOnly bool

var Cmd = &cobra.Command{
	Use:   "git-sign",
	Short: "Sign unpushed commits authored by you that are not yet signed",
	RunE: func(cmd *cobra.Command, args []string) error {
		if headOnly {
			return runHead()
		}
		return run()
	},
}

func init() {
	Cmd.Flags().BoolVar(&headOnly, "head-only", false, "sign HEAD only if it is unsigned and authored by you (used internally by rebase --exec)")
	_ = Cmd.Flags().MarkHidden("head-only")
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

func firstLine(s string) string {
	if i := strings.Index(s, "\n"); i >= 0 {
		return s[:i]
	}
	return s
}

func gitConfig(key string) (string, error) {
	out, err := exec.Command("git", "config", key).Output()
	if err != nil {
		return "", fmt.Errorf("failed to get git config %s: %w", key, err)
	}
	return strings.TrimSpace(string(out)), nil
}

type commitInfo struct {
	hash        string
	shortHash   string
	authorName  string
	authorEmail string
	authorDate  string
	sigStatus   string
	body        string
}

func getCommitInfo(ref string) (commitInfo, error) {
	out, err := exec.Command("git", "log", "-1", ref, "--format=%H%n%an%n%ae%n%aD%n%G?%n%B").Output()
	if err != nil {
		return commitInfo{}, fmt.Errorf("failed to get commit %s: %w", ref, err)
	}
	parts := strings.SplitN(string(out), "\n", 6)
	if len(parts) < 6 {
		return commitInfo{}, fmt.Errorf("unexpected git log output for %s", ref)
	}
	c := commitInfo{
		hash:        parts[0],
		authorName:  parts[1],
		authorEmail: parts[2],
		authorDate:  parts[3],
		sigStatus:   parts[4],
		body:        strings.TrimRight(parts[5], "\n"),
	}
	if len(c.hash) >= 7 {
		c.shortHash = c.hash[:7]
	} else {
		c.shortHash = c.hash
	}
	return c, nil
}

func resolveUpstream() (string, error) {
	for _, spec := range []string{"@{upstream}", "@{push}"} {
		out, err := exec.Command("git", "rev-parse", "--symbolic-full-name", spec).Output()
		if err != nil {
			continue
		}
		ref := strings.TrimSpace(string(out))
		if ref != "" {
			return ref, nil
		}
	}
	return "", fmt.Errorf("no upstream configured for current branch (set one with `git branch --set-upstream-to=...`)")
}

func listUnpushedHashes() ([]string, error) {
	upstream, err := resolveUpstream()
	if err != nil {
		return nil, err
	}
	rangeSpec := upstream + "..HEAD"

	out, err := exec.Command("git", "log", "--reverse", "--format=%H", rangeSpec).Output()
	if err != nil {
		return nil, fmt.Errorf("failed to list unpushed commits: %w", err)
	}
	trimmed := strings.TrimSpace(string(out))
	if trimmed == "" {
		return nil, nil
	}
	return strings.Split(trimmed, "\n"), nil
}

func shouldSign(c commitInfo, userName, userEmail string) (bool, string) {
	if c.authorName != userName || c.authorEmail != userEmail {
		return false, fmt.Sprintf("not yours (%s <%s>)", c.authorName, c.authorEmail)
	}
	if c.sigStatus != "N" && c.sigStatus != "" {
		return false, "already signed"
	}
	return true, ""
}

func amendSign() error {
	amend := exec.Command("git", "commit", "--amend", "--no-edit", "-S")
	amend.Env = append(os.Environ(), "HUSKY=0")
	amend.Stdin = os.Stdin
	amend.Stdout = os.Stdout
	amend.Stderr = os.Stderr
	return amend.Run()
}

func printCommit(c commitInfo) {
	fmt.Fprintf(os.Stderr, "Author: %s <%s>\n", c.authorName, c.authorEmail)
	fmt.Fprintf(os.Stderr, "Date:   %s\n", c.authorDate)
	fmt.Fprintf(os.Stderr, "\n%s\n\n", indent(c.body, "    "))
}

func runHead() error {
	userName, err := gitConfig("user.name")
	if err != nil {
		return err
	}
	userEmail, err := gitConfig("user.email")
	if err != nil {
		return err
	}

	c, err := getCommitInfo("HEAD")
	if err != nil {
		return err
	}

	if ok, reason := shouldSign(c, userName, userEmail); !ok {
		fmt.Fprintf(os.Stderr, "skip: commit %s is %s\n", c.shortHash, reason)
		return nil
	}

	fmt.Fprintf(os.Stderr, "signing commit %s\n", c.shortHash)
	printCommit(c)
	return amendSign()
}

func shellQuote(s string) string {
	return "'" + strings.ReplaceAll(s, "'", `'\''`) + "'"
}

func resolvedExecutable() (string, error) {
	p, err := os.Executable()
	if err != nil {
		return "", fmt.Errorf("failed to determine executable path: %w", err)
	}
	if r, err := filepath.EvalSymlinks(p); err == nil {
		return r, nil
	}
	return p, nil
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

	hashes, err := listUnpushedHashes()
	if err != nil {
		return err
	}
	if len(hashes) == 0 {
		fmt.Fprintln(os.Stderr, "no unpushed commits")
		return nil
	}

	var toSign []commitInfo
	for _, h := range hashes {
		c, err := getCommitInfo(h)
		if err != nil {
			return err
		}
		if ok, _ := shouldSign(c, userName, userEmail); ok {
			toSign = append(toSign, c)
		}
	}

	if len(toSign) == 0 {
		fmt.Fprintln(os.Stderr, "no unsigned commits authored by you among unpushed commits")
		return nil
	}

	headHash := hashes[len(hashes)-1]

	// If only HEAD itself needs signing, just amend.
	if len(toSign) == 1 && toSign[0].hash == headHash {
		c := toSign[0]
		fmt.Fprintf(os.Stderr, "signing commit %s\n", c.shortHash)
		printCommit(c)
		return amendSign()
	}

	fmt.Fprintf(os.Stderr, "signing %d commit(s) via rebase:\n", len(toSign))
	for _, c := range toSign {
		fmt.Fprintf(os.Stderr, "  %s %s\n", c.shortHash, firstLine(c.body))
	}

	// Rebase from the parent of the earliest commit that needs signing.
	earliest := toSign[0]
	parentOut, parentErr := exec.Command("git", "rev-parse", earliest.hash+"^").Output()

	executable, err := resolvedExecutable()
	if err != nil {
		return err
	}
	execCmd := fmt.Sprintf("%s git-sign --head-only", shellQuote(executable))

	var args []string
	if parentErr != nil {
		args = []string{"rebase", "--rebase-merges", "--exec", execCmd, "--root"}
	} else {
		parent := strings.TrimSpace(string(parentOut))
		args = []string{"rebase", "--rebase-merges", "--exec", execCmd, parent}
	}

	rebaseCmd := exec.Command("git", args...)
	rebaseCmd.Env = append(os.Environ(), "HUSKY=0")
	rebaseCmd.Stdin = os.Stdin
	rebaseCmd.Stdout = os.Stdout
	rebaseCmd.Stderr = os.Stderr
	return rebaseCmd.Run()
}
