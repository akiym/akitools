package htmlmd

import (
	nurl "net/url"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

const testPage = `<!DOCTYPE html>
<html><head><title>Sample Article - Example Site</title></head>
<body>
<nav><a href="/">home</a> | <a href="/about">about</a></nav>
<article>
<h1>Sample Article</h1>
<p>This is the <strong>first</strong> paragraph with a <a href="/relative/link">relative link</a>. It needs to be long enough for readability to consider it content, so let us add more words about nothing in particular, filling space with prose.</p>
<p>Second paragraph with <em>emphasis</em>, <del>strikethrough</del> and <code>inline code</code>. Readability requires a certain density of commas, sentences, and general text mass before it accepts an element as the main article content of the page.</p>
<table><tr><th>Name</th><th>Value</th></tr><tr><td>foo</td><td>1</td></tr><tr><td>bar</td><td>2</td></tr></table>
<ul><li>one</li><li>two</li></ul>
</article>
<footer>Copyright 2026 boilerplate that should be stripped away by readability.</footer>
</body></html>`

func TestConvert(t *testing.T) {
	md, err := convert(strings.NewReader(testPage), nil)
	if err != nil {
		t.Fatalf("convert() error = %v", err)
	}

	for _, want := range []string{
		"# Sample Article - Example Site",
		"**first**",
		"[relative link](/relative/link)",
		"*emphasis*",
		"~~strikethrough~~",
		"`inline code`",
		"| Name | Value |",
		"- one",
	} {
		if !strings.Contains(md, want) {
			t.Errorf("convert() output does not contain %q\noutput:\n%s", want, md)
		}
	}

	for _, unwant := range []string{
		"home",
		"Copyright 2026",
	} {
		if strings.Contains(md, unwant) {
			t.Errorf("convert() output contains boilerplate %q\noutput:\n%s", unwant, md)
		}
	}
}

func TestConvertResolvesRelativeLinks(t *testing.T) {
	pageURL, err := nurl.Parse("https://example.com/post")
	if err != nil {
		t.Fatal(err)
	}

	md, err := convert(strings.NewReader(testPage), pageURL)
	if err != nil {
		t.Fatalf("convert() error = %v", err)
	}

	want := "[relative link](https://example.com/relative/link)"
	if !strings.Contains(md, want) {
		t.Errorf("convert() output does not contain %q\noutput:\n%s", want, md)
	}
}

func TestConvertNoReadableContent(t *testing.T) {
	if _, err := convert(strings.NewReader("<html><body></body></html>"), nil); err == nil {
		t.Error("convert() expected error for empty document, got nil")
	}
}

func TestConvertFile(t *testing.T) {
	name := filepath.Join(t.TempDir(), "a.html")
	if err := os.WriteFile(name, []byte(testPage), 0o644); err != nil {
		t.Fatal(err)
	}

	md, err := convertFile(name, nil)
	if err != nil {
		t.Fatalf("convertFile() error = %v", err)
	}
	if !strings.Contains(md, "# Sample Article - Example Site") {
		t.Errorf("convertFile() output does not contain title\noutput:\n%s", md)
	}

	if _, err := convertFile(filepath.Join(t.TempDir(), "nonexistent.html"), nil); err == nil {
		t.Error("convertFile() expected error for nonexistent file, got nil")
	}
}
