package indexing

import (
	"path/filepath"
	"strings"
	"testing"

	"github.com/gtsteffaniak/filebrowser/backend/common/utils"
)

func getDirectoryPathForRuleInheritanceBaseline(indexPath string, isDir bool) string {
	if indexPath == "" {
		return "/"
	}

	normalized := indexPath
	if !strings.HasPrefix(normalized, "/") {
		normalized = "/" + normalized
	}

	if isDir {
		return utils.AddTrailingSlashIfNotExists(normalized)
	}

	parent := filepath.Dir(normalized)
	if parent == "." || parent == "" {
		return "/"
	}

	return utils.AddTrailingSlashIfNotExists(parent)
}

func TestGetDirectoryPathForRuleInheritance(t *testing.T) {
	tests := []struct {
		name      string
		indexPath string
		isDir     bool
		expected  string
	}{
		{name: "empty path", indexPath: "", isDir: false, expected: "/"},
		{name: "root path file", indexPath: "/", isDir: false, expected: "/"},
		{name: "root path dir", indexPath: "/", isDir: true, expected: "/"},
		{name: "directory already normalized", indexPath: "/secret/", isDir: true, expected: "/secret/"},
		{name: "directory missing trailing slash", indexPath: "/secret", isDir: true, expected: "/secret/"},
		{name: "directory missing leading slash", indexPath: "secret/sub", isDir: true, expected: "/secret/sub/"},
		{name: "file in directory", indexPath: "/secret/video.mp4", isDir: false, expected: "/secret/"},
		{name: "file in nested directory", indexPath: "/secret/deep/video.mp4", isDir: false, expected: "/secret/deep/"},
		{name: "file at root with leading slash", indexPath: "/video.mp4", isDir: false, expected: "/"},
		{name: "file at root without leading slash", indexPath: "video.mp4", isDir: false, expected: "/"},
		{name: "file path with trailing slash", indexPath: "/secret/deep/", isDir: false, expected: "/secret/"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := getDirectoryPathForRuleInheritance(tt.indexPath, tt.isDir)
			if got != tt.expected {
				t.Fatalf("getDirectoryPathForRuleInheritance(%q, %v) = %q, want %q", tt.indexPath, tt.isDir, got, tt.expected)
			}
		})
	}
}

func BenchmarkGetDirectoryPathForRuleInheritanceFile(b *testing.B) {
	path := "/projects/archive-2026/subfolder/video.mp4"
	b.ReportAllocs()
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		_ = getDirectoryPathForRuleInheritance(path, false)
	}
}

func BenchmarkGetDirectoryPathForRuleInheritanceBaselineFile(b *testing.B) {
	path := "/projects/archive-2026/subfolder/video.mp4"
	b.ReportAllocs()
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		_ = getDirectoryPathForRuleInheritanceBaseline(path, false)
	}
}

func BenchmarkGetDirectoryPathForRuleInheritanceDir(b *testing.B) {
	path := "/projects/archive-2026/subfolder/"
	b.ReportAllocs()
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		_ = getDirectoryPathForRuleInheritance(path, true)
	}
}

func BenchmarkGetDirectoryPathForRuleInheritanceBaselineDir(b *testing.B) {
	path := "/projects/archive-2026/subfolder/"
	b.ReportAllocs()
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		_ = getDirectoryPathForRuleInheritanceBaseline(path, true)
	}
}
