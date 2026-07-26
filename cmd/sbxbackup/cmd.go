package sbxbackup

import (
	"bufio"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"io/fs"
	"os"
	"os/exec"
	"path"
	"path/filepath"
	"strings"

	diskfs "github.com/diskfs/go-diskfs"
	"github.com/diskfs/go-diskfs/disk"
	"github.com/diskfs/go-diskfs/filesystem"
	"github.com/mattn/go-isatty"
	"github.com/spf13/cobra"
)

// Requirements:
// - sbx (Docker Sandboxes CLI) — used only to list and remove sandboxes.
//   Logs are extracted by reading the containerd ext4 images directly, so
//   sandboxes are never started (or otherwise touched) during backup.

// Layout under the Docker Sandboxes containerd root:
//
//	com.docker.volume.driver.v1.block/images/sandbox-plugin-<name>-claude-<hash>.img
//	    per-sandbox Claude Code block volumes (ext4, written live). Only the
//	    projects volume has '-<encoded-cwd>' dirs at its root; the sibling
//	    sessions/todos/statsig/shell-snapshots volumes stay empty there.
//	io.containerd.snapshotter.v1.erofs/snapshots/<N>/rwlayer.img
//	    per-container writable overlay layer (ext4) holding Codex sessions and
//	    the opencode db under /upper/home/agent/..., flushed on stop/destroy.
//	    Snapshot numbers cannot be attributed to a sandbox without parsing
//	    containerd's bolt metadata, so every snapshot is scanned instead.
const (
	volumeImagesRel = "com.docker.volume.driver.v1.block/images"
	snapshotsRel    = "io.containerd.snapshotter.v1.erofs/snapshots"

	codexSessionsInner = "upper/home/agent/.codex/sessions"
	opencodeDBInner    = "upper/home/agent/.local/share/opencode/opencode.db"
)

var (
	flagKeep bool
	flagAll  bool
)

var Cmd = &cobra.Command{
	Use:   "sbx-backup [SANDBOX...]",
	Short: "Backup agent logs (claude, codex, opencode) from stopped sbx sandboxes, then remove them",
	RunE: func(cmd *cobra.Command, args []string) error {
		return run(args)
	},
}

func init() {
	Cmd.Flags().BoolVar(&flagKeep, "keep", false, "backup only; do not remove sandboxes")
	Cmd.Flags().BoolVarP(&flagAll, "all", "a", false, "include running sandboxes in backup (only stopped ones are removed)")
}

type sandbox struct {
	Name   string
	ID     string
	Agent  string
	Status string
}

func run(args []string) error {
	sandboxes, err := listSandboxes()
	if err != nil {
		return err
	}

	var targets []sandbox
	if len(args) > 0 {
		byName := make(map[string]sandbox, len(sandboxes))
		for _, s := range sandboxes {
			byName[s.Name] = s
		}
		for _, name := range args {
			s, ok := byName[name]
			if !ok {
				return fmt.Errorf("sandbox not found: %s", name)
			}
			if isRunning(s.Status) && !flagAll {
				return fmt.Errorf("sandbox %s is running; stop it first or use --all", name)
			}
			targets = append(targets, s)
		}
	} else {
		for _, s := range sandboxes {
			if flagAll || !isRunning(s.Status) {
				targets = append(targets, s)
			}
		}
		if len(targets) == 0 {
			fmt.Println("no stopped sandboxes")
			return nil
		}
	}

	croot, err := containerdRoot()
	if err != nil {
		return err
	}
	root := filepath.Join(xdgDataHome(), "akitools", "sbx-backup")

	for _, s := range targets {
		if isRunning(s.Status) {
			fmt.Printf("note: %s is running; codex/opencode logs may be stale\n", s.Name)
		}
	}

	// Codex/opencode logs live in anonymous snapshots, so they are always
	// backed up for every sandbox at once, regardless of the targets.
	total, err := backupSnapshots(croot, root)
	if err != nil {
		return err
	}

	for _, s := range targets {
		n, err := backupClaude(s, croot, root)
		total += n
		if err != nil {
			return fmt.Errorf("%s: %w", s.Name, err)
		}
	}
	fmt.Printf("saved %d files to %s\n", total, displayPath(root))

	if flagKeep {
		return nil
	}
	// Running sandboxes are only ever backed up (--all), never removed.
	var stopped []string
	for _, s := range targets {
		if !isRunning(s.Status) {
			stopped = append(stopped, s.Name)
		}
	}
	return removeSandboxes(stopped)
}

