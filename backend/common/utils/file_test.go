package utils

import (
	"strings"
	"testing"
)

func TestSanitizeUserPath_indexPaths(t *testing.T) {
	tests := []struct {
		name    string
		input   string
		want    string
		wantErr bool
	}{
		{"root slash", "/", "/", false},
		{"nested", "/foo/bar", "/foo/bar", false},
		{"traversal segment", "..", "", true},
		{"resolved traversal", "/../secret", "/secret", false},
		{"empty", "", "", true},
		{"backslash traversal", `..\secret`, "", true},
		{"backslash double traversal", `..\..\secret`, "", true},
		{"backslash path normalized", `\foo\bar`, "/foo/bar", false},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got, err := SanitizeUserPath(tt.input)
			if tt.wantErr {
				if err == nil {
					t.Fatalf("SanitizeUserPath(%q) expected error", tt.input)
				}
				return
			}
			if err != nil {
				t.Fatalf("SanitizeUserPath(%q) unexpected error: %v", tt.input, err)
			}
			if strings.Contains(got, `\`) {
				t.Errorf("SanitizeUserPath(%q) = %q contains backslash", tt.input, got)
			}
			if got != tt.want {
				t.Errorf("SanitizeUserPath(%q) = %q, want %q", tt.input, got, tt.want)
			}
		})
	}
}
