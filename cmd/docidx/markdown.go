package docidx

import (
	"regexp"
	"strings"
)

var atxHeadingRe = regexp.MustCompile(`^(#{1,6})\s+(.+?)\s*#*\s*$`)

func chunkMarkdown(relPath string, src []byte) []Chunk {
	docTitle, secs := parseMarkdownSections(string(src))
	return sectionsToChunks(relPath, docTitle, secs)
}

func parseMarkdownSections(src string) (string, []*section) {
	docTitle := ""
	cur := &section{level: 0}
	secs := []*section{cur}

	inFence := false
	var fenceChar byte
	fenceLen := 0
	for _, line := range strings.Split(src, "\n") {
		trimmed := strings.TrimSpace(line)

		if inFence {
			cur.body = append(cur.body, line)
			// A closing fence must be at least as long as the opener and
			// carry no info string.
			if run := runLength(trimmed, fenceChar); run >= fenceLen && strings.TrimSpace(trimmed[run:]) == "" {
				inFence = false
			}
			continue
		}
		if trimmed != "" && (trimmed[0] == '`' || trimmed[0] == '~') {
			if run := runLength(trimmed, trimmed[0]); run >= 3 {
				inFence = true
				fenceChar = trimmed[0]
				fenceLen = run
				cur.body = append(cur.body, line)
				continue
			}
		}

		if len(line) == 0 || line[0] != '#' {
			cur.body = append(cur.body, line)
			continue
		}
		m := atxHeadingRe.FindStringSubmatch(line)
		if m == nil {
			cur.body = append(cur.body, line)
			continue
		}

		level := len(m[1])
		title := m[2]
		if docTitle == "" && level == 1 {
			docTitle = title
		}
		if level >= 4 {
			cur.subheadings = append(cur.subheadings, title)
			cur.body = append(cur.body, line)
			continue
		}
		cur = &section{level: level, title: title, anchor: slugify(title)}
		cur.body = append(cur.body, line)
		secs = append(secs, cur)
	}
	return docTitle, secs
}

func runLength(s string, ch byte) int {
	i := 0
	for i < len(s) && s[i] == ch {
		i++
	}
	return i
}
