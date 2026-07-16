package docidx

import (
	"bytes"
	"strings"

	"golang.org/x/net/html"
	"golang.org/x/net/html/atom"
)

var htmlSkipTags = map[atom.Atom]bool{
	atom.Script:   true,
	atom.Style:    true,
	atom.Nav:      true,
	atom.Aside:    true,
	atom.Footer:   true,
	atom.Noscript: true,
	atom.Template: true,
}

var htmlBlockTags = map[atom.Atom]bool{
	atom.P: true, atom.Div: true, atom.Section: true, atom.Article: true,
	atom.Main: true, atom.Li: true, atom.Ul: true, atom.Ol: true,
	atom.Pre: true, atom.Blockquote: true, atom.Table: true, atom.Tr: true,
	atom.Td: true, atom.Th: true, atom.Caption: true,
	atom.Dt: true, atom.Dd: true, atom.Br: true, atom.Hr: true,
	atom.Figure: true, atom.Figcaption: true,
}

// htmlSkipDivIDs are container divs holding doxygen page chrome rather than
// content: the header (title area, menus) lives in <div id="top">, and the
// search UI widgets land inside or outside it depending on the page type.
// They are only skipped when the page declares a Doxygen generator, since
// id="top" is also a common back-to-top wrapper on hand-written pages.
var htmlSkipDivIDs = map[string]bool{
	"top":                  true,
	"MSearchSelectWindow":  true,
	"MSearchResultsWindow": true,
}

func headingLevel(a atom.Atom) int {
	switch a {
	case atom.H1:
		return 1
	case atom.H2:
		return 2
	case atom.H3:
		return 3
	case atom.H4:
		return 4
	case atom.H5:
		return 5
	case atom.H6:
		return 6
	}
	return 0
}

func chunkHTML(relPath string, src []byte) ([]Chunk, error) {
	doc, err := html.Parse(bytes.NewReader(src))
	if err != nil {
		return nil, err
	}

	p := &htmlParser{cur: &section{level: 0}}
	p.secs = []*section{p.cur}
	p.walk(doc)
	p.flushLine()

	docTitle := p.firstH1
	if docTitle == "" {
		docTitle = strings.TrimSpace(p.titleTag)
	}
	return sectionsToChunks(relPath, docTitle, p.secs), nil
}

type htmlParser struct {
	secs         []*section
	cur          *section
	line         strings.Builder
	pendingSpace bool
	titleTag     string
	firstH1      string
	preDepth     int
	doxygen      bool
}

func (p *htmlParser) walk(n *html.Node) {
	if n.Type == html.TextNode {
		p.text(n.Data)
		return
	}
	if n.Type != html.ElementNode && n.Type != html.DocumentNode {
		return
	}
	if n.Type == html.ElementNode {
		// Headings dispatch before the skip checks: generators like
		// Docusaurus put class="anchor" on the heading element itself.
		if headingLevel(n.DataAtom) > 0 {
			p.heading(headingLevel(n.DataAtom), n)
			return
		}
		if htmlSkipTags[n.DataAtom] || isSelfLink(n) || p.isSkipDiv(n) {
			return
		}
		switch {
		case n.DataAtom == atom.Meta:
			p.detectGenerator(n)
			return
		case n.DataAtom == atom.Title && n.Namespace == "":
			// Only the document <title>; inline <svg><title> tooltips
			// also parse as atom.Title but carry the svg namespace.
			if p.titleTag == "" {
				p.titleTag = nodeText(n)
			}
			return
		case n.DataAtom == atom.Pre:
			p.preDepth++
			defer func() { p.preDepth-- }()
		}
	}
	for c := n.FirstChild; c != nil; c = c.NextSibling {
		p.walk(c)
	}
	if n.Type == html.ElementNode && htmlBlockTags[n.DataAtom] {
		p.flushLine()
	}
}

func (p *htmlParser) heading(level int, n *html.Node) {
	p.flushLine()
	title := nodeText(n)
	if title == "" {
		return
	}
	if p.firstH1 == "" && level == 1 {
		p.firstH1 = title
	}
	if level >= 4 {
		p.cur.subheadings = append(p.cur.subheadings, title)
		p.cur.body = append(p.cur.body, title)
		return
	}
	anchor := headingAnchor(n)
	if anchor == "" {
		anchor = slugify(title)
	}
	p.cur = &section{level: level, title: title, anchor: anchor}
	p.cur.body = append(p.cur.body, title)
	p.secs = append(p.secs, p.cur)
}

