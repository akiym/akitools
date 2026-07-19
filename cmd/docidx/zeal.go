package docidx

import (
	"archive/tar"
	"bufio"
	"compress/gzip"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"os"
	"os/exec"
	"path/filepath"
	"sort"
	"strconv"
	"strings"
	"time"

	"github.com/mattn/go-isatty"
)

const (
	zealAPIURL      = "https://zealusercontributions.vercel.app/api/docsets"
	zealCacheFile   = "zeal-docsets.json"
	zealCacheMaxAge = 24 * time.Hour
	zealMenuLimit   = 50
)

type zealAuthor struct {
	Name string `json:"name"`
	Link string `json:"link"`
}

type zealDocset struct {
	Name    string     `json:"name"`
	Aliases []string   `json:"aliases"`
	Archive string     `json:"archive"`
	Version string     `json:"version"`
	URLs    []string   `json:"urls"`
	Author  zealAuthor `json:"author"`
}

func zealCachePath() (string, error) {
	cacheDir, err := os.UserCacheDir()
	if err != nil {
		return "", err
	}
	return filepath.Join(cacheDir, "docidx", zealCacheFile), nil
}

func loadZealDocsets(refresh bool) ([]zealDocset, error) {
	p, err := zealCachePath()
	if err != nil {
		return nil, err
	}
	if !refresh {
		if fi, err := os.Stat(p); err == nil && time.Since(fi.ModTime()) < zealCacheMaxAge {
			if out, err := readZealCache(p); err == nil && len(out) > 0 {
				return out, nil
			}
		}
	}
	return fetchZealDocsets(p)
}

func readZealCache(path string) ([]zealDocset, error) {
	f, err := os.Open(path)
	if err != nil {
		return nil, err
	}
	defer f.Close()
	var out []zealDocset
	if err := json.NewDecoder(f).Decode(&out); err != nil {
		return nil, err
	}
	return out, nil
}

func fetchZealDocsets(cachePath string) ([]zealDocset, error) {
	fmt.Fprintln(os.Stderr, "fetching docset list from zealusercontributions...")
	req, err := http.NewRequest(http.MethodGet, zealAPIURL, nil)
	if err != nil {
		return nil, err
	}
	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("zeal API %s: %s", zealAPIURL, resp.Status)
	}
	var out []zealDocset
	if err := json.NewDecoder(resp.Body).Decode(&out); err != nil {
		return nil, fmt.Errorf("decode zeal API: %w", err)
	}
	sort.Slice(out, func(i, j int) bool {
		return strings.ToLower(out[i].Name) < strings.ToLower(out[j].Name)
	})
	if err := os.MkdirAll(filepath.Dir(cachePath), 0o755); err == nil {
		if f, err := os.Create(cachePath); err == nil {
			_ = json.NewEncoder(f).Encode(out)
			f.Close()
		}
	}
	return out, nil
}

func zealHaystack(d zealDocset) string {
	return strings.ToLower(d.Name + " " + d.Author.Name)
}

func filterZealDocsets(all []zealDocset, query string) []zealDocset {
	q := strings.TrimSpace(query)
	if q == "" {
		return all
	}
	terms := strings.Fields(strings.ToLower(q))
	out := make([]zealDocset, 0, len(all))
	for _, d := range all {
		hay := zealHaystack(d)
		match := true
		for _, t := range terms {
			if !strings.Contains(hay, t) {
				match = false
				break
			}
		}
		if match {
			out = append(out, d)
		}
	}
	return out
}

func pickZealDocset(all []zealDocset, query string) (*zealDocset, error) {
	filtered := filterZealDocsets(all, query)
	if len(filtered) == 0 {
		if query == "" {
			return nil, errors.New("docset list is empty")
		}
		return nil, fmt.Errorf("no docsets match %q", query)
	}
	if len(filtered) == 1 {
		return &filtered[0], nil
	}
	if isatty.IsTerminal(os.Stdin.Fd()) {
		if _, err := exec.LookPath("fzf"); err == nil {
			return fzfPickZeal(filtered)
		}
	}
	return numberedPickZeal(filtered)
}

func fzfPickZeal(items []zealDocset) (*zealDocset, error) {
	var buf strings.Builder
	for i, d := range items {
		display := fmt.Sprintf("%s  v%s", d.Name, d.Version)
		if d.Author.Name != "" {
			display += "  by " + d.Author.Name
		}
		fmt.Fprintf(&buf, "%d\t%s\n", i, display)
	}
	cmd := exec.Command("fzf",
		"--with-nth=2..",
		"--delimiter=\t",
		"--ansi",
		"--prompt=zeal docset > ",
		"--no-multi",
		"--height=80%",
		"--reverse",
	)
	cmd.Stdin = strings.NewReader(buf.String())
	cmd.Stderr = os.Stderr
	out, err := cmd.Output()
	if err != nil {
		var exitErr *exec.ExitError
		if errors.As(err, &exitErr) && exitErr.ExitCode() == 130 {
			return nil, nil
		}
		return nil, fmt.Errorf("fzf: %w", err)
	}
	line := strings.TrimRight(string(out), "\n")
	if line == "" {
		return nil, nil
	}
	parts := strings.SplitN(line, "\t", 3)
	idx, err := strconv.Atoi(parts[0])
	if err != nil || idx < 0 || idx >= len(items) {
		return nil, fmt.Errorf("unexpected fzf output: %q", line)
	}
	return &items[idx], nil
}

