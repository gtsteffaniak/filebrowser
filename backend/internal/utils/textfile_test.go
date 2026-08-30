package utils

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestIsTextFile(t *testing.T) {
	t.Parallel()

	asciiPageStream := strings.Repeat("BT /Ft0 1 Tf <00430044004500460047> Tj ET\n", 300)

	tests := []struct {
		name    string
		content []byte
		want    bool
	}{
		{name: "empty", content: nil, want: true},
		{name: "plain text", content: []byte("hello\nworld\n"), want: true},
		{name: "utf-8 text", content: []byte("café ☕\n"), want: true},
		{name: "svg", content: []byte(`<?xml version="1.0"?><svg xmlns="http://www.w3.org/2000/svg"></svg>`), want: true},
		{name: "binary with null bytes", content: append([]byte("GIF89a"), make([]byte, 64)...), want: false},
		{
			name:    "pdf with an ascii text layer larger than the sample",
			content: append([]byte("%PDF-1.3\n"+asciiPageStream), 0xFF, 0xD8, 0xFF, 0xE0),
			want:    false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			path := filepath.Join(t.TempDir(), "sample")
			if err := os.WriteFile(path, tt.content, 0o600); err != nil {
				t.Fatalf("writing sample: %v", err)
			}

			got, err := IsTextFile(path)
			if err != nil {
				t.Fatalf("IsTextFile: %v", err)
			}
			if got != tt.want {
				t.Fatalf("IsTextFile(%s) = %v, want %v", tt.name, got, tt.want)
			}
		})
	}
}
