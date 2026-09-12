package git_sign

import (
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"slices"
	"strings"
	"testing"
)

// run() re-executes this binary via `git rebase --exec '<executable> git-sign
// --head-only'`. When invoked that way, act as the CLI instead of running the
// test suite again.
func TestMain(m *testing.M) {
	if len(os.Args) >= 2 && os.Args[1] == "git-sign" {
		var err error
		if slices.Contains(os.Args[2:], "--head-only") {
			err = runHead()
		} else {
			err = run()
		}
		if err != nil {
			fmt.Fprintln(os.Stderr, err)
			os.Exit(1)
		}
		os.Exit(0)
	}
	os.Exit(m.Run())
}

func git(t *testing.T, dir string, args ...string) string {
	t.Helper()
	cmd := exec.Command("git", args...)
	cmd.Dir = dir
	out, err := cmd.CombinedOutput()
	if err != nil {
		t.Fatalf("git %s: %v\n%s", strings.Join(args, " "), err, out)
	}
	return strings.TrimSpace(string(out))
}

// setupRepo creates an isolated git environment under a tmpdir: a global
// gitconfig detached from the real one, a fresh SSH signing key, a bare
// origin, and a work repo with one pushed commit. It chdirs into the repo.
func setupRepo(t *testing.T) string {
	t.Helper()

	tmp := t.TempDir()
	home := filepath.Join(tmp, "home")
	if err := os.MkdirAll(home, 0o755); err != nil {
		t.Fatal(err)
	}

	t.Setenv("HOME", home)
	t.Setenv("XDG_CONFIG_HOME", filepath.Join(home, ".config"))
	t.Setenv("GIT_CONFIG_NOSYSTEM", "1")
	t.Setenv("GIT_CONFIG_GLOBAL", filepath.Join(home, "gitconfig"))

	keyPath := filepath.Join(home, "id_ed25519")
	if out, err := exec.Command("ssh-keygen", "-q", "-t", "ed25519", "-N", "", "-C", "signer@example.com", "-f", keyPath).CombinedOutput(); err != nil {
		t.Fatalf("ssh-keygen: %v\n%s", err, out)
	}
	pub, err := os.ReadFile(keyPath + ".pub")
	if err != nil {
		t.Fatal(err)
	}
	allowedSigners := filepath.Join(home, "allowed_signers")
	if err := os.WriteFile(allowedSigners, []byte("signer@example.com "+string(pub)), 0o644); err != nil {
		t.Fatal(err)
	}

	for _, kv := range [][2]string{
		{"user.name", "Test Signer"},
		{"user.email", "signer@example.com"},
		{"gpg.format", "ssh"},
		{"user.signingkey", keyPath},
		{"gpg.ssh.allowedSignersFile", allowedSigners},
		{"commit.gpgsign", "false"},
		{"init.defaultBranch", "main"},
	} {
		git(t, tmp, "config", "--global", kv[0], kv[1])
	}

	origin := filepath.Join(tmp, "origin.git")
	git(t, tmp, "init", "--bare", origin)

	repo := filepath.Join(tmp, "repo")
	git(t, tmp, "init", repo)
	git(t, repo, "remote", "add", "origin", origin)
	commit(t, repo, "initial commit")
	git(t, repo, "push", "-u", "origin", "main")

	t.Chdir(repo)
	return repo
}

