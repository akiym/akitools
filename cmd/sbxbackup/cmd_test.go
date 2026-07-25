package sbxbackup

import (
	"os"
	"path/filepath"
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

func TestBackupPaths(t *testing.T) {
	if got := backupPaths("claude"); len(got) != 1 || got[0] != ".claude/projects" {
		t.Errorf("claude: got %v", got)
	}
	if got := backupPaths("unknown"); len(got) != 5 {
		t.Errorf("unknown: got %v", got)
	}
}

func TestExtractLogs(t *testing.T) {
	staging := t.TempDir()
	root := t.TempDir()

	writeFile := func(p string) {
		t.Helper()
		if err := os.MkdirAll(filepath.Dir(p), 0o755); err != nil {
			t.Fatal(err)
		}
		if err := os.WriteFile(p, []byte("x"), 0o644); err != nil {
			t.Fatal(err)
		}
	}
	writeFile(filepath.Join(staging, ".claude/projects/-Users-a-proj/session.jsonl"))
	writeFile(filepath.Join(staging, ".codex/sessions/2026/07/25/rollout-1.jsonl"))
	writeFile(filepath.Join(staging, ".local/share/opencode/opencode.db"))
	writeFile(filepath.Join(staging, ".local/share/opencode/opencode.db-wal"))
	writeFile(filepath.Join(root, "claude/projects/-Users-a-old/old.jsonl"))

	s := sandbox{Name: "opencode-tg", ID: "a400da86-bdfd-4401-b444-1b9373074954", Agent: "opencode"}
	if err := extractLogs(staging, root, s); err != nil {
		t.Fatal(err)
	}

	for _, p := range []string{
		"claude/projects/-Users-a-proj/session.jsonl",
		"claude/projects/-Users-a-old/old.jsonl",
		"codex/sessions/2026/07/25/rollout-1.jsonl",
		"opencode/opencode-tg-a400da86.opencode.db",
		"opencode/opencode-tg-a400da86.opencode.db-wal",
	} {
		if _, err := os.Stat(filepath.Join(root, p)); err != nil {
			t.Errorf("missing %s: %v", p, err)
		}
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
