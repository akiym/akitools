package docidx

import (
	"regexp"
	"strings"
	"unicode/utf8"
)

var (
	rstTargetRe    = regexp.MustCompile("^\\.\\. _(?:`([^`]+)`|([^:`]+)):$")
	rstDirectiveRe = regexp.MustCompile(`^\.\. [A-Za-z0-9_.:+-]+::(\s|$)`)
	rstFootnoteRe  = regexp.MustCompile(`^\.\. \[[^\]]*\]`)
	rstFieldRe     = regexp.MustCompile(`^:[A-Za-z0-9_. -]+:(\s|$)`)
	rstBoldRe      = regexp.MustCompile(`\*\*(.+?)\*\*`)

	// Grid-table lines: borders carry no text and column padding dwarfs the
	// content (a wide API summary table can be >100KB of mostly spaces), so
	// borders are dropped and row padding collapsed.
	rstTableBorderRe = regexp.MustCompile(`^\s*\+[-=+]+\+$`)
	rstTableRowRe    = regexp.MustCompile(`^\s*\|.*\|$`)
)

const rstAdornmentChars = "!\"#$%&'()*+,-./:;<=>?@[\\]^_`{|}~"

func chunkRST(relPath string, src []byte) []Chunk {
	p := &rstParser{styles: map[string]int{}, inDocinfo: true, prevBlank: true, cur: &section{level: 0}}
	p.secs = []*section{p.cur}
	p.parse(strings.Split(string(src), "\n"))
	return sectionsToChunks(relPath, p.docTitle, p.secs)
}

type rstParser struct {
	secs     []*section
	cur      *section
	styles   map[string]int // adornment style -> level, in order of first use
	docTitle string
	// headingLvl is the level of the innermost adornment heading; target
	// items (see content) become its children, and stay siblings of each
	// other because items do not update it.
	headingLvl int
	pending    string // explicit target awaiting the element it labels
	inDocinfo  bool
	prevBlank  bool
}

func (p *rstParser) parse(lines []string) {
	for i := range lines {
		lines[i] = strings.TrimRight(lines[i], " \t\r")
	}
	for i := 0; i < len(lines); i++ {
		line := lines[i]

		if line == "" {
			p.cur.body = append(p.cur.body, "")
			p.prevBlank = true
			continue
		}

		if line == ".." || strings.HasPrefix(line, ".. ") {
			if m := rstTargetRe.FindStringSubmatch(line); m != nil {
				if name := m[1] + m[2]; p.pending == "" && name != "_" {
					p.pending = name
				}
			} else if strings.HasPrefix(line, ".. rst-class::") {
				// Invisible: only attaches a class to the next element.
			} else if rstFootnoteRe.MatchString(line) || rstDirectiveRe.MatchString(line) {
				p.content(line, false)
				continue
			} else {
				// Comments, substitution definitions, and hyperlink
				// targets produce no rendered output; drop them along
				// with their indented continuation lines.
				for i+1 < len(lines) && (lines[i+1] == "" || isIndented(lines[i+1])) {
					i++
				}
			}
			p.prevBlank = true
			continue
		}

		if isIndented(line) {
			p.content(line, false)
			continue
		}

		// Docinfo field list at the top of the file (:github_url: hide).
		if p.inDocinfo && rstFieldRe.MatchString(line) {
			p.prevBlank = true
			continue
		}

		if p.prevBlank {
			if _, ok := rstAdornmentRun(line); ok {
				if p.overlineHeading(lines, i) {
					i += 2
					continue
				}
				if len(line) >= 4 && (i+1 >= len(lines) || lines[i+1] == "") {
					// Transition: renders as a horizontal rule, no text.
					continue
				}
			} else if i+1 < len(lines) {
				under := lines[i+1]
				if _, ok := rstAdornmentRun(under); ok && (len(under) >= utf8.RuneCountInString(line) || len(under) >= 4) {
					p.heading(line, string(under[0]))
					i++
					continue
				}
			}
		}

		p.content(line, true)
	}
}

