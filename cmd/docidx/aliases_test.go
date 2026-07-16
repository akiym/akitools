package docidx

import (
	"os"
	"path/filepath"
	"reflect"
	"testing"
)

func TestBuildFTSQueries(t *testing.T) {
	dict := aliasDict{"spawn": {"instantiate", "PackedScene"}}

	and, or := buildFTSQueries("spawn enemy", dict)
	wantAnd := `("spawn" OR "instantiate" OR "PackedScene") AND "enemy"`
	if and != wantAnd {
		t.Errorf("and = %q, want %q", and, wantAnd)
	}
	wantOr := `("spawn" OR "instantiate" OR "PackedScene") OR "enemy"`
	if or != wantOr {
		t.Errorf("or = %q, want %q", or, wantOr)
	}

	and, _ = buildFTSQueries("Spawn", dict)
	if and != `("Spawn" OR "instantiate" OR "PackedScene")` {
		t.Errorf("case-insensitive alias lookup failed: %q", and)
	}
}

func TestTokenizeQuery(t *testing.T) {
	got := tokenizeQuery("How do I spawn enemies? (AnimationPlayer.play)")
	want := []string{"How", "do", "I", "spawn", "enemies", "AnimationPlayer.play"}
	if !reflect.DeepEqual(got, want) {
		t.Errorf("tokenizeQuery = %v, want %v", got, want)
	}

	got = tokenizeQuery("spawn . enemy ...")
	want = []string{"spawn", "enemy"}
	if !reflect.DeepEqual(got, want) {
		t.Errorf("punctuation-only tokens not dropped: %v, want %v", got, want)
	}
}

func TestLoadAliases(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "aliases.json")
	if err := os.WriteFile(path, []byte(`{"Save": ["FileAccess", "ResourceSaver"]}`), 0o644); err != nil {
		t.Fatal(err)
	}

	dict, err := loadAliases(path, false)
	if err != nil {
		t.Fatal(err)
	}
	if !reflect.DeepEqual(dict["save"], []string{"FileAccess", "ResourceSaver"}) {
		t.Errorf("dict = %v", dict)
	}

	dict, err = loadAliases(filepath.Join(dir, "missing.json"), false)
	if err != nil || dict != nil {
		t.Errorf("missing optional file: dict=%v err=%v", dict, err)
	}

	if _, err := loadAliases(filepath.Join(dir, "missing.json"), true); err == nil {
		t.Error("missing required file should error")
	}
}
