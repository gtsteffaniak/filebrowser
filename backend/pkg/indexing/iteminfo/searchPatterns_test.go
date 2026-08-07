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
		{"test[1]", false, false, false, "*test\\[1]*"},
	}

	for _, tt := range tests {
		got := BuildNameGlobPattern(tt.term, tt.quoted, tt.useWildcard, tt.exactCase)
		if got != tt.want {
			t.Errorf("BuildNameGlobPattern(%q, quoted=%v, wildcard=%v, exact=%v) = %q, want %q",
				tt.term, tt.quoted, tt.useWildcard, tt.exactCase, got, tt.want)
		}
	}
}

func TestNameGlobPatternsForSearch(t *testing.T) {
	opts := SearchOptions{
		Terms: []string{"house", "car"},
	}
	patterns := NameGlobPatternsForSearch(opts, false, false)
	if len(patterns) != 2 || patterns[0] != "*house*" || patterns[1] != "*car*" {
		t.Fatalf("unexpected patterns: %#v", patterns)
	}

	optsQuoted := SearchOptions{
		Terms:  []string{"new folder"},
		Quoted: true,
	}
	patterns = NameGlobPatternsForSearch(optsQuoted, false, false)
	if len(patterns) != 1 || patterns[0] != "*new folder*" {
		t.Fatalf("unexpected quoted patterns: %#v", patterns)
	}

	optsLargest := SearchOptions{Terms: []string{"test"}}
	if p := NameGlobPatternsForSearch(optsLargest, false, true); p != nil {
		t.Fatalf("largest mode should return nil patterns, got %#v", p)
	}
}
