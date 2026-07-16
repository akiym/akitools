package docidx

import (
	"path"
	"regexp"
	"strings"
	"unicode"
)

type Chunk struct {
	Path        string
	Title       string
	Headings    string
	Breadcrumbs string
	Body        string
	Anchor      string
	Kind        string
}

// section is an intermediate representation shared by the Markdown and HTML
// parsers: a run of content under a single H1-H3 heading. H4+ headings stay
// inside their parent section and are recorded as subheadings.
type section struct {
	level       int // 0 = preamble before the first heading
	title       string
	anchor      string
	body        []string
	subheadings []string
}

func sectionsToChunks(relPath, docTitle string, secs []*section) []Chunk {
	if docTitle == "" {
		docTitle = strings.TrimSuffix(path.Base(relPath), path.Ext(relPath))
	}
	var chunks []Chunk
	var titles [7]string // heading text by level, 1..6
	for _, sec := range secs {
		if sec.level > 0 {
			titles[sec.level] = sec.title
			for l := sec.level + 1; l <= 6; l++ {
				titles[l] = ""
			}
		}

		body := strings.TrimSpace(strings.Join(sec.body, "\n"))
		if bodyWithoutHeading(sec, body) == "" && len(sec.subheadings) == 0 {
			continue
		}

		title := sec.title
		if title == "" {
			title = docTitle
		}

		var crumbs []string
		if docTitle != "" {
			crumbs = append(crumbs, docTitle)
		}
		for l := 1; l <= sec.level; l++ {
			if titles[l] != "" && (l != 1 || titles[l] != docTitle) {
				crumbs = append(crumbs, titles[l])
			}
		}

		headings := append([]string{title}, sec.subheadings...)

		chunks = append(chunks, Chunk{
			Path:        relPath,
			Title:       title,
			Headings:    strings.Join(headings, "\n"),
			Breadcrumbs: strings.Join(crumbs, " > "),
			Body:        body,
			Anchor:      sec.anchor,
			Kind:        detectKind(relPath, title, sec.level),
		})
	}
	return chunks
}

// bodyWithoutHeading reports the section body minus its own heading line, so
// heading-only sections (e.g. an H1 immediately followed by an H2) can be
// skipped: their title is already part of every child's breadcrumbs.
func bodyWithoutHeading(sec *section, body string) string {
	if sec.level == 0 {
		return body
	}
	lines := strings.Split(body, "\n")
	if len(lines) > 0 && strings.Contains(lines[0], sec.title) {
		lines = lines[1:]
	}
	return strings.TrimSpace(strings.Join(lines, "\n"))
}

var methodTitleRe = regexp.MustCompile(`^[A-Za-z_][A-Za-z0-9_.]*\s*\(`)

func detectKind(relPath, title string, level int) string {
	category := ""
	for _, seg := range strings.Split(strings.ToLower(path.Clean(relPath)), "/") {
		seg = strings.TrimSuffix(seg, path.Ext(seg))
		switch {
		case strings.Contains(seg, "faq"):
			return "faq"
		case seg == "api" || strings.Contains(seg, "reference") || strings.HasPrefix(seg, "class"):
			category = "api"
		case strings.HasPrefix(seg, "tutorial"):
			category = "tutorial"
		case strings.HasPrefix(seg, "guide") || seg == "how-to" || seg == "howto":
			category = "guide"
		}
	}
	isMethod := methodTitleRe.MatchString(title)
	switch category {
	case "api":
		if isMethod {
			return "method"
		}
		if level <= 1 {
			return "class"
		}
		return "api"
	case "":
		if isMethod {
			return "method"
		}
		return "doc"
	default:
		return category
	}
}

func slugify(s string) string {
	var b strings.Builder
	for _, r := range strings.ToLower(strings.TrimSpace(s)) {
		switch {
		case unicode.IsLetter(r) || unicode.IsDigit(r) || r == '_' || r == '-':
			b.WriteRune(r)
		case r == ' ':
			b.WriteByte('-')
		}
	}
	return b.String()
}
