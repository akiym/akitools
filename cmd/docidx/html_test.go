package docidx

import (
	"strings"
	"testing"
)

const testHTML = `<html>
<head><title>AnimationPlayer — API</title></head>
<body>
<nav><a href="/">Skip this nav</a></nav>
<h1>AnimationPlayer</h1>
<p>Player node for <em>animations</em>.</p>
<h2 id="method-play">play()</h2>
<p>Starts playback.</p>
<pre>player.play("walk")
player.stop()</pre>
<h2>stop()</h2>
<p>Stops playback.</p>
<footer>Copyright notice</footer>
</body>
</html>`

func TestChunkHTML(t *testing.T) {
	chunks, err := chunkHTML("classes/animationplayer.html", []byte(testHTML))
	if err != nil {
		t.Fatal(err)
	}
	if len(chunks) != 3 {
		t.Fatalf("got %d chunks, want 3: %+v", len(chunks), chunks)
	}

	if chunks[0].Title != "AnimationPlayer" {
		t.Errorf("chunks[0].Title = %q", chunks[0].Title)
	}
	if !strings.Contains(chunks[0].Body, "Player node for animations.") {
		t.Errorf("inline text not joined: %q", chunks[0].Body)
	}
	if chunks[0].Kind != "class" {
		t.Errorf("chunks[0].Kind = %q", chunks[0].Kind)
	}

	if chunks[1].Title != "play()" {
		t.Errorf("chunks[1].Title = %q", chunks[1].Title)
	}
	if chunks[1].Anchor != "method-play" {
		t.Errorf("id attribute not used as anchor: %q", chunks[1].Anchor)
	}
	if chunks[1].Kind != "method" {
		t.Errorf("chunks[1].Kind = %q", chunks[1].Kind)
	}
	if !strings.Contains(chunks[1].Body, "player.play(\"walk\")\nplayer.stop()") {
		t.Errorf("pre content lost line breaks: %q", chunks[1].Body)
	}
	if chunks[1].Breadcrumbs != "AnimationPlayer > play()" {
		t.Errorf("chunks[1].Breadcrumbs = %q", chunks[1].Breadcrumbs)
	}

	if chunks[2].Anchor != "stop" {
		t.Errorf("slug fallback anchor = %q", chunks[2].Anchor)
	}

	for _, c := range chunks {
		if strings.Contains(c.Body, "Skip this nav") || strings.Contains(c.Body, "Copyright notice") {
			t.Errorf("nav/footer content leaked into chunk %q", c.Title)
		}
	}
}

func TestChunkHTMLTitleTagFallback(t *testing.T) {
	src := `<html><head><title>Setup Guide</title></head><body>
<h2>Install</h2><p>Run the installer.</p>
</body></html>`
	chunks, err := chunkHTML("guide.html", []byte(src))
	if err != nil {
		t.Fatal(err)
	}
	if len(chunks) != 1 {
		t.Fatalf("got %d chunks, want 1", len(chunks))
	}
	if chunks[0].Breadcrumbs != "Setup Guide > Install" {
		t.Errorf("Breadcrumbs = %q", chunks[0].Breadcrumbs)
	}
}

func TestChunkHTMLDoxygenSelfLink(t *testing.T) {
	src := `<html><head><title>Godot: AnimationPlayer Class Reference</title></head><body>
<div class="contents">
<a id="a3e88025" name="a3e88025"></a>
<h2 class="memtitle"><span class="permalink"><a href="#a3e88025">&#9670;&#160;</a></span>advance()</h2>
<div class="memitem"><p>Advances the animation.</p></div>
</div>
</body></html>`
	chunks, err := chunkHTML("class_animation_player.html", []byte(src))
	if err != nil {
		t.Fatal(err)
	}
	if len(chunks) != 1 {
		t.Fatalf("got %d chunks, want 1: %+v", len(chunks), chunks)
	}
	if chunks[0].Title != "advance()" {
		t.Errorf("permalink marker not stripped from title: %q", chunks[0].Title)
	}
	if chunks[0].Anchor != "a3e88025" {
		t.Errorf("anchor not taken from self-link href: %q", chunks[0].Anchor)
	}
	if chunks[0].Kind != "method" {
		t.Errorf("Kind = %q, want method", chunks[0].Kind)
	}
}

func TestChunkHTMLTableCellsSeparated(t *testing.T) {
	src := `<html><body>
<h1>AESContext</h1>
<table class="memberdecls">
<tr><td class="memItemLeft">Error</td><td class="memItemRight"><a href="#a1">set_encode_key</a> (const uint8_t *p_key)</td></tr>
<tr><td class="memItemLeft">void</td><td class="memItemRight"><a href="#a2">finish</a> ()</td></tr>
</table>
</body></html>`
	chunks, err := chunkHTML("class_aes.html", []byte(src))
	if err != nil {
		t.Fatal(err)
	}
	if len(chunks) != 1 {
		t.Fatalf("got %d chunks, want 1", len(chunks))
	}
	if strings.Contains(chunks[0].Body, "Errorset_encode_key") {
		t.Errorf("table cells glued together: %q", chunks[0].Body)
	}
	if !strings.Contains(chunks[0].Body, "set_encode_key") {
		t.Errorf("identifier missing from body: %q", chunks[0].Body)
	}
}

