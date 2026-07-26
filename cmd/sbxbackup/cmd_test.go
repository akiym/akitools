package sbxbackup

import (
	"archive/tar"
	"compress/gzip"
	"errors"
	"io"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestParseSandboxes(t *testing.T) {
	tests := []struct {
		name  string
		input string
		want  []sandbox
	}{
		{
			name:  "array",
			input: `[{"name":"a","status":"running"},{"name":"b","status":"stopped"}]`,
			want:  []sandbox{{Name: "a", Status: "running"}, {Name: "b", Status: "stopped"}},
		},
		{
			name:  "wrapped",
			input: `{"sandboxes":[{"name":"a","state":"exited"}]}`,
			want:  []sandbox{{Name: "a", Status: "exited"}},
		},
		{
			name: "sbx ls output",
			input: `{"sandboxes":[
				{"name":"claude-ccv","id":"61507cc7-b2c6-4f05-8f6f-98804ec21b40","agent":"claude","status":"stopped","workspaces":["/Users/akiym/src/github.com/akiym/ccv"]},
				{"name":"claude-sbx-templates","id":"5f42ce4f-ffca-462a-8ec8-8ee3776fc21e","agent":"claude","status":"running","workspaces":["/Users/akiym/src/github.com/akiym/sbx-templates"]}
			]}`,
			want: []sandbox{
				{Name: "claude-ccv", ID: "61507cc7-b2c6-4f05-8f6f-98804ec21b40", Agent: "claude", Status: "stopped"},
				{Name: "claude-sbx-templates", ID: "5f42ce4f-ffca-462a-8ec8-8ee3776fc21e", Agent: "claude", Status: "running"},
			},
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got, err := parseSandboxes([]byte(tt.input))
			if err != nil {
				t.Fatal(err)
			}
			if len(got) != len(tt.want) {
				t.Fatalf("got %v, want %v", got, tt.want)
			}
			for i := range got {
				if got[i] != tt.want[i] {
					t.Errorf("got %v, want %v", got[i], tt.want[i])
				}
			}
		})
	}
}

func TestIsRunning(t *testing.T) {
	for status, want := range map[string]bool{
		"running": true,
		"Running": true,
		"stopped": false,
		"exited":  false,
		"":        false,
	} {
		if got := isRunning(status); got != want {
			t.Errorf("isRunning(%q) = %v, want %v", status, got, want)
		}
	}
}

// extractFixtures unpacks testdata/sbx-images.tar.gz (ext4 images built with
// mkfs.ext4 -d) into a temp dir and returns the paths of the two images.
func extractFixtures(t *testing.T) (projectsVolume, rwlayer string) {
	t.Helper()
	f, err := os.Open(filepath.Join("testdata", "sbx-images.tar.gz"))
	if err != nil {
		t.Fatal(err)
	}
	defer f.Close()
	gz, err := gzip.NewReader(f)
	if err != nil {
		t.Fatal(err)
	}
	dir := t.TempDir()
	tr := tar.NewReader(gz)
	for {
		hdr, err := tr.Next()
		if errors.Is(err, io.EOF) {
			break
		}
		if err != nil {
			t.Fatal(err)
		}
		if hdr.Typeflag != tar.TypeReg {
			continue
		}
		out, err := os.Create(filepath.Join(dir, filepath.Base(hdr.Name)))
		if err != nil {
			t.Fatal(err)
		}
		if _, err := io.Copy(out, tr); err != nil {
			t.Fatal(err)
		}
		if err := out.Close(); err != nil {
			t.Fatal(err)
		}
	}
	return filepath.Join(dir, "projects-volume.img"), filepath.Join(dir, "rwlayer.img")
}

// fixtureContainerdRoot lays out a containerd root with the fixture images:
// a Claude projects volume for sandbox "claude-tg" and one snapshot rwlayer.
func fixtureContainerdRoot(t *testing.T) string {
	t.Helper()
	vol, rw := extractFixtures(t)
	croot := t.TempDir()

	volDir := filepath.Join(croot, volumeImagesRel)
	if err := os.MkdirAll(volDir, 0o755); err != nil {
		t.Fatal(err)
	}
	copyHostFile(t, vol, filepath.Join(volDir, "sandbox-plugin-claude-tg-claude-691d2fcf7e8e.img"))

	snapDir := filepath.Join(croot, snapshotsRel, "12")
	if err := os.MkdirAll(snapDir, 0o755); err != nil {
		t.Fatal(err)
	}
	copyHostFile(t, rw, filepath.Join(snapDir, "rwlayer.img"))
	if err := os.MkdirAll(filepath.Join(croot, snapshotsRel, "13"), 0o755); err != nil {
		t.Fatal(err)
	}
	return croot
}

