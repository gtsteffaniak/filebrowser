package files

import (
	"mime"
	"testing"
)

// Video containers that browsers cannot play must still be detected as video so
// the preview routes to the transcode UI instead of showing "no preview".
func TestVideoContainerExtensionsAreRegisteredAsVideo(t *testing.T) {
	tests := map[string]string{
		".wmv": "video/x-ms-wmv",
		".3gp": "video/3gpp",
		".3g2": "video/3gpp2",
		".mp4": "video/mp4",
		".avi": "video/x-msvideo",
		".mkv": "video/x-matroska",
	}
	for ext, want := range tests {
		got := mime.TypeByExtension(ext)
		if got != want {
			t.Errorf("mime.TypeByExtension(%q) = %q, want %q", ext, got, want)
		}
	}
}