func TestChunkHTMLSkipsChromeDivOnDoxygen(t *testing.T) {
	src := `<html><head><meta name="generator" content="Doxygen 1.17.0"/></head><body>
<div id="top">
<div id="titlearea">Godot Game Engine MIT</div>
<div class="SRStatus" id="Loading">Loading...</div>
</div>
<div class="contents">
<h1>AESContext</h1>
<p>Real content.</p>
</div>
</body></html>`
	chunks, err := chunkHTML("class_aes.html", []byte(src))
	if err != nil {
		t.Fatal(err)
	}
	if len(chunks) != 1 {
		t.Fatalf("got %d chunks, want 1: %+v", len(chunks), chunks)
	}
	if strings.Contains(chunks[0].Body, "Loading...") || strings.Contains(chunks[0].Body, "Game Engine") {
		t.Errorf("page chrome leaked into body: %q", chunks[0].Body)
	}
	if !strings.Contains(chunks[0].Body, "Real content.") {
		t.Errorf("content missing: %q", chunks[0].Body)
	}
}

func TestChunkHTMLKeepsTopDivWithoutDoxygen(t *testing.T) {
	src := `<html><head><title>Guide</title></head><body>
<div id="top">
<h1>Guide</h1>
<p>All the content lives inside the top wrapper.</p>
</div>
</body></html>`
	chunks, err := chunkHTML("guide.html", []byte(src))
	if err != nil {
		t.Fatal(err)
	}
	if len(chunks) != 1 {
		t.Fatalf("non-doxygen div#top content was dropped: %+v", chunks)
	}
	if !strings.Contains(chunks[0].Body, "top wrapper") {
		t.Errorf("content missing: %q", chunks[0].Body)
	}
}

func TestChunkHTMLAnchorClassHeading(t *testing.T) {
	src := `<html><head><title>V1</title></head><body>
<h2 class="anchor" id="real-section">Real Section</h2>
<p>Section body text.</p>
</body></html>`
	chunks, err := chunkHTML("doc.html", []byte(src))
	if err != nil {
		t.Fatal(err)
	}
	if len(chunks) != 1 {
		t.Fatalf("got %d chunks, want 1: %+v", len(chunks), chunks)
	}
	if chunks[0].Title != "Real Section" {
		t.Errorf("heading with anchor class was dropped: %+v", chunks[0])
	}
	if chunks[0].Anchor != "real-section" {
		t.Errorf("Anchor = %q", chunks[0].Anchor)
	}
}

func TestChunkHTMLIgnoresSVGTitle(t *testing.T) {
	src := `<html><head><title>Networking Guide</title></head><body>
<h2>Sockets</h2>
<p>Socket text.</p>
<svg viewBox="0 0 24 24"><title>external link</title></svg>
</body></html>`
	chunks, err := chunkHTML("net.html", []byte(src))
	if err != nil {
		t.Fatal(err)
	}
	if len(chunks) == 0 {
		t.Fatal("no chunks")
	}
	if !strings.HasPrefix(chunks[0].Breadcrumbs, "Networking Guide") {
		t.Errorf("svg <title> overrode document title: breadcrumbs = %q", chunks[0].Breadcrumbs)
	}
}

func TestChunkHTMLXrefLinkIsNotAnchor(t *testing.T) {
	src := `<html><head><title>V6</title></head><body>
<h2>Migration from <a href="#legacy-api">the legacy API</a></h2>
<p>Migration body.</p>
</body></html>`
	chunks, err := chunkHTML("doc.html", []byte(src))
	if err != nil {
		t.Fatal(err)
	}
	if len(chunks) != 1 {
		t.Fatalf("got %d chunks, want 1", len(chunks))
	}
	if chunks[0].Anchor == "legacy-api" {
		t.Errorf("cross-reference fragment used as anchor: %+v", chunks[0])
	}
	if chunks[0].Anchor != "migration-from-the-legacy-api" {
		t.Errorf("Anchor = %q, want slug fallback", chunks[0].Anchor)
	}
}

func TestChunkHTMLSphinxHeaderlink(t *testing.T) {
	src := `<html><body>
<h1>Instancing</h1>
<p>Intro.</p>
<h2>Creating instances<a class="headerlink" href="#creating-instances" title="Permalink">&para;</a></h2>
<p>Use PackedScene.</p>
</body></html>`
	chunks, err := chunkHTML("tutorials/instancing.html", []byte(src))
	if err != nil {
		t.Fatal(err)
	}
	if len(chunks) != 2 {
		t.Fatalf("got %d chunks, want 2: %+v", len(chunks), chunks)
	}
	if chunks[1].Title != "Creating instances" {
		t.Errorf("headerlink pilcrow not stripped: %q", chunks[1].Title)
	}
	if chunks[1].Anchor != "creating-instances" {
		t.Errorf("Anchor = %q", chunks[1].Anchor)
	}
	if strings.Contains(chunks[1].Body, "¶") {
		t.Errorf("pilcrow leaked into body: %q", chunks[1].Body)
	}
}
