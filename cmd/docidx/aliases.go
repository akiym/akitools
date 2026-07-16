package docidx

import (
	"encoding/json"
	"fmt"
	"os"
	"strings"
	"unicode"
)

type aliasDict map[string][]string

func loadAliases(path string, required bool) (aliasDict, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		if os.IsNotExist(err) && !required {
			return nil, nil
		}
		return nil, err
	}
	var raw map[string][]string
	if err := json.Unmarshal(data, &raw); err != nil {
		return nil, fmt.Errorf("%s: %w", path, err)
	}
	dict := make(aliasDict, len(raw))
	for k, v := range raw {
		dict[strings.ToLower(k)] = v
	}
	return dict, nil
}

// buildFTSQueries expands each query token with its aliases and returns two
// FTS5 match expressions: all tokens ANDed (precise) and ORed (fallback).
func buildFTSQueries(query string, dict aliasDict) (andQuery, orQuery string) {
	var parts []string
	for _, tok := range tokenizeQuery(query) {
		terms := append([]string{tok}, dict[strings.ToLower(tok)]...)
		quoted := make([]string, len(terms))
		for i, t := range terms {
			quoted[i] = `"` + strings.ReplaceAll(t, `"`, `""`) + `"`
		}
		if len(quoted) == 1 {
			parts = append(parts, quoted[0])
		} else {
			parts = append(parts, "("+strings.Join(quoted, " OR ")+")")
		}
	}
	return strings.Join(parts, " AND "), strings.Join(parts, " OR ")
}

func tokenizeQuery(q string) []string {
	fields := strings.FieldsFunc(q, func(r rune) bool {
		return !unicode.IsLetter(r) && !unicode.IsDigit(r) && r != '_' && r != '.'
	})
	// Drop punctuation-only tokens like "." or "...": they quote into FTS5
	// phrases with zero terms, which match nothing and poison the AND query.
	tokens := fields[:0]
	for _, tok := range fields {
		if strings.ContainsFunc(tok, func(r rune) bool { return unicode.IsLetter(r) || unicode.IsDigit(r) }) {
			tokens = append(tokens, tok)
		}
	}
	return tokens
}
