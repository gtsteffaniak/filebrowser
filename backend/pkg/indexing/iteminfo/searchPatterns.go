package iteminfo

import "strings"

type namePatternMode int

const (
	patternLazyDefault namePatternMode = iota
	patternQuotedLiteral
	patternUserGlob
)

// EscapeGlobLiteral escapes SQLite GLOB metacharacters so text is matched literally.
// SQLite GLOB has no backslash escape; literals are written with bracket classes
// instead (e.g. * -> [*], ? -> [?], [ -> [[]).
func EscapeGlobLiteral(s string) string {
	var b strings.Builder
	for _, r := range s {
		switch r {
		case '*', '?':
			b.WriteByte('[')
			b.WriteRune(r)
			b.WriteByte(']')
		case '[':
			b.WriteString("[[]")
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
		pattern = wrapGlobPattern(EscapeGlobLiteral(term))
	case patternLazyDefault:
		parts := strings.Fields(term)
		if len(parts) == 0 {
			return ""
		}
		escaped := make([]string, len(parts))
		for i, p := range parts {
			escaped[i] = EscapeGlobLiteral(p)
		}
		pattern = wrapGlobPattern(strings.Join(escaped, "*"))
	}

	if !exactCase {
		pattern = strings.ToLower(pattern)
	}
	return pattern
}

// BuildNameLikePrefilter builds an unescaped LIKE pattern used only to narrow rows via
// the FTS trigram index. It is a superset of the exact match: characters like % and _
// in the term only widen the candidate set, and the exact GLOB predicate removes the
// false positives. Never use it without an accompanying exact predicate.
func BuildNameLikePrefilter(term string, quoted bool) string {
	if term == "" {
		return ""
	}
	if quoted {
		return "%" + term + "%"
	}
	parts := strings.Fields(term)
	if len(parts) == 0 {
		return ""
	}
	return "%" + strings.Join(parts, "%") + "%"
}

// NameFilter describes how a search narrows results by file name. It carries both the
// FTS trigram prefilter (a superset) and the exact predicate applied to the base table.
type NameFilter struct {
	// UseFTS is true when the FTS prefilter is usable (patterns present and a
	// trigram-usable literal run exists). The query layer also requires FTSReady().
	UseFTS bool
	// PrefilterLike is true when FTS holds LIKE patterns (default mode) and false when
	// they are GLOB patterns (case:exact mode).
	PrefilterLike bool
	// FTS is the prefilter applied to index_items_fts.name.
	FTS []string
	// Exact is the exact predicate applied to the base table's name column.
	Exact []string
	// ExactLower is true when the exact predicate uses lower(name) (case-insensitive).
	ExactLower bool
	// JoinAnd combines patterns with AND instead of OR.
	JoinAnd bool
}

// hasTrigramLiteral reports whether any pattern contains a run of at least 3
// non-wildcard characters, which is required for the FTS trigram index to help.
func hasTrigramLiteral(patterns []string) bool {
	for _, p := range patterns {
		run := 0
		for _, r := range p {
			switch r {
			case '%', '_', '*', '?', '[', ']':
				run = 0
			default:
				run++
				if run >= 3 {
					return true
				}
			}
		}
	}
	return false
}

// NameFilterForSearch builds the name filter for a search, or nil when no name
// filtering applies (largest mode or no terms).
func NameFilterForSearch(opts SearchOptions, useWildcard, largest bool) *NameFilter {
	if largest || len(opts.Terms) == 0 {
		return nil
	}
	exactCase := opts.Conditions["exact"]
	filter := &NameFilter{
		JoinAnd:    opts.MatchAllTerms,
		ExactLower: !exactCase,
	}
	for _, t := range opts.Terms {
		if t == "" {
			continue
		}
		if p := BuildNameGlobPattern(t, opts.Quoted, useWildcard, exactCase); p != "" {
			filter.Exact = append(filter.Exact, p)
		}
		// Wildcard searches stay on the legacy path (case-insensitive lower(name) GLOB).
		if useWildcard {
			continue
		}
		if exactCase {
			if p := BuildNameGlobPattern(t, opts.Quoted, false, true); p != "" {
				filter.FTS = append(filter.FTS, p)
			}
		} else if p := BuildNameLikePrefilter(t, opts.Quoted); p != "" {
			filter.FTS = append(filter.FTS, p)
		}
	}
	if len(filter.Exact) == 0 {
		return nil
	}
	filter.PrefilterLike = !exactCase
	filter.UseFTS = len(filter.FTS) > 0 && hasTrigramLiteral(filter.FTS)
	return filter
}
