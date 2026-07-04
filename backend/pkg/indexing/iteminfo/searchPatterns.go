package iteminfo

import "strings"

type namePatternMode int

const (
	patternLazyDefault namePatternMode = iota
	patternQuotedLiteral
	patternUserGlob
)

func escapeGlobLiteral(s string) string {
	var b strings.Builder
	for _, r := range s {
		switch r {
		case '*', '?', '[', '\\':
			b.WriteByte('\\')
			b.WriteRune(r)
		default:
			b.WriteRune(r)
		}
	}
	return b.String()
}

func wrapGlobPattern(s string) string {
	return "*" + s + "*"
}

// BuildNameGlobPattern converts a search term into a SQLite GLOB pattern for indexed file names.
func BuildNameGlobPattern(term string, quoted, useWildcard, exactCase bool) string {
	if term == "" {
		return ""
	}

	var mode namePatternMode
	switch {
	case useWildcard:
		mode = patternUserGlob
	case quoted:
		mode = patternQuotedLiteral
	default:
		mode = patternLazyDefault
	}

	var pattern string
	switch mode {
	case patternUserGlob:
		pattern = term
	case patternQuotedLiteral:
		pattern = wrapGlobPattern(escapeGlobLiteral(term))
	case patternLazyDefault:
		parts := strings.Fields(term)
		if len(parts) == 0 {
			return ""
		}
		escaped := make([]string, len(parts))
		for i, p := range parts {
			escaped[i] = escapeGlobLiteral(p)
		}
		pattern = wrapGlobPattern(strings.Join(escaped, "*"))
	}

	if !exactCase {
		pattern = strings.ToLower(pattern)
	}
	return pattern
}

// NameGlobPatternsForSearch builds SQLite name GLOB patterns for the given search options.
func NameGlobPatternsForSearch(opts SearchOptions, useWildcard, largest bool) []string {
	if largest || len(opts.Terms) == 0 {
		return nil
	}

	exactCase := opts.Conditions["exact"]
	var patterns []string
	for _, t := range opts.Terms {
		if t == "" {
			continue
		}
		p := BuildNameGlobPattern(t, opts.Quoted, useWildcard, exactCase)
		if p != "" {
			patterns = append(patterns, p)
		}
	}
	if len(patterns) == 0 {
		return nil
	}
	return patterns
}
