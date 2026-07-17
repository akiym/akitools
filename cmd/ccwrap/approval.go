package ccwrap

import (
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
	"time"
)

// approval はワークスペースごとの承認記録。Files は承認時に確認対象だった
// ファイル(自動読み込みファイルとプロジェクト側settings)の相対パスと
// 内容のsha256。次回起動時に集合ごと比較し、差分がなければ確認を省略する
type approval struct {
	Version    int               `json:"version"`
	Path       string            `json:"path"`
	ApprovedAt time.Time         `json:"approvedAt"`
	Files      map[string]string `json:"files"`
}

func xdgStateBase() (string, error) {
	dir := os.Getenv("XDG_STATE_HOME")
	if dir == "" {
		home, err := os.UserHomeDir()
		if err != nil {
			return "", err
		}
		dir = filepath.Join(home, ".local", "state")
	}
	return dir, nil
}

func stateDir() (string, error) {
	base, err := xdgStateBase()
	if err != nil {
		return "", err
	}
	return filepath.Join(base, "akitools", "ccwrap"), nil
}

func approvalPath(cwd string) (string, error) {
	base, err := stateDir()
	if err != nil {
		return "", err
	}
	return filepath.Join(base, "approvals", strings.ReplaceAll(cwd, "/", "-")+".json"), nil
}

// loadApproval は保存済みの承認記録を返す。記録がない・壊れている・
// ワークスペース名の衝突でパスが一致しない場合はnilを返し、再確認に倒す
func loadApproval(cwd string) (*approval, error) {
	path, err := approvalPath(cwd)
	if err != nil {
		return nil, err
	}
	data, err := os.ReadFile(path)
	if err != nil {
		if os.IsNotExist(err) {
			return nil, nil
		}
		return nil, err
	}
	var a approval
	if err := json.Unmarshal(data, &a); err != nil {
		return nil, nil
	}
	if a.Path != cwd {
		return nil, nil
	}
	return &a, nil
}

func saveApproval(cwd string, files map[string]string) error {
	path, err := approvalPath(cwd)
	if err != nil {
		return err
	}
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		return err
	}
	a := approval{
		Version:    1,
		Path:       cwd,
		ApprovedAt: time.Now(),
		Files:      files,
	}
	data, err := json.MarshalIndent(a, "", "  ")
	if err != nil {
		return err
	}
	tmp := path + ".tmp"
	if err := os.WriteFile(tmp, append(data, '\n'), 0o644); err != nil {
		return err
	}
	return os.Rename(tmp, path)
}

func hashFile(path string) (string, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		return "", err
	}
	sum := sha256.Sum256(data)
	return hex.EncodeToString(sum[:]), nil
}

func diffApproval(saved *approval, current map[string]string) (added, changed, removed []string) {
	var approved map[string]string
	if saved != nil {
		approved = saved.Files
	}
	for _, f := range sortedKeys(current) {
		old, ok := approved[f]
		switch {
		case !ok:
			added = append(added, f)
		case old != current[f]:
			changed = append(changed, f)
		}
	}
	for _, f := range sortedKeys(approved) {
		if _, ok := current[f]; !ok {
			removed = append(removed, f)
		}
	}
	return added, changed, removed
}
