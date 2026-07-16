package docidx

import (
	"strings"
	"testing"
)

const testMarkdown = `# Instancing scenes

Intro paragraph.

## Creating instances

Use PackedScene to instantiate.

### At runtime

Call instantiate() on the scene.

#### Return value

Returns a Node.

## Cleanup

` + "```" + `
# not a heading inside fence
` + "```" + `

Free the node when done.
`

func TestChunkMarkdown(t *testing.T) {
	chunks := chunkMarkdown("tutorials/instancing.md", []byte(testMarkdown))
	if len(chunks) != 4 {
		t.Fatalf("got %d chunks, want 4: %+v", len(chunks), chunks)
	}

	if chunks[0].Title != "Instancing scenes" {
		t.Errorf("chunks[0].Title = %q", chunks[0].Title)
	}
	if chunks[0].Breadcrumbs != "Instancing scenes" {
		t.Errorf("chunks[0].Breadcrumbs = %q", chunks[0].Breadcrumbs)
	}
	if !strings.Contains(chunks[0].Body, "Intro paragraph.") {
		t.Errorf("chunks[0].Body = %q", chunks[0].Body)
	}
	if chunks[0].Kind != "tutorial" {
		t.Errorf("chunks[0].Kind = %q", chunks[0].Kind)
	}

	if chunks[1].Title != "Creating instances" {
		t.Errorf("chunks[1].Title = %q", chunks[1].Title)
	}
	if chunks[1].Anchor != "creating-instances" {
		t.Errorf("chunks[1].Anchor = %q", chunks[1].Anchor)
	}
	if chunks[1].Breadcrumbs != "Instancing scenes > Creating instances" {
		t.Errorf("chunks[1].Breadcrumbs = %q", chunks[1].Breadcrumbs)
	}

	if chunks[2].Title != "At runtime" {
		t.Errorf("chunks[2].Title = %q", chunks[2].Title)
	}
	if chunks[2].Breadcrumbs != "Instancing scenes > Creating instances > At runtime" {
		t.Errorf("chunks[2].Breadcrumbs = %q", chunks[2].Breadcrumbs)
	}
	if !strings.Contains(chunks[2].Headings, "Return value") {
		t.Errorf("H4 subheading missing from Headings: %q", chunks[2].Headings)
	}
	if !strings.Contains(chunks[2].Body, "Returns a Node.") {
		t.Errorf("H4 body not merged into parent: %q", chunks[2].Body)
	}

	if chunks[3].Title != "Cleanup" {
		t.Errorf("chunks[3].Title = %q", chunks[3].Title)
	}
	if !strings.Contains(chunks[3].Body, "# not a heading inside fence") {
		t.Errorf("fenced content missing: %q", chunks[3].Body)
	}
}

func TestChunkMarkdownNoH1(t *testing.T) {
	src := "Some intro.\n\n## Section\n\nBody text.\n"
	chunks := chunkMarkdown("guides/setup.md", []byte(src))
	if len(chunks) != 2 {
		t.Fatalf("got %d chunks, want 2", len(chunks))
	}
	if chunks[0].Title != "setup" {
		t.Errorf("preamble title = %q, want filename fallback", chunks[0].Title)
	}
	if chunks[1].Breadcrumbs != "setup > Section" {
		t.Errorf("chunks[1].Breadcrumbs = %q", chunks[1].Breadcrumbs)
	}
	if chunks[1].Kind != "guide" {
		t.Errorf("chunks[1].Kind = %q", chunks[1].Kind)
	}
}

func TestChunkMarkdownNestedFences(t *testing.T) {
	src := "# Fences\n\n````\n```\n# not a heading\n```\n````\n\nDone.\n"
	chunks := chunkMarkdown("doc.md", []byte(src))
	if len(chunks) != 1 {
		t.Fatalf("inner fence closed the outer one: %+v", chunks)
	}
	if !strings.Contains(chunks[0].Body, "# not a heading") || !strings.Contains(chunks[0].Body, "Done.") {
		t.Errorf("Body = %q", chunks[0].Body)
	}
}

func TestChunkMarkdownSkipsHeadingOnlySections(t *testing.T) {
	src := "# Title\n\n## Empty\n\n## Full\n\nText.\n"
	chunks := chunkMarkdown("doc.md", []byte(src))
	for _, c := range chunks {
		if c.Title == "Empty" {
			t.Errorf("heading-only section was not skipped: %+v", c)
		}
	}
}

func TestDetectKind(t *testing.T) {
	tests := []struct {
		path, title string
		level       int
		want        string
	}{
		{"tutorials/instancing.md", "Instancing", 1, "tutorial"},
		{"classes/animationplayer.md", "AnimationPlayer", 1, "class"},
		{"classes/animationplayer.md", "play()", 2, "method"},
		{"api/rest.md", "Endpoints", 2, "api"},
		{"faq.md", "General", 2, "faq"},
		{"guides/intro.md", "Intro", 1, "guide"},
		{"misc/notes.md", "Notes", 1, "doc"},
	}
	for _, tt := range tests {
		if got := detectKind(tt.path, tt.title, tt.level); got != tt.want {
			t.Errorf("detectKind(%q, %q, %d) = %q, want %q", tt.path, tt.title, tt.level, got, tt.want)
		}
	}
}