func (p *rstParser) overlineHeading(lines []string, i int) bool {
	if i+2 >= len(lines) {
		return false
	}
	title := strings.TrimSpace(lines[i+1])
	if title == "" {
		return false
	}
	if _, ok := rstAdornmentRun(title); ok {
		return false
	}
	under := lines[i+2]
	uch, ok := rstAdornmentRun(under)
	if !ok || uch != lines[i][0] || len(under) != len(lines[i]) {
		return false
	}
	p.heading(title, "o"+string(uch))
	return true
}

func (p *rstParser) heading(title, style string) {
	level, ok := p.styles[style]
	if !ok {
		level = len(p.styles) + 1
		p.styles[style] = level
	}
	anchor := p.pending
	p.pending = ""
	p.inDocinfo = false
	p.prevBlank = true
	if p.docTitle == "" && level == 1 {
		p.docTitle = title
	}
	if level < 4 {
		p.headingLvl = level
	}
	p.startSection(level, title, anchor, []string{title})
}

// startSection opens a new section, or, when the level is too deep to become
// its own chunk, folds the title into the current section as a subheading.
func (p *rstParser) startSection(level int, title, anchor string, body []string) {
	if level >= 4 {
		p.cur.subheadings = append(p.cur.subheadings, title)
		p.cur.body = append(p.cur.body, body...)
		return
	}
	if anchor == "" {
		anchor = slugify(title)
	}
	p.cur = &section{level: level, title: title, anchor: anchor, body: body}
	p.secs = append(p.secs, p.cur)
}

// content handles a rendered line. When an explicit target is pending and the
// line opens a signature — the shape Sphinx API references use for methods,
// properties, and constants (**add_point**\ (\ id\: ...)) — the target starts
// a new item section one level below the enclosing heading, so each API item
// becomes its own chunk. Otherwise the target is dropped and the line is
// plain body.
func (p *rstParser) content(line string, splittable bool) {
	p.inDocinfo = false
	p.prevBlank = false
	if p.pending != "" && splittable {
		if title := rstItemTitle(line); title != "" {
			anchor := p.pending
			p.pending = ""
			p.startSection(p.headingLvl+1, title, anchor, []string{title, line})
			return
		}
	}
	p.pending = ""
	if t := strings.TrimLeft(line, " \t"); t != "" && (t[0] == '+' || t[0] == '|') {
		if rstTableBorderRe.MatchString(line) {
			return
		}
		if rstTableRowRe.MatchString(line) {
			line = normalizeSpace(line)
		}
	}
	p.cur.body = append(p.cur.body, line)
}

// rstItemTitle extracts the bold name of a signature line: the first bold run
// followed by a parameter list, a value assignment, or a colon (enum headers).
// A parameter list appends "()" so method items read like method titles
// ("add_point()") and are detected as such. Lines whose bold is mid-prose
// return "" and stay plain body.
func rstItemTitle(line string) string {
	m := rstBoldRe.FindStringSubmatchIndex(line)
	if m == nil {
		return ""
	}
	title := line[m[2]:m[3]]
	rest := line[m[1]:]
	// A bold name ending in stars (Godot's "operator *" / "operator **")
	// makes the non-greedy match close early; shift the leftover stars back
	// into the name.
	if n := runLength(rest, '*'); n > 0 {
		title += rest[:n]
		rest = rest[n:]
	}
	rest = strings.TrimLeft(rest, "\\ ")
	switch {
	case strings.HasPrefix(rest, "("):
		return title + "()"
	case strings.HasPrefix(rest, "=") || strings.HasPrefix(rest, ":"):
		return title
	}
	return ""
}

func rstAdornmentRun(s string) (byte, bool) {
	if len(s) < 2 || strings.IndexByte(rstAdornmentChars, s[0]) < 0 || runLength(s, s[0]) != len(s) {
		return 0, false
	}
	return s[0], true
}

func isIndented(s string) bool {
	return len(s) > 0 && (s[0] == ' ' || s[0] == '\t')
}
