package web

import (
	"io/fs"
	"testing"
	"testing/fstest"
)

func TestEmbeddedFrontendInDist(t *testing.T) {
	tests := []struct {
		name string
		dist fs.FS
		want bool
	}{
		{
			name: "placeholder only",
			dist: fstest.MapFS{".gitkeep": &fstest.MapFile{}},
			want: false,
		},
		{
			name: "empty dist",
			dist: fstest.MapFS{},
			want: false,
		},
		{
			name: "built frontend",
			dist: fstest.MapFS{
				"public/index.html": &fstest.MapFile{Data: []byte("<!doctype html>")},
			},
			want: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := embeddedFrontendInDist(tt.dist); got != tt.want {
				t.Fatalf("embeddedFrontendInDist() = %v, want %v", got, tt.want)
			}
		})
	}
}

func TestEmbeddedFrontendAvailable_matchesCompileTimeEmbed(t *testing.T) {
	sub, err := fs.Sub(assets, "dist")
	if err != nil {
		t.Fatalf("fs.Sub(assets, dist): %v", err)
	}
	want := embeddedFrontendInDist(sub)
	if got := EmbeddedFrontendAvailable(); got != want {
		t.Fatalf("EmbeddedFrontendAvailable() = %v, want %v (compile-time embed)", got, want)
	}
}