func numberedPickZeal(items []zealDocset) (*zealDocset, error) {
	total := len(items)
	shown := total
	if shown > zealMenuLimit {
		shown = zealMenuLimit
	}
	for i := 0; i < shown; i++ {
		d := items[i]
		line := fmt.Sprintf("%3d  %s\tv%s", i+1, d.Name, d.Version)
		if d.Author.Name != "" {
			line += "\tby " + d.Author.Name
		}
		fmt.Fprintln(os.Stderr, line)
	}
	if total > shown {
		fmt.Fprintf(os.Stderr, "... (%d more; narrow the query)\n", total-shown)
	}
	fmt.Fprint(os.Stderr, "select number (blank to cancel): ")
	line, err := bufio.NewReader(os.Stdin).ReadString('\n')
	if err != nil && !errors.Is(err, io.EOF) {
		return nil, err
	}
	line = strings.TrimSpace(line)
	if line == "" {
		return nil, nil
	}
	n, err := strconv.Atoi(line)
	if err != nil || n < 1 || n > total {
		return nil, fmt.Errorf("invalid selection %q", line)
	}
	return &items[n-1], nil
}

func downloadAndExtractZeal(ctx context.Context, d zealDocset, outDir string) (string, error) {
	if outDir == "" {
		outDir = "."
	}
	if err := os.MkdirAll(outDir, 0o755); err != nil {
		return "", err
	}
	if len(d.URLs) == 0 {
		return "", fmt.Errorf("%s: no download URLs in API response", d.Name)
	}
	absOut, err := filepath.Abs(outDir)
	if err != nil {
		return "", err
	}
	var lastErr error
	for _, u := range d.URLs {
		fmt.Fprintf(os.Stderr, "downloading %s...\n", u)
		root, err := downloadOneZeal(ctx, u, absOut)
		if err != nil {
			fmt.Fprintf(os.Stderr, "  %v\n", err)
			lastErr = err
			continue
		}
		return filepath.Join(outDir, root), nil
	}
	return "", fmt.Errorf("all mirrors failed: %w", lastErr)
}

func downloadOneZeal(ctx context.Context, url, absOut string) (string, error) {
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, url, nil)
	if err != nil {
		return "", err
	}
	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		return "", err
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		return "", fmt.Errorf("%s: %s", url, resp.Status)
	}
	return extractTarGz(resp.Body, absOut)
}

func extractTarGz(r io.Reader, absDest string) (string, error) {
	gz, err := gzip.NewReader(r)
	if err != nil {
		return "", err
	}
	defer gz.Close()
	tr := tar.NewReader(gz)
	root := ""
	sep := string(filepath.Separator)
	for {
		hdr, err := tr.Next()
		if errors.Is(err, io.EOF) {
			break
		}
		if err != nil {
			return "", err
		}
		name := filepath.Clean(hdr.Name)
		if name == "." || name == ".." || strings.HasPrefix(name, ".."+sep) {
			continue
		}
		target := filepath.Join(absDest, name)
		if target != absDest && !strings.HasPrefix(target, absDest+sep) {
			return "", fmt.Errorf("illegal path in archive: %s", hdr.Name)
		}
		if first := strings.SplitN(name, sep, 2)[0]; root == "" && first != "" {
			root = first
		}
		mode := hdr.FileInfo().Mode()
		switch {
		case mode.IsDir():
			if err := os.MkdirAll(target, 0o755); err != nil {
				return "", err
			}
		case mode.IsRegular():
			if err := os.MkdirAll(filepath.Dir(target), 0o755); err != nil {
				return "", err
			}
			f, err := os.OpenFile(target, os.O_CREATE|os.O_WRONLY|os.O_TRUNC, mode.Perm()|0o200)
			if err != nil {
				return "", err
			}
			if _, err := io.Copy(f, tr); err != nil {
				f.Close()
				return "", err
			}
			if err := f.Close(); err != nil {
				return "", err
			}
		case mode&os.ModeSymlink != 0:
			if err := os.MkdirAll(filepath.Dir(target), 0o755); err != nil {
				return "", err
			}
			_ = os.Remove(target)
			if err := os.Symlink(hdr.Linkname, target); err != nil {
				return "", err
			}
		}
	}
	if root == "" {
		return "", errors.New("empty archive")
	}
	return root, nil
}