// printFiles lists freshly copied files relative to the backup root.
func printFiles(root string, files []string) {
	for _, f := range files {
		rel, err := filepath.Rel(root, f)
		if err != nil {
			rel = f
		}
		fmt.Printf("  %s\n", rel)
	}
}

func displayPath(p string) string {
	home, err := os.UserHomeDir()
	if err == nil && strings.HasPrefix(p, home+string(filepath.Separator)) {
		return "~" + p[len(home):]
	}
	return p
}

func listSandboxes() ([]sandbox, error) {
	out, err := exec.Command("sbx", "ls", "--json").Output()
	if err != nil {
		return nil, fmt.Errorf("sbx ls --json: %w", err)
	}
	return parseSandboxes(out)
}

func parseSandboxes(data []byte) ([]sandbox, error) {
	var v any
	if err := json.Unmarshal(data, &v); err != nil {
		return nil, fmt.Errorf("parse sbx ls output: %w", err)
	}
	items, ok := v.([]any)
	if !ok {
		m, ok := v.(map[string]any)
		if !ok {
			return nil, errors.New("unexpected sbx ls output format")
		}
		for _, key := range []string{"sandboxes", "items"} {
			if a, ok := m[key].([]any); ok {
				items = a
				break
			}
		}
		if items == nil {
			return nil, errors.New("unexpected sbx ls output format")
		}
	}
	var out []sandbox
	for _, it := range items {
		m, ok := it.(map[string]any)
		if !ok {
			continue
		}
		s := sandbox{
			Name:   stringField(m, "name", "Name", "id", "ID"),
			ID:     stringField(m, "id", "ID"),
			Agent:  stringField(m, "agent", "Agent"),
			Status: stringField(m, "status", "state", "Status", "State"),
		}
		if s.Name != "" {
			out = append(out, s)
		}
	}
	return out, nil
}

func stringField(m map[string]any, keys ...string) string {
	for _, k := range keys {
		if v, ok := m[k].(string); ok && v != "" {
			return v
		}
	}
	return ""
}

func isRunning(status string) bool {
	return strings.Contains(strings.ToLower(status), "running")
}

func xdgDataHome() string {
	if v := os.Getenv("XDG_DATA_HOME"); v != "" {
		return v
	}
	home, err := os.UserHomeDir()
	if err != nil {
		return ".local/share"
	}
	return filepath.Join(home, ".local", "share")
}

func containerdRoot() (string, error) {
	home, err := os.UserHomeDir()
	if err != nil {
		return "", err
	}
	root := filepath.Join(home, "Library", "Application Support",
		"com.docker.sandboxes", "sandboxes", "sandboxd", "containerd", "root")
	if _, err := os.Stat(root); err != nil {
		return "", fmt.Errorf("containerd root not found: %w", err)
	}
	return root, nil
}

type ext4Image struct {
	disk *disk.Disk
	fsys filesystem.FileSystem
}

func openExt4(imgPath string) (*ext4Image, error) {
	d, err := diskfs.Open(imgPath, diskfs.WithOpenMode(diskfs.ReadOnly))
	if err != nil {
		return nil, err
	}
	fsys, err := d.GetFilesystem(0)
	if err != nil {
		d.Close()
		return nil, err
	}
	return &ext4Image{disk: d, fsys: fsys}, nil
}

func (i *ext4Image) Close() error {
	return i.disk.Close()
}

// readFile reads a whole file out of fsys in a single Read call.
//
// go-diskfs's ext4 File.Read panics ("makeslice: len out of range") when a
// read starts exactly at an extent boundary, which fs.ReadFile triggers on
// any multi-extent file via io.ReadAll's repeated small reads. A single
// full-size Read never re-enters at a nonzero offset, avoiding the bug.
func readFile(fsys fs.FS, p string) ([]byte, error) {
	f, err := fsys.Open(p)
	if err != nil {
		return nil, err
	}
	defer f.Close()
	info, err := f.Stat()
	if err != nil {
		return nil, err
	}
	buf := make([]byte, info.Size())
	if _, err := io.ReadFull(f, buf); err != nil {
		return nil, err
	}
	return buf, nil
}

