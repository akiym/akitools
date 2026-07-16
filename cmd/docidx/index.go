package docidx

import (
	"bytes"
	"database/sql"
	"errors"
	"fmt"
	"io"
	"io/fs"
	"os"
	"path/filepath"
	"strings"

	ignore "github.com/sabhiram/go-gitignore"
	_ "modernc.org/sqlite"
)

const schema = `
CREATE TABLE documents (
	id INTEGER PRIMARY KEY,
	path TEXT NOT NULL,
	title TEXT NOT NULL,
	headings TEXT NOT NULL,
	breadcrumbs TEXT NOT NULL,
	body TEXT NOT NULL,
	anchor TEXT NOT NULL,
	kind TEXT NOT NULL
);
CREATE INDEX documents_path ON documents (path);
CREATE VIRTUAL TABLE documents_fts USING fts5(
	title, headings, breadcrumbs, body,
	content='documents',
	content_rowid='id',
	tokenize='porter unicode61'
);
`

// BM25 column weights for title, headings, breadcrumbs, body.
const bm25Weights = "8.0, 4.0, 2.0, 1.0"

func openIndex(dbPath string) (*sql.DB, error) {
	if _, err := os.Stat(dbPath); err != nil {
		return nil, fmt.Errorf("index %s not found (run `docidx build` first): %w", dbPath, err)
	}
	return sql.Open("sqlite", dbPath)
}

func buildIndex(dbPath, docsDir string, excludes []string) (files, chunks int, err error) {
	if fi, err := os.Stat(docsDir); err != nil {
		return 0, 0, err
	} else if !fi.IsDir() {
		return 0, 0, fmt.Errorf("%s is not a directory", docsDir)
	}

	docFiles, err := collectDocFiles(docsDir, excludes)
	if err != nil {
		return 0, 0, err
	}
	if len(docFiles) == 0 {
		return 0, 0, fmt.Errorf("no .md/.html files found under %s", docsDir)
	}

	if err := removeExistingIndex(dbPath); err != nil {
		return 0, 0, err
	}
	db, err := sql.Open("sqlite", dbPath)
	if err != nil {
		return 0, 0, err
	}
	defer db.Close()
	if _, err := db.Exec(schema); err != nil {
		return 0, 0, err
	}

	tx, err := db.Begin()
	if err != nil {
		return 0, 0, err
	}
	defer tx.Rollback()

	insertDoc, err := tx.Prepare(`INSERT INTO documents (path, title, headings, breadcrumbs, body, anchor, kind) VALUES (?, ?, ?, ?, ?, ?, ?)`)
	if err != nil {
		return 0, 0, err
	}

	for _, file := range docFiles {
		src, err := os.ReadFile(file)
		if err != nil {
			return 0, 0, err
		}
		relPath, err := filepath.Rel(docsDir, file)
		if err != nil {
			return 0, 0, err
		}
		relPath = filepath.ToSlash(relPath)

		var cs []Chunk
		switch strings.ToLower(filepath.Ext(file)) {
		case ".md", ".markdown":
			cs = chunkMarkdown(relPath, src)
		case ".html", ".htm":
			cs, err = chunkHTML(relPath, src)
			if err != nil {
				return 0, 0, fmt.Errorf("%s: %w", relPath, err)
			}
		}
		for _, c := range cs {
			if _, err := insertDoc.Exec(c.Path, c.Title, c.Headings, c.Breadcrumbs, c.Body, c.Anchor, c.Kind); err != nil {
				return 0, 0, err
			}
		}
		files++
		chunks += len(cs)
	}

	// Populate the external-content FTS table from documents in one pass.
	if _, err := tx.Exec(`INSERT INTO documents_fts (documents_fts) VALUES ('rebuild')`); err != nil {
		return 0, 0, err
	}

	if err := tx.Commit(); err != nil {
		return 0, 0, err
	}
	return files, chunks, nil
}

// removeExistingIndex deletes a previous index at path, refusing to touch a
// file that is not an SQLite database (e.g. a mistyped --db).
func removeExistingIndex(path string) error {
	f, err := os.Open(path)
	if os.IsNotExist(err) {
		return nil
	}
	if err != nil {
		return err
	}
	header := make([]byte, 16)
	n, _ := io.ReadFull(f, header)
	f.Close()
	if n > 0 && !bytes.HasPrefix(header[:n], []byte("SQLite format 3\x00")) {
		return fmt.Errorf("refusing to overwrite %s: not an SQLite database", path)
	}
	return os.Remove(path)
}

