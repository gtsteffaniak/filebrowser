package utils

import (
	"os"
	"path/filepath"
	"runtime"
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

// TestIsTextFileReadsOnlySample guards against loading the whole file into memory:
// on a multi-GB file (e.g. a video opened in the file watcher) that exhausts RAM.
func TestIsTextFileReadsOnlySample(t *testing.T) {
	const fileSize = 256 << 20 // 256 MiB, sparse: no real disk usage

	path := filepath.Join(t.TempDir(), "large.log")
	f, err := os.Create(path)
	if err != nil {
		t.Fatalf("creating sample: %v", err)
	}
	if _, err = f.WriteString(strings.Repeat("log line\n", 1024)); err != nil {
		t.Fatalf("writing sample: %v", err)
	}
	if err = f.Truncate(fileSize); err != nil {
		t.Fatalf("growing sample: %v", err)
	}
	if err = f.Close(); err != nil {
		t.Fatalf("closing sample: %v", err)
	}

	var before, after runtime.MemStats
	runtime.ReadMemStats(&before)
	got, err := IsTextFile(path)
	runtime.ReadMemStats(&after)

	if err != nil {
		t.Fatalf("IsTextFile: %v", err)
	}
	if !got {
		t.Fatalf("IsTextFile(large text file) = false, want true")
	}
	if allocated := after.TotalAlloc - before.TotalAlloc; allocated > 1<<20 {
		t.Fatalf("IsTextFile allocated %d bytes for a %d-byte file, want only the sample", allocated, fileSize)
	}
}