// backupSnapshots extracts Codex sessions and opencode databases from every
// snapshot's rwlayer.img into the backup root, returning the number of files
// saved.
func backupSnapshots(croot, root string) (int, error) {
	entries, err := os.ReadDir(filepath.Join(croot, snapshotsRel))
	if err != nil {
		if errors.Is(err, fs.ErrNotExist) {
			return 0, nil
		}
		return 0, err
	}
	total := 0
	var failed []string
	for _, e := range entries {
		if !e.IsDir() {
			continue
		}
		img := filepath.Join(croot, snapshotsRel, e.Name(), "rwlayer.img")
		if _, err := os.Stat(img); err != nil {
			continue
		}
		n, err := backupSnapshot(img, root)
		total += n
		if err != nil {
			fmt.Printf("warning: snapshot %s: %s\n", e.Name(), err)
			failed = append(failed, e.Name())
		}
	}
	if len(failed) > 0 {
		return total, fmt.Errorf("failed to backup snapshots: %s", strings.Join(failed, ", "))
	}
	return total, nil
}

func backupSnapshot(img, root string) (n int, err error) {
	// go-diskfs panics on some ext4 layouts; contain it to this snapshot.
	defer func() {
		if r := recover(); r != nil {
			err = fmt.Errorf("panic: %v", r)
		}
	}()

	ext4, err := openExt4(img)
	if err != nil {
		return 0, err
	}
	defer ext4.Close()
	files, err := copyTree(ext4.fsys, codexSessionsInner, filepath.Join(root, "codex", "sessions"))
	printFiles(root, files)
	n = len(files)
	if err != nil {
		return n, err
	}
	saved, err := backupOpencodeDB(ext4.fsys, root)
	printFiles(root, saved)
	n += len(saved)
	return n, err
}

// backupOpencodeDB copies opencode.db (+ WAL/SHM sidecars) out of a rwlayer
// filesystem. Snapshots cannot be attributed to a sandbox, so the file is
// named by the hash of the main db, which also makes re-runs idempotent.
func backupOpencodeDB(fsys fs.FS, root string) ([]string, error) {
	db, err := readFile(fsys, opencodeDBInner)
	if err != nil {
		return nil, nil
	}
	sum := sha256.Sum256(db)
	dst := filepath.Join(root, "opencode", hex.EncodeToString(sum[:6])+".opencode.db")
	if err := writeFile(dst, db); err != nil {
		return nil, err
	}
	saved := []string{dst}
	for _, suffix := range []string{"-wal", "-shm"} {
		data, err := readFile(fsys, opencodeDBInner+suffix)
		if err != nil {
			continue
		}
		if err := writeFile(dst+suffix, data); err != nil {
			return saved, err
		}
		saved = append(saved, dst+suffix)
	}
	return saved, nil
}

// backupClaude copies the sandbox's Claude Code project volumes, returning
// the number of files saved.
func backupClaude(s sandbox, croot, root string) (int, error) {
	total := 0
	for _, img := range claudeVolumeImages(croot, s.Name) {
		files, err := backupVolume(img, filepath.Join(root, "claude", "projects"))
		printFiles(root, files)
		total += len(files)
		if err != nil {
			return total, fmt.Errorf("%s: %w", filepath.Base(img), err)
		}
	}
	return total, nil
}

func backupVolume(img, dstRoot string) (files []string, err error) {
	defer func() {
		if r := recover(); r != nil {
			err = fmt.Errorf("panic: %v", r)
		}
	}()

	ext4, err := openExt4(img)
	if err != nil {
		fmt.Printf("skip %s: %s\n", filepath.Base(img), err)
		return nil, nil
	}
	defer ext4.Close()
	return backupClaudeVolume(ext4.fsys, dstRoot)
}

