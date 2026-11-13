package fulltext

import (
	"os"
	"path/filepath"
	"strings"
	"syscall"
	"testing"
)

func TestExtract(t *testing.T) {
	dir := t.TempDir()
	write := func(name string, data []byte) string {
		p := filepath.Join(dir, name)
		if err := os.WriteFile(p, data, 0o644); err != nil {
			t.Fatal(err)
		}
		return p
	}

	t.Run("text file includes name and content", func(t *testing.T) {
		p := write("meeting.md", []byte("# Notes\nquarterly budget"))
		got := Extract(p, "meeting.md", "text/markdown")
		if got == nil || !strings.Contains(*got, "meeting.md") || !strings.Contains(*got, "quarterly budget") {
			t.Fatalf("got %v", got)
		}
	})

	t.Run("extension without MIME mapping still indexed", func(t *testing.T) {
		p := write("main.go", []byte("package main // hello"))
		got := Extract(p, "main.go", "blob")
		if got == nil || !strings.Contains(*got, "package main") {
			t.Fatalf("got %v", got)
		}
	})

	t.Run("binary content indexes name only", func(t *testing.T) {
		p := write("blob.txt", []byte("abc\x00def"))
		got := Extract(p, "blob.txt", "text/plain")
		if got == nil || *got != "blob.txt" {
			t.Fatalf("got %v", got)
		}
	})

	t.Run("unsupported type returns nil", func(t *testing.T) {
		p := write("pic.png", []byte("\x89PNG"))
		if got := Extract(p, "pic.png", "image/png"); got != nil {
			t.Fatalf("want nil, got %v", *got)
		}
	})

	t.Run("missing file returns nil", func(t *testing.T) {
		if got := Extract(filepath.Join(dir, "gone.txt"), "gone.txt", "text/plain"); got != nil {
			t.Fatalf("got %v", *got)
		}
	})

	t.Run("fifo is not opened", func(t *testing.T) {
		p := filepath.Join(dir, "pipe.txt")
		if err := syscall.Mkfifo(p, 0o600); err != nil {
			t.Skip("mkfifo unavailable:", err)
		}
		if got := Extract(p, "pipe.txt", "text/plain"); got != nil {
			t.Fatalf("fifo should be name-only, got %v", *got)
		}
	})

	t.Run("content is capped", func(t *testing.T) {
		big := strings.Repeat("a", MaxPlainTextBytes+100)
		p := write("big.txt", []byte(big))
		got := Extract(p, "big.txt", "text/plain")
		if got == nil || len(*got) > MaxPlainTextBytes+len("big.txt")+1 {
			t.Fatalf("content not capped: %d", len(*got))
		}
	})
}