func (p *htmlParser) text(s string) {
	if p.preDepth > 0 {
		lines := strings.Split(s, "\n")
		for i, l := range lines {
			if i > 0 {
				p.flushLine()
			}
			p.line.WriteString(l)
		}
		return
	}
	leadingSpace := len(s) > 0 && isHTMLSpace(s[0])
	trailingSpace := len(s) > 0 && isHTMLSpace(s[len(s)-1])
	s = normalizeSpace(s)
	if s == "" {
		if p.line.Len() > 0 {
			p.pendingSpace = true
		}
		return
	}
	if p.line.Len() > 0 && (leadingSpace || p.pendingSpace) {
		p.line.WriteByte(' ')
	}
	p.line.WriteString(s)
	p.pendingSpace = trailingSpace
}

func isHTMLSpace(c byte) bool {
	return c == ' ' || c == '\t' || c == '\n' || c == '\r' || c == '\f'
}

func (p *htmlParser) flushLine() {
	line := strings.TrimRight(p.line.String(), " \t")
	p.line.Reset()
	p.pendingSpace = false
	if line == "" {
		return
	}
	p.cur.body = append(p.cur.body, line)
}

// headingAnchor returns the id attribute of the heading element itself or of
// the first descendant that has one (e.g. <h2><a id="play"></a>play()</h2>).
// Failing that, it falls back to the fragment of a self-link inside the
// heading (doxygen puts the real anchor just before the <h2> and only links
// to it from a permalink: <h2><span class="permalink"><a href="#ahash">).
// Ordinary cross-reference links in the heading text are not anchors.
func headingAnchor(n *html.Node) string {
	if anchor := findAttrAnchor(n); anchor != "" {
		return anchor
	}
	return findSelfLinkHref(n)
}

func findSelfLinkHref(n *html.Node) string {
	if isSelfLink(n) {
		return findHrefAnchor(n)
	}
	for c := n.FirstChild; c != nil; c = c.NextSibling {
		if anchor := findSelfLinkHref(c); anchor != "" {
			return anchor
		}
	}
	return ""
}

func findAttrAnchor(n *html.Node) string {
	if n.Type == html.ElementNode {
		for _, a := range n.Attr {
			if a.Key == "id" && a.Val != "" {
				return a.Val
			}
			if a.Key == "name" && a.Val != "" && n.DataAtom == atom.A {
				return a.Val
			}
		}
	}
	for c := n.FirstChild; c != nil; c = c.NextSibling {
		if anchor := findAttrAnchor(c); anchor != "" {
			return anchor
		}
	}
	return ""
}

func findHrefAnchor(n *html.Node) string {
	if n.Type == html.ElementNode && n.DataAtom == atom.A {
		for _, a := range n.Attr {
			if a.Key == "href" && len(a.Val) > 1 && strings.HasPrefix(a.Val, "#") {
				return a.Val[1:]
			}
		}
	}
	for c := n.FirstChild; c != nil; c = c.NextSibling {
		if anchor := findHrefAnchor(c); anchor != "" {
			return anchor
		}
	}
	return ""
}

// selfLinkClasses mark heading self-link widgets injected by documentation
// generators (doxygen "◆", Sphinx/MkDocs "¶", rustdoc "§") that must not
// leak into extracted text.
var selfLinkClasses = map[string]bool{
	"permalink":  true,
	"headerlink": true,
	"anchor":     true,
}

func (p *htmlParser) isSkipDiv(n *html.Node) bool {
	if !p.doxygen || n.DataAtom != atom.Div {
		return false
	}
	for _, a := range n.Attr {
		if a.Key == "id" {
			return htmlSkipDivIDs[a.Val]
		}
	}
	return false
}

func (p *htmlParser) detectGenerator(n *html.Node) {
	var name, content string
	for _, a := range n.Attr {
		switch a.Key {
		case "name":
			name = a.Val
		case "content":
			content = a.Val
		}
	}
	if strings.EqualFold(name, "generator") && strings.HasPrefix(content, "Doxygen") {
		p.doxygen = true
	}
}

func isSelfLink(n *html.Node) bool {
	if n.Type != html.ElementNode {
		return false
	}
	for _, a := range n.Attr {
		if a.Key == "class" {
			for _, c := range strings.Fields(a.Val) {
				if selfLinkClasses[strings.ToLower(c)] {
					return true
				}
			}
		}
	}
	return false
}

// nodeText extracts the visible text of a subtree, skipping self-link
// widgets. The root itself is exempt from the self-link check so that
// headings carrying such a class (Docusaurus h2.anchor) keep their text.
func nodeText(n *html.Node) string {
	var b strings.Builder
	var walk func(*html.Node)
	walk = func(c *html.Node) {
		if c != n && isSelfLink(c) {
			return
		}
		if c.Type == html.TextNode {
			b.WriteString(c.Data)
		}
		for cc := c.FirstChild; cc != nil; cc = cc.NextSibling {
			walk(cc)
		}
	}
	walk(n)
	return normalizeSpace(b.String())
}

func normalizeSpace(s string) string {
	return strings.Join(strings.Fields(s), " ")
}