// claudeVolumeImages lists the block volume images belonging to the named
// sandbox. The sandbox name may contain '-', so match on the full prefix.
func claudeVolumeImages(croot, name string) []string {
	dir := filepath.Join(croot, volumeImagesRel)
	entries, err := os.ReadDir(dir)
	if err != nil {
		return nil
	}
	prefix := "sandbox-plugin-" + name + "-claude-"
	var out []string
	for _, e := range entries {
		if !e.Type().IsRegular() {
			continue
		}
		if strings.HasPrefix(e.Name(), prefix) && strings.HasSuffix(e.Name(), ".img") {
			out = append(out, filepath.Join(dir, e.Name()))
		}
	}
	return out
}

// backupClaudeVolume copies every '-<encoded-cwd>' project dir found at the
// volume root. Non-projects volumes (sessions/todos/...) have none and
// contribute nothing.
func backupClaudeVolume(fsys fs.FS, dstRoot string) ([]string, error) {
	entries, err := fs.ReadDir(fsys, ".")
	if err != nil {
		return nil, err
	}
	var copied []string
	for _, e := range entries {
		if !e.IsDir() || !strings.HasPrefix(e.Name(), "-") {
			continue
		}
		files, err := copyTree(fsys, e.Name(), filepath.Join(dstRoot, e.Name()))
		copied = append(copied, files...)
		if err != nil {
			return copied, err
		}
	}
	return copied, nil
}

// copyTree merge-copies all regular files under src (a directory inside fsys)
// into dstRoot, preserving mtimes, and returns the destination paths. A
// missing src is not an error.
func copyTree(fsys fs.FS, src, dstRoot string) ([]string, error) {
	var copied []string
	err := fs.WalkDir(fsys, src, func(p string, d fs.DirEntry, err error) error {
		if err != nil {
			if p == src {
				return fs.SkipAll
			}
			return err
		}
		if d.IsDir() {
			if d.Name() == "lost+found" {
				return fs.SkipDir
			}
			return nil
		}
		if !d.Type().IsRegular() {
			return nil
		}
		rel := strings.TrimPrefix(p, src)
		rel = strings.TrimPrefix(rel, "/")
		if rel == "" {
			rel = path.Base(p)
		}
		data, err := readFile(fsys, p)
		if err != nil {
			return fmt.Errorf("read %s: %w", p, err)
		}
		dst := filepath.Join(dstRoot, filepath.FromSlash(rel))
		if err := writeFile(dst, data); err != nil {
			return err
		}
		if info, err := d.Info(); err == nil {
			_ = os.Chtimes(dst, info.ModTime(), info.ModTime())
		}
		copied = append(copied, dst)
		return nil
	})
	return copied, err
}

func writeFile(dst string, data []byte) error {
	if err := os.MkdirAll(filepath.Dir(dst), 0o755); err != nil {
		return err
	}
	return os.WriteFile(dst, data, 0o644)
}

// removeSandboxes confirms and removes the named sandboxes in one batch.
func removeSandboxes(names []string) error {
	if len(names) == 0 {
		return nil
	}
	if !confirmRemoval(names) {
		fmt.Printf("kept %s\n", strings.Join(names, ", "))
		return nil
	}
	for _, name := range names {
		if err := removeSandbox(name); err != nil {
			return fmt.Errorf("%s: %w", name, err)
		}
	}
	return nil
}

func confirmRemoval(names []string) bool {
	if !isatty.IsTerminal(os.Stdin.Fd()) {
		fmt.Println("stdin is not a terminal; skipping removal")
		return false
	}
	fmt.Println("The following stopped sandboxes will be removed:")
	for _, name := range names {
		fmt.Printf("  %s\n", name)
	}
	fmt.Print("Do you want to continue? [y/N]: ")
	line, err := bufio.NewReader(os.Stdin).ReadString('\n')
	if err != nil {
		return false
	}
	answer := strings.ToLower(strings.TrimSpace(line))
	return answer == "y" || answer == "yes"
}

func removeSandbox(name string) error {
	cmd := exec.Command("sbx", "rm", "--force", name)
	cmd.Stdout = os.Stdout
	cmd.Stderr = os.Stderr
	return cmd.Run()
}
