package docidx

import (
	"archive/tar"
	"bytes"
	"compress/gzip"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

type tarEntry struct {
	name     string
	typeflag byte
	linkname string
	body     string
}

func makeTarGz(t *testing.T, entries []tarEntry) *bytes.Reader {
	t.Helper()
	var buf bytes.Buffer
	gz := gzip.NewWriter(&buf)
	tw := tar.NewWriter(gz)
	for _, e := range entries {
		hdr := &tar.Header{
			Name:     e.name,
			Typeflag: e.typeflag,
			Linkname: e.linkname,
			Mode:     0o644,
			Size:     int64(len(e.body)),
		}
		if e.typeflag == tar.TypeDir {
			hdr.Mode = 0o755
			hdr.Size = 0
		}
		if e.typeflag == tar.TypeSymlink {
			hdr.Size = 0
		}
		if err := tw.WriteHeader(hdr); err != nil {
			t.Fatal(err)
		}
		if e.typeflag == tar.TypeReg {
			if _, err := tw.Write([]byte(e.body)); err != nil {
				t.Fatal(err)
			}
		}
	}
	if err := tw.Close(); err != nil {
		t.Fatal(err)
	}
	if err := gz.Close(); err != nil {
		t.Fatal(err)
	}
	return bytes.NewReader(buf.Bytes())
}

func TestExtractTarGz(t *testing.T) {
	dest := t.TempDir()
	r := makeTarGz(t, []tarEntry{
		{name: "Go.docset/", typeflag: tar.TypeDir},
		{name: "Go.docset/index.html", typeflag: tar.TypeReg, body: "<h1>hi</h1>"},
		{name: "Go.docset/alias.html", typeflag: tar.TypeSymlink, linkname: "index.html"},
	})

	root, err := extractTarGz(r, dest)
	if err != nil {
		t.Fatal(err)
	}
	if root != "Go.docset" {
		t.Errorf("root = %q, want Go.docset", root)
	}
	data, err := os.ReadFile(filepath.Join(dest, "Go.docset", "index.html"))
	if err != nil {
		t.Fatal(err)
	}
	if string(data) != "<h1>hi</h1>" {
		t.Errorf("index.html = %q", data)
	}
}

// A docset is third-party content, so a symlink pointing out of the
// extraction root must be rejected: a later entry written through it would
// otherwise land anywhere on the host.
func TestExtractTarGzRejectsEscapingSymlink(t *testing.T) {
	outside := t.TempDir()
	victim := filepath.Join(outside, "victim")
	if err := os.WriteFile(victim, []byte("original"), 0o600); err != nil {
		t.Fatal(err)
	}

	for _, tt := range []struct {
		name     string
		linkname string
	}{
		{"absolute", outside},
		{"relative", "../.."},
	} {
		t.Run(tt.name, func(t *testing.T) {
			dest := t.TempDir()
			r := makeTarGz(t, []tarEntry{
				{name: "Go.docset/", typeflag: tar.TypeDir},
				{name: "Go.docset/escape", typeflag: tar.TypeSymlink, linkname: tt.linkname},
				{name: "Go.docset/escape/victim", typeflag: tar.TypeReg, body: "pwned"},
			})

			_, err := extractTarGz(r, dest)
			if err == nil || !strings.Contains(err.Error(), "illegal symlink") {
				t.Fatalf("err = %v, want illegal symlink", err)
			}
			data, err := os.ReadFile(victim)
			if err != nil {
				t.Fatal(err)
			}
			if string(data) != "original" {
				t.Errorf("victim was overwritten: %q", data)
			}
		})
	}
}

func TestExtractTarGzSkipsEscapingPath(t *testing.T) {
	outside := t.TempDir()
	dest := filepath.Join(outside, "out")
	if err := os.Mkdir(dest, 0o755); err != nil {
		t.Fatal(err)
	}
	r := makeTarGz(t, []tarEntry{
		{name: "Go.docset/ok", typeflag: tar.TypeReg, body: "x"},
		{name: "Go.docset/../../escape", typeflag: tar.TypeReg, body: "pwned"},
	})

	if _, err := extractTarGz(r, dest); err != nil {
		t.Fatal(err)
	}
	if _, err := os.Stat(filepath.Join(outside, "escape")); !os.IsNotExist(err) {
		t.Errorf("entry escaped the destination: %v", err)
	}
}

func TestExtractTarGzDropsOtherWritePermissions(t *testing.T) {
	dest := t.TempDir()
	var buf bytes.Buffer
	gz := gzip.NewWriter(&buf)
	tw := tar.NewWriter(gz)
	hdr := &tar.Header{Name: "Go.docset/loose", Typeflag: tar.TypeReg, Mode: 0o777, Size: 1}
	if err := tw.WriteHeader(hdr); err != nil {
		t.Fatal(err)
	}
	if _, err := tw.Write([]byte("x")); err != nil {
		t.Fatal(err)
	}
	if err := tw.Close(); err != nil {
		t.Fatal(err)
	}
	if err := gz.Close(); err != nil {
		t.Fatal(err)
	}

	if _, err := extractTarGz(bytes.NewReader(buf.Bytes()), dest); err != nil {
		t.Fatal(err)
	}
	fi, err := os.Stat(filepath.Join(dest, "Go.docset", "loose"))
	if err != nil {
		t.Fatal(err)
	}
	if fi.Mode().Perm()&0o022 != 0 {
		t.Errorf("mode = %v, want no group/other write", fi.Mode().Perm())
	}
}
