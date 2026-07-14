package ccwrap

import (
	"os"
	"path/filepath"
	"testing"
)

func TestDataDir(t *testing.T) {
	base := t.TempDir()
	t.Setenv("XDG_DATA_HOME", base)

	dir, err := dataDir()
	if err != nil {
		t.Fatal(err)
	}
	if want := filepath.Join(base, "akitools", "ccwrap"); dir != want {
		t.Errorf("dataDir = %q, want %q", dir, want)
	}
}

func TestDataDirMigratesOldLayout(t *testing.T) {
	base := t.TempDir()
	t.Setenv("XDG_DATA_HOME", base)
	oldDir := filepath.Join(base, "ccwrap")
	writeTestFiles(t, oldDir, []string{"ws/20240101-000000.har"})

	dir, err := dataDir()
	if err != nil {
		t.Fatal(err)
	}
	if _, err := os.Stat(filepath.Join(dir, "ws", "20240101-000000.har")); err != nil {
		t.Errorf("old data was not migrated: %v", err)
	}
	if _, err := os.Stat(oldDir); !os.IsNotExist(err) {
		t.Error("old dir still exists")
	}
}

func TestDataDirKeepsExistingNewLayout(t *testing.T) {
	base := t.TempDir()
	t.Setenv("XDG_DATA_HOME", base)
	writeTestFiles(t, base, []string{
		"ccwrap/old.har",
		"akitools/ccwrap/new.har",
	})

	dir, err := dataDir()
	if err != nil {
		t.Fatal(err)
	}
	if _, err := os.Stat(filepath.Join(dir, "new.har")); err != nil {
		t.Errorf("new layout data missing: %v", err)
	}
	if _, err := os.Stat(filepath.Join(base, "ccwrap", "old.har")); err != nil {
		t.Error("old dir should be left untouched when new dir exists")
	}
}
