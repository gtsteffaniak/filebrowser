package ffmpeg

import (
	"os"
	"path/filepath"
	"testing"
)

func TestResolveLocalMediaPath(t *testing.T) {
	t.Parallel()

	tmpFile, err := os.CreateTemp(t.TempDir(), "probe-*")
	if err != nil {
		t.Fatalf("CreateTemp() error = %v", err)
	}
	tmpPath := tmpFile.Name()
	tmpFile.Close()

	tests := []struct {
		name    string
		path    string
		wantErr bool
	}{
		{name: "absolute local file", path: tmpPath},
		{name: "empty path", path: "", wantErr: true},
		{name: "remote url", path: "http://example.com/video.mp4", wantErr: true},
		{name: "rtsp url", path: "rtsp://camera/stream", wantErr: true},
		{name: "pipe protocol", path: "pipe:0", wantErr: true},
		{name: "relative path", path: "video.mp4", wantErr: true},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			got, info, err := resolveLocalMediaPath(tt.path)
			if tt.wantErr {
				if err == nil {
					t.Fatalf("resolveLocalMediaPath(%q) error = nil, want error", tt.path)
				}
				return
			}
			if err != nil {
				t.Fatalf("resolveLocalMediaPath(%q) error = %v", tt.path, err)
			}
			if got != filepath.Clean(tt.path) {
				t.Fatalf("resolveLocalMediaPath(%q) = %q, want %q", tt.path, got, filepath.Clean(tt.path))
			}
			if info == nil || info.IsDir() {
				t.Fatalf("resolveLocalMediaPath(%q) returned invalid file info", tt.path)
			}
		})
	}
}
