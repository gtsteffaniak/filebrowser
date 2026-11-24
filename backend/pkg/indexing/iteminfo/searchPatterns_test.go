package iteminfo

import "testing"

func TestBuildNameGlobPattern(t *testing.T) {
	tests := []struct {
		term        string
		quoted      bool
		useWildcard bool
		exactCase   bool
		want        string
	}{
		{"cons", false, false, false, "*cons*"},
		{"new folder", false, false, false, "*new*folder*"},
		{"new folder", true, false, false, "*new folder*"},
		{"new*folder", false, true, false, "new*folder"},
		{"Blade Runner", false, false, false, "*blade*runner*"},
		{"Blade Runner", false, false, true, "*Blade*Runner*"},
		{"test[1]", false, false, false, "*test[[]1]*"},
		{"a?b*c", false, false, false, "*a[?]b[*]c*"},
	}

	for _, tt := range tests {
		got := BuildNameGlobPattern(tt.term, tt.quoted, tt.useWildcard, tt.exactCase)
		if got != tt.want {
			t.Errorf("BuildNameGlobPattern(%q, quoted=%v, wildcard=%v, exact=%v) = %q, want %q",
				tt.term, tt.quoted, tt.useWildcard, tt.exactCase, got, tt.want)
		}
	}
}

func TestBuildNameLikePrefilter(t *testing.T) {
	tests := []struct {
		term   string
		quoted bool
		want   string
	}{
		{"house", false, "%house%"},
		{"new folder", false, "%new%folder%"},
		{"new folder", true, "%new folder%"},
		{"", false, ""},
	}
	for _, tt := range tests {
		if got := BuildNameLikePrefilter(tt.term, tt.quoted); got != tt.want {
			t.Errorf("BuildNameLikePrefilter(%q, quoted=%v) = %q, want %q", tt.term, tt.quoted, got, tt.want)
		}
	}
}

func TestNameFilterForSearch(t *testing.T) {
	// Default (case-insensitive substring): LIKE prefilter plus lower(name) exact.
	f := NameFilterForSearch(SearchOptions{Terms: []string{"house", "car"}}, false, false)
	if f == nil {
		t.Fatal("expected a filter")
	}
	if !f.UseFTS || !f.PrefilterLike || !f.ExactLower || f.JoinAnd {
		t.Fatalf("unexpected flags: %+v", f)
	}
	if len(f.Exact) != 2 || f.Exact[0] != "*house*" || f.Exact[1] != "*car*" {
		t.Fatalf("unexpected exact: %#v", f.Exact)
	}
	if len(f.FTS) != 2 || f.FTS[0] != "%house%" || f.FTS[1] != "%car%" {
		t.Fatalf("unexpected fts prefilter: %#v", f.FTS)
	}

	// Quoted phrase keeps the space literal.
	f = NameFilterForSearch(SearchOptions{Terms: []string{"new folder"}, Quoted: true}, false, false)
	if len(f.Exact) != 1 || f.Exact[0] != "*new folder*" || f.FTS[0] != "%new folder%" {
		t.Fatalf("unexpected quoted filter: %#v / %#v", f.Exact, f.FTS)
	}

	// case:exact uses a case-sensitive GLOB prefilter and no lower().
	f = NameFilterForSearch(SearchOptions{
		Terms:      []string{"House"},
		Conditions: map[string]bool{"exact": true},
	}, false, false)
	if f.PrefilterLike || f.ExactLower {
		t.Fatalf("case:exact should not use LIKE/lower: %+v", f)
	}
	if len(f.FTS) != 1 || f.FTS[0] != "*House*" {
		t.Fatalf("unexpected case:exact fts: %#v", f.FTS)
	}

	// Wildcard stays on the legacy path (no FTS prefilter).
	f = NameFilterForSearch(SearchOptions{Terms: []string{"new*folder"}}, true, false)
	if f.UseFTS || len(f.FTS) != 0 {
		t.Fatalf("wildcard should not use FTS: %+v", f)
	}
	if len(f.Exact) != 1 || f.Exact[0] != "new*folder" {
		t.Fatalf("unexpected wildcard exact: %#v", f.Exact)
	}

	// Too short for trigrams => no FTS.
	f = NameFilterForSearch(SearchOptions{Terms: []string{"ab"}}, false, false)
	if f.UseFTS {
		t.Fatalf("short term should not use FTS: %+v", f)
	}

	// largest mode => no filter.
	if f := NameFilterForSearch(SearchOptions{Terms: []string{"test"}}, false, true); f != nil {
		t.Fatalf("largest mode should return nil, got %#v", f)
	}
}