func commit(t *testing.T, repo, msg string, extraConfig ...string) {
	t.Helper()
	f := filepath.Join(repo, "file.txt")
	if err := os.WriteFile(f, []byte(msg+"\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	args := []string{}
	for _, c := range extraConfig {
		args = append(args, "-c", c)
	}
	git(t, repo, append(args, "add", "file.txt")...)
	git(t, repo, append(args, "commit", "-m", msg)...)
}

func signedCommit(t *testing.T, repo, msg string) {
	t.Helper()
	f := filepath.Join(repo, "file.txt")
	if err := os.WriteFile(f, []byte(msg+"\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	git(t, repo, "add", "file.txt")
	git(t, repo, "commit", "-S", "-m", msg)
}

func revParse(t *testing.T, repo, ref string) string {
	t.Helper()
	return git(t, repo, "rev-parse", ref)
}

// unpushedLog returns "sigStatus subject" lines for origin/main..HEAD in
// oldest-first order.
func unpushedLog(t *testing.T, repo string) []string {
	t.Helper()
	out := git(t, repo, "log", "--reverse", "--format=%G? %s", "origin/main..HEAD")
	if out == "" {
		return nil
	}
	return strings.Split(out, "\n")
}

func assertNotMidRebase(t *testing.T, repo string) {
	t.Helper()
	for _, d := range []string{"rebase-merge", "rebase-apply"} {
		if _, err := os.Stat(filepath.Join(repo, ".git", d)); err == nil {
			t.Errorf("repository left in rebase state: .git/%s exists", d)
		}
	}
}

func TestRunNoUnpushedCommits(t *testing.T) {
	repo := setupRepo(t)
	head := revParse(t, repo, "HEAD")

	if err := run(); err != nil {
		t.Fatalf("run: %v", err)
	}

	if got := revParse(t, repo, "HEAD"); got != head {
		t.Errorf("HEAD changed: %s -> %s", head, got)
	}
	assertNotMidRebase(t, repo)
}

func TestRunSignsHeadByAmend(t *testing.T) {
	repo := setupRepo(t)
	base := revParse(t, repo, "HEAD")
	commit(t, repo, "unsigned head")

	if err := run(); err != nil {
		t.Fatalf("run: %v", err)
	}

	want := []string{"G unsigned head"}
	if got := unpushedLog(t, repo); !slices.Equal(got, want) {
		t.Errorf("unpushed log = %v, want %v", got, want)
	}
	if got := revParse(t, repo, "HEAD^"); got != base {
		t.Errorf("parent commit rewritten: %s -> %s", base, got)
	}
	assertNotMidRebase(t, repo)
}

func TestRunSignsMultipleViaRebase(t *testing.T) {
	repo := setupRepo(t)
	base := revParse(t, repo, "HEAD")
	commit(t, repo, "first")
	commit(t, repo, "second")
	commit(t, repo, "third")

	if err := run(); err != nil {
		t.Fatalf("run: %v", err)
	}

	want := []string{"G first", "G second", "G third"}
	if got := unpushedLog(t, repo); !slices.Equal(got, want) {
		t.Errorf("unpushed log = %v, want %v", got, want)
	}
	if got := revParse(t, repo, "HEAD~3"); got != base {
		t.Errorf("pushed base rewritten: %s -> %s", base, got)
	}
	assertNotMidRebase(t, repo)
}

func TestRunSkipsOtherAuthors(t *testing.T) {
	repo := setupRepo(t)
	commit(t, repo, "mine first")
	commit(t, repo, "theirs", "user.name=Other", "user.email=other@example.com")
	commit(t, repo, "mine second")

	if err := run(); err != nil {
		t.Fatalf("run: %v", err)
	}

	want := []string{"G mine first", "N theirs", "G mine second"}
	if got := unpushedLog(t, repo); !slices.Equal(got, want) {
		t.Errorf("unpushed log = %v, want %v", got, want)
	}
	if got := git(t, repo, "log", "-1", "--format=%an <%ae>", "HEAD^"); got != "Other <other@example.com>" {
		t.Errorf("author of other's commit changed: %s", got)
	}
	assertNotMidRebase(t, repo)
}

func TestRunSkipsAlreadySignedAndAmendsHead(t *testing.T) {
	repo := setupRepo(t)
	signedCommit(t, repo, "already signed")
	signedHash := revParse(t, repo, "HEAD")
	commit(t, repo, "unsigned head")

	if err := run(); err != nil {
		t.Fatalf("run: %v", err)
	}

	want := []string{"G already signed", "G unsigned head"}
	if got := unpushedLog(t, repo); !slices.Equal(got, want) {
		t.Errorf("unpushed log = %v, want %v", got, want)
	}
	// Only HEAD needed signing, so the signed commit must not be rewritten.
	if got := revParse(t, repo, "HEAD^"); got != signedHash {
		t.Errorf("already-signed commit rewritten: %s -> %s", signedHash, got)
	}
	assertNotMidRebase(t, repo)
}

// If signing fails while the rebase is replaying commits (e.g. the signing
// key is unusable), git stops at the failed --exec step; run() must abort the
// rebase and restore the original state instead of leaving the repository
// mid-rebase.
func TestRunAbortsRebaseWhenSigningFails(t *testing.T) {
	repo := setupRepo(t)
	commit(t, repo, "first")
	commit(t, repo, "second")
	head := revParse(t, repo, "HEAD")
	git(t, repo, "config", "--global", "user.signingkey", filepath.Join(repo, "no-such-key"))

	err := run()
	if err == nil {
		t.Fatal("run succeeded despite broken signing key")
	}
	if !strings.Contains(err.Error(), "rebase aborted") {
		t.Errorf("error does not mention the abort: %v", err)
	}

	assertNotMidRebase(t, repo)
	if got := revParse(t, repo, "HEAD"); got != head {
		t.Errorf("HEAD not restored: %s -> %s", head, got)
	}
}

func TestRunThenPushUpdatesRemote(t *testing.T) {
	repo := setupRepo(t)
	origin := filepath.Join(filepath.Dir(repo), "origin.git")
	commit(t, repo, "unsigned head")

	if err := run(); err != nil {
		t.Fatalf("run: %v", err)
	}
	if err := push(); err != nil {
		t.Fatalf("push: %v", err)
	}

	if local, remote := revParse(t, repo, "HEAD"), revParse(t, origin, "main"); local != remote {
		t.Errorf("remote main = %s, want %s", remote, local)
	}
}

// push must behave like `git push origin HEAD`, so a push.default that makes a
// bare `git push` refuse to pick a refspec must not affect it.
func TestPushIgnoresPushDefault(t *testing.T) {
	repo := setupRepo(t)
	origin := filepath.Join(filepath.Dir(repo), "origin.git")
	git(t, repo, "config", "push.default", "nothing")
	commit(t, repo, "unsigned head")

	if err := run(); err != nil {
		t.Fatalf("run: %v", err)
	}
	if err := push(); err != nil {
		t.Fatalf("push: %v", err)
	}

	if local, remote := revParse(t, repo, "HEAD"), revParse(t, origin, "main"); local != remote {
		t.Errorf("remote main = %s, want %s", remote, local)
	}
}

func TestRunHeadOnlySkipsForeignCommit(t *testing.T) {
	repo := setupRepo(t)
	commit(t, repo, "theirs", "user.name=Other", "user.email=other@example.com")
	head := revParse(t, repo, "HEAD")

	if err := runHead(); err != nil {
		t.Fatalf("runHead: %v", err)
	}

	if got := revParse(t, repo, "HEAD"); got != head {
		t.Errorf("HEAD changed: %s -> %s", head, got)
	}
}
