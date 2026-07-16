package docidx

import (
	"os"
	"path/filepath"
	"reflect"
	"strings"
	"testing"
)

func buildTestIndex(t *testing.T) string {
	t.Helper()
	dir := t.TempDir()
	docs := filepath.Join(dir, "docs")

	files := map[string]string{
		"tutorials/instancing.md": `# Instancing scenes

How to create instances of a scene at runtime.

## Spawning enemies

Load a PackedScene and call instantiate for each enemy.
`,
		"classes/animationplayer.md": `# AnimationPlayer

Node for playing animations.

## play()

Starts playing the current animation.

## stop()

Stops the running animation.
`,
		"guides/saving.md": `# Saving games

Persist game state with FileAccess. Instancing is unrelated here but
mentioned once in the body for ranking tests.
`,
		"classes/class_vector2.rst": `.. _class_Vector2:

Vector2
=======

2D vector class used for math.

Method Descriptions
-------------------

.. _class_Vector2_method_angle:

float **angle**\ (\ )

Returns the angle of the vector.
`,
	}
	for name, content := range files {
		path := filepath.Join(docs, filepath.FromSlash(name))
		if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
			t.Fatal(err)
		}
		if err := os.WriteFile(path, []byte(content), 0o644); err != nil {
			t.Fatal(err)
		}
	}

	dbPath := filepath.Join(dir, "index.db")
	nfiles, nchunks, err := buildIndex(dbPath, docs, nil)
	if err != nil {
		t.Fatal(err)
	}
	if nfiles != 4 {
		t.Fatalf("indexed %d files, want 4", nfiles)
	}
	if nchunks == 0 {
		t.Fatal("no chunks indexed")
	}
	return dbPath
}

func TestBuildSearchCat(t *testing.T) {
	dbPath := buildTestIndex(t)
	db, err := openIndex(dbPath)
	if err != nil {
		t.Fatal(err)
	}
	defer db.Close()

	results, err := searchIndex(db, "instancing", nil, 30)
	if err != nil {
		t.Fatal(err)
	}
	if len(results) < 2 {
		t.Fatalf("got %d results, want >= 2", len(results))
	}
	if results[0].Path != "tutorials/instancing.md" {
		t.Errorf("title match should outrank body match, got %+v", results[0])
	}

	chunk, err := getChunk(db, results[0].ID)
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(chunk.Body, "create instances") {
		t.Errorf("cat body = %q", chunk.Body)
	}

	if _, err := getChunk(db, 99999); err == nil {
		t.Error("getChunk with unknown id should error")
	}
}

func TestSearchAliasExpansion(t *testing.T) {
	dbPath := buildTestIndex(t)
	db, err := openIndex(dbPath)
	if err != nil {
		t.Fatal(err)
	}
	defer db.Close()

	dict := aliasDict{"save": {"FileAccess"}}
	results, err := searchIndex(db, "save", dict, 30)
	if err != nil {
		t.Fatal(err)
	}
	found := false
	for _, r := range results {
		if r.Path == "guides/saving.md" {
			found = true
		}
	}
	if !found {
		t.Errorf("alias expansion did not surface guides/saving.md: %+v", results)
	}
}

func TestSearchOrFallback(t *testing.T) {
	dbPath := buildTestIndex(t)
	db, err := openIndex(dbPath)
	if err != nil {
		t.Fatal(err)
	}
	defer db.Close()

	// "nonexistentword" matches nothing, so AND returns 0 rows; the OR
	// fallback should still surface documents matching "enemies".
	results, err := searchIndex(db, "nonexistentword enemies", nil, 30)
	if err != nil {
		t.Fatal(err)
	}
	if len(results) == 0 {
		t.Fatal("OR fallback returned no results")
	}
	if results[0].Title != "Spawning enemies" {
		t.Errorf("results[0] = %+v", results[0])
	}
	for _, r := range results {
		if !r.Fallback {
			t.Errorf("OR-fallback row not marked: %+v", r)
		}
	}

	// All-terms matches must not be marked as fallback.
	exact, err := searchIndex(db, "instancing", nil, 30)
	if err != nil {
		t.Fatal(err)
	}
	if len(exact) == 0 || exact[0].Fallback {
		t.Errorf("AND match marked as fallback: %+v", exact)
	}
}

