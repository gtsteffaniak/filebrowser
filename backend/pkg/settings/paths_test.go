package settings

import (
	"os"
	"path/filepath"
	"testing"
)

func TestExpandTilde(t *testing.T) {
	home, err := os.UserHomeDir()
	if err != nil {
		t.Fatalf("UserHomeDir: %v", err)
	}

	tests := []struct {
		name string
		in   string
		want string
	}{
		{name: "empty", in: "", want: ""},
		{name: "absolute", in: "/abs/path", want: "/abs/path"},
		{name: "relative", in: "relative/path", want: "relative/path"},
		{name: "tilde only", in: "~", want: home},
		{name: "tilde slash", in: "~/", want: filepath.Join(home, "")},
		{name: "tilde subpath", in: "~/Documents", want: filepath.Join(home, "Documents")},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got, err := ExpandTilde(tt.in)
			if err != nil {
				t.Fatalf("ExpandTilde(%q): %v", tt.in, err)
			}
			if got != tt.want {
				t.Fatalf("ExpandTilde(%q) = %q, want %q", tt.in, got, tt.want)
			}
		})
	}
}

func TestAbsPath_expandsTilde(t *testing.T) {
	home, err := os.UserHomeDir()
	if err != nil {
		t.Fatalf("UserHomeDir: %v", err)
	}

	got, err := AbsPath("~/")
	if err != nil {
		t.Fatalf("AbsPath: %v", err)
	}
	if got != home {
		t.Fatalf("AbsPath(\"~/\") = %q, want %q", got, home)
	}
}