func collectDocFiles(root string, excludes []string) ([]string, error) {
	var matcher *ignore.GitIgnore
	if len(excludes) > 0 {
		matcher = ignore.CompileIgnoreLines(excludes...)
	}
	var files []string
	err := filepath.WalkDir(root, func(p string, d fs.DirEntry, err error) error {
		if err != nil {
			return err
		}
		var rel string
		if p != root {
			if rel, err = filepath.Rel(root, p); err != nil {
				return err
			}
			rel = filepath.ToSlash(rel)
		}
		if d.IsDir() {
			if p != root && (strings.HasPrefix(d.Name(), ".") || d.Name() == "node_modules") {
				return filepath.SkipDir
			}
			// As with git, files inside an excluded directory cannot be
			// re-included by a negated pattern.
			if matcher != nil && p != root && matcher.MatchesPath(rel+"/") {
				return filepath.SkipDir
			}
			return nil
		}
		switch strings.ToLower(filepath.Ext(d.Name())) {
		case ".md", ".markdown", ".html", ".htm":
			if matcher != nil && matcher.MatchesPath(rel) {
				return nil
			}
			files = append(files, p)
		}
		return nil
	})
	return files, err
}

type searchResult struct {
	ID      int64
	Score   float64
	Path    string
	Anchor  string
	Title   string
	Kind    string
	BodyLen int64
	// Fallback marks rows appended from the OR query, which match only
	// some of the query terms.
	Fallback bool
}

// searchIndex runs the AND form of the query first; if it fills fewer than
// limit results, the OR form backfills the remainder so that partial matches
// still surface (recall over precision).
func searchIndex(db *sql.DB, query string, dict aliasDict, limit int) ([]searchResult, error) {
	andQuery, orQuery := buildFTSQueries(query, dict)
	if andQuery == "" {
		return nil, errors.New("empty query")
	}

	results, err := runFTSQuery(db, andQuery, limit)
	if err != nil {
		return nil, err
	}
	if len(results) >= limit || orQuery == andQuery {
		return results, nil
	}

	seen := make(map[int64]bool, len(results))
	for _, r := range results {
		seen[r.ID] = true
	}
	more, err := runFTSQuery(db, orQuery, limit)
	if err != nil {
		return nil, err
	}
	for _, r := range more {
		if len(results) >= limit {
			break
		}
		if !seen[r.ID] {
			r.Fallback = true
			results = append(results, r)
		}
	}
	return results, nil
}

func runFTSQuery(db *sql.DB, ftsQuery string, limit int) ([]searchResult, error) {
	rows, err := db.Query(fmt.Sprintf(`
		SELECT d.id, bm25(documents_fts, %s) AS score, d.path, d.anchor, d.title, d.kind, length(d.body)
		FROM documents_fts
		JOIN documents d ON d.id = documents_fts.rowid
		WHERE documents_fts MATCH ?
		ORDER BY score
		LIMIT ?`, bm25Weights), ftsQuery, limit)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var results []searchResult
	for rows.Next() {
		var r searchResult
		if err := rows.Scan(&r.ID, &r.Score, &r.Path, &r.Anchor, &r.Title, &r.Kind, &r.BodyLen); err != nil {
			return nil, err
		}
		results = append(results, r)
	}
	return results, rows.Err()
}

const chunkColumns = "path, title, headings, breadcrumbs, body, anchor, kind"

func scanChunk(scan func(...any) error) (Chunk, error) {
	var c Chunk
	err := scan(&c.Path, &c.Title, &c.Headings, &c.Breadcrumbs, &c.Body, &c.Anchor, &c.Kind)
	return c, err
}

// getPageChunks returns all chunks of one source file in document order,
// allowing a whole page to be reassembled from the index alone. If the exact
// path does not exist, a "#anchor" suffix is stripped and retried, so paths
// pasted from search output work without breaking files whose real name
// contains '#'.
func getPageChunks(db *sql.DB, path string) ([]Chunk, error) {
	chunks, err := queryPageChunks(db, path)
	if err != nil {
		return nil, err
	}
	if len(chunks) == 0 {
		if i := strings.IndexByte(path, '#'); i >= 0 {
			if chunks, err = queryPageChunks(db, path[:i]); err != nil {
				return nil, err
			}
		}
	}
	if len(chunks) == 0 {
		return nil, fmt.Errorf("no chunks for path %q", path)
	}
	return chunks, nil
}

func queryPageChunks(db *sql.DB, path string) ([]Chunk, error) {
	rows, err := db.Query(`SELECT `+chunkColumns+` FROM documents WHERE path = ? ORDER BY id`, path)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var chunks []Chunk
	for rows.Next() {
		c, err := scanChunk(rows.Scan)
		if err != nil {
			return nil, err
		}
		chunks = append(chunks, c)
	}
	return chunks, rows.Err()
}

func getChunk(db *sql.DB, id int64) (*Chunk, error) {
	c, err := scanChunk(db.QueryRow(`SELECT `+chunkColumns+` FROM documents WHERE id = ?`, id).Scan)
	if errors.Is(err, sql.ErrNoRows) {
		return nil, fmt.Errorf("no chunk with id %d", id)
	}
	if err != nil {
		return nil, err
	}
	return &c, nil
}