func TestOpenIndexMissing(t *testing.T) {
	if _, err := openIndex(filepath.Join(t.TempDir(), "missing.db")); err == nil {
		t.Error("openIndex on missing file should error")
	}
}

func TestGetPageChunks(t *testing.T) {
	dbPath := buildTestIndex(t)
	db, err := openIndex(dbPath)
	if err != nil {
		t.Fatal(err)
	}
	defer db.Close()

	chunks, err := getPageChunks(db, "classes/animationplayer.md")
	if err != nil {
		t.Fatal(err)
	}
	var titles []string
	for _, c := range chunks {
		titles = append(titles, c.Title)
	}
	want := []string{"AnimationPlayer", "play()", "stop()"}
	if !reflect.DeepEqual(titles, want) {
		t.Errorf("page chunks = %v, want %v", titles, want)
	}

	if _, err := getPageChunks(db, "classes/missing.md"); err == nil {
		t.Error("unknown path should error")
	}

	withAnchor, err := getPageChunks(db, "classes/animationplayer.md#play")
	if err != nil {
		t.Fatalf("anchor suffix should be ignored: %v", err)
	}
	if len(withAnchor) != len(chunks) {
		t.Errorf("got %d chunks with anchor suffix, want %d", len(withAnchor), len(chunks))
	}
}

func TestGetPageChunksHashInFilename(t *testing.T) {
	dir := t.TempDir()
	docs := filepath.Join(dir, "docs")
	if err := os.MkdirAll(docs, 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(docs, "c#.md"), []byte("# CSharp\n\nBindings.\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	dbPath := filepath.Join(dir, "index.db")
	if _, _, err := buildIndex(dbPath, docs, nil); err != nil {
		t.Fatal(err)
	}
	db, err := openIndex(dbPath)
	if err != nil {
		t.Fatal(err)
	}
	defer db.Close()

	chunks, err := getPageChunks(db, "c#.md")
	if err != nil {
		t.Fatalf("exact path with '#' should match before anchor stripping: %v", err)
	}
	if len(chunks) != 1 || chunks[0].Title != "CSharp" {
		t.Errorf("chunks = %+v", chunks)
	}
}

func TestBuildIndexRefusesNonSQLiteDB(t *testing.T) {
	dir := t.TempDir()
	docs := filepath.Join(dir, "docs")
	if err := os.MkdirAll(docs, 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(docs, "a.md"), []byte("# A\n\nText.\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	precious := filepath.Join(dir, "precious.txt")
	if err := os.WriteFile(precious, []byte("precious data"), 0o644); err != nil {
		t.Fatal(err)
	}

	if _, _, err := buildIndex(precious, docs, nil); err == nil {
		t.Fatal("build over a non-SQLite file should be refused")
	}
	data, err := os.ReadFile(precious)
	if err != nil || string(data) != "precious data" {
		t.Errorf("existing file was destroyed: %q, %v", data, err)
	}
}

func TestBuildIndexRejectsNonDirectory(t *testing.T) {
	dir := t.TempDir()
	file := filepath.Join(dir, "readme.md")
	if err := os.WriteFile(file, []byte("# A\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	if _, _, err := buildIndex(filepath.Join(dir, "index.db"), file, nil); err == nil {
		t.Fatal("plain file as docs-dir should be rejected")
	}
}

func TestCollectDocFilesExclude(t *testing.T) {
	root := t.TempDir()
	for _, name := range []string{
		"index.html",
		"class_foo.html",
		"foo_8h_source.html",
		"api/index.html",
		"api/bar_8h_source.html",
		"search/all.html",
	} {
		path := filepath.Join(root, filepath.FromSlash(name))
		if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
			t.Fatal(err)
		}
		if err := os.WriteFile(path, []byte("<html></html>"), 0o644); err != nil {
			t.Fatal(err)
		}
	}

	files, err := collectDocFiles(root, []string{"*_source.html", "/index.html", "search/"})
	if err != nil {
		t.Fatal(err)
	}
	var rels []string
	for _, f := range files {
		rel, _ := filepath.Rel(root, f)
		rels = append(rels, filepath.ToSlash(rel))
	}
	want := []string{"api/index.html", "class_foo.html"}
	if !reflect.DeepEqual(rels, want) {
		t.Errorf("collectDocFiles = %v, want %v", rels, want)
	}
}
