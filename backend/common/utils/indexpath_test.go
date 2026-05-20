package utils

import "testing"

func TestIndexPathFromNormalized_String(t *testing.T) {
	tests := []struct {
		name     string
		input    string
		isDir    bool
		expected string
	}{
		{"root slash", "/", true, "/"},
		{"empty", "", true, "/"},
		{"hidden at root", "/.test", true, "/.test/"},
		{"trailing slash dir", "/test/", true, "/test/"},
		{"nested dir", "/nested/path", true, "/nested/path/"},
		{"file no trailing", "/test/file.txt", false, "/test/file.txt"},
		{"single file segment", "file.txt", false, "/file.txt"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := IndexPathFromNormalized(tt.input, tt.isDir).String()
			if got != tt.expected {
				t.Errorf("IndexPathFromNormalized(%q, %v).String() = %q, want %q", tt.input, tt.isDir, got, tt.expected)
			}
		})
	}
}

func TestParseSanitizedIndexPath(t *testing.T) {
	_, err := ParseSanitizedIndexPath("..", true)
	if err == nil {
		t.Fatal("expected error for .. path")
	}

	got, err := ParseSanitizedIndexPath("/valid/path/", true)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if got.String() != "/valid/path/" {
		t.Errorf("got %q want /valid/path/", got.String())
	}

	// SanitizeUserPath resolves traversal; result is a normal index path
	got, err = ParseSanitizedIndexPath("/../secret", true)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if got.String() != "/secret/" {
		t.Errorf("got %q want /secret/", got.String())
	}
}

func TestIndexPath_Parent(t *testing.T) {
	tests := []struct {
		name     string
		path     IndexPath
		expected string
	}{
		{"root parent", IndexPathFromNormalized("/", true), "/"},
		{"one level", IndexPathFromNormalized("/a/", true), "/"},
		{"two levels", IndexPathFromNormalized("/a/b/", true), "/a/"},
		{"file parent is dir", IndexPathFromNormalized("/a/b.txt", false), "/a/"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := tt.path.Parent().String()
			if got != tt.expected {
				t.Errorf("Parent().String() = %q, want %q", got, tt.expected)
			}
		})
	}
}

func TestIndexPath_Join(t *testing.T) {
	root := IndexPathFromNormalized("/", true)
	child := root.Join("test", true)
	if child.String() != "/test/" {
		t.Errorf("Join dir got %q, want /test/", child.String())
	}
	file := root.Join("readme.txt", false)
	if file.String() != "/readme.txt" {
		t.Errorf("Join file got %q, want /readme.txt", file.String())
	}
}

func TestIndexPath_RuleKey(t *testing.T) {
	file := IndexPathFromNormalized("/foo/bar.txt", false)
	if file.RuleKey() != "/foo/bar.txt/" {
		t.Errorf("RuleKey() = %q, want /foo/bar.txt/", file.RuleKey())
	}
	dir := IndexPathFromNormalized("/foo/", true)
	if dir.RuleKey() != "/foo/" {
		t.Errorf("RuleKey() = %q, want /foo/", dir.RuleKey())
	}
}

func TestAddTrailingSlashIfNotExists(t *testing.T) {
	if AddTrailingSlashIfNotExists("/foo") != "/foo/" {
		t.Errorf("expected /foo/")
	}
	if AddTrailingSlashIfNotExists("/") != "/" {
		t.Errorf("root should stay /")
	}
}

func BenchmarkIndexPathFromNormalized(b *testing.B) {
	const input = "/nested/path/to/resource/"
	b.ReportAllocs()
	for b.Loop() {
		_ = IndexPathFromNormalized(input, true)
	}
}

func BenchmarkParseSanitizedIndexPath(b *testing.B) {
	const input = "/nested/path/to/resource/"
	b.ReportAllocs()
	for b.Loop() {
		_, _ = ParseSanitizedIndexPath(input, true)
	}
}