func copyHostFile(t *testing.T, src, dst string) {
	t.Helper()
	data, err := os.ReadFile(src)
	if err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(dst, data, 0o644); err != nil {
		t.Fatal(err)
	}
}

func TestBackupClaudeVolume(t *testing.T) {
	croot := fixtureContainerdRoot(t)
	root := t.TempDir()

	imgs := claudeVolumeImages(croot, "claude-tg")
	if len(imgs) != 1 {
		t.Fatalf("claudeVolumeImages: got %v", imgs)
	}
	if claudeVolumeImages(croot, "claude") != nil {
		t.Error("prefix of a sandbox name must not match")
	}
	if claudeVolumeImages(croot, "other") != nil {
		t.Error("unrelated sandbox must not match")
	}

	ext4, err := openExt4(imgs[0])
	if err != nil {
		t.Fatal(err)
	}
	defer ext4.Close()
	files, err := backupClaudeVolume(ext4.fsys, filepath.Join(root, "claude", "projects"))
	if err != nil {
		t.Fatal(err)
	}
	if len(files) != 2 {
		t.Errorf("copied %v, want 2 files", files)
	}

	session := filepath.Join(root, "claude", "projects", "-Users-akiym-myproj", "11111111-2222-3333-4444-555555555555.jsonl")
	data, err := os.ReadFile(session)
	if err != nil {
		t.Fatal(err)
	}
	if want := `"cwd":"/Users/akiym/myproj"`; !strings.Contains(string(data), want) {
		t.Errorf("session content missing %q: %s", want, data)
	}
	if _, err := os.Stat(filepath.Join(root, "claude", "projects", "-Users-akiym-myproj", "agent-aaa.jsonl")); err != nil {
		t.Errorf("subagent file must be backed up too: %v", err)
	}
}

func TestBackupSnapshots(t *testing.T) {
	croot := fixtureContainerdRoot(t)
	root := t.TempDir()

	n, err := backupSnapshots(croot, root)
	if err != nil {
		t.Fatal(err)
	}
	if n != 3 {
		t.Errorf("saved %d files, want 3 (rollout, db, wal)", n)
	}

	rollout := filepath.Join(root, "codex", "sessions", "2026", "07", "25",
		"rollout-2026-07-25T00-00-00-01982588-1111-2222-3333-444455556666.jsonl")
	data, err := os.ReadFile(rollout)
	if err != nil {
		t.Fatal(err)
	}
	if want := `"cwd":"/repo"`; !strings.Contains(string(data), want) {
		t.Errorf("rollout content missing %q: %s", want, data)
	}

	ocDir := filepath.Join(root, "opencode")
	entries, err := os.ReadDir(ocDir)
	if err != nil {
		t.Fatal(err)
	}
	var names []string
	for _, e := range entries {
		names = append(names, e.Name())
	}
	if len(names) != 2 {
		t.Fatalf("opencode files: got %v, want main db and wal", names)
	}
	db, wal := names[0], names[1]
	if !strings.HasSuffix(db, ".opencode.db") {
		t.Errorf("unexpected db name %q", db)
	}
	if wal != db+"-wal" {
		t.Errorf("wal %q does not match db %q", wal, db)
	}

	// Re-running must be idempotent: same hash-based names, no new files.
	if _, err := backupSnapshots(croot, root); err != nil {
		t.Fatal(err)
	}
	entries, err = os.ReadDir(ocDir)
	if err != nil {
		t.Fatal(err)
	}
	if len(entries) != 2 {
		t.Errorf("re-run produced extra opencode files: %d", len(entries))
	}
}

func TestBackupSnapshotsMissingRoot(t *testing.T) {
	if _, err := backupSnapshots(filepath.Join(t.TempDir(), "nope"), t.TempDir()); err != nil {
		t.Fatalf("missing snapshots dir must not fail: %v", err)
	}
}
