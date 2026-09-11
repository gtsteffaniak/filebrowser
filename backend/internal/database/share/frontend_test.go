package share

import (
	"testing"

	"github.com/gtsteffaniak/filebrowser/backend/pkg/settings"
)

func TestPublicShareURLPasswordProtectedDirectLink(t *testing.T) {
	orig := settings.Config.Http
	t.Cleanup(func() { settings.Config.Http = orig })
	settings.Config.Http.BaseURL = "/files/"

	got := PublicShareURL("example.com", "https", "abc123", true, true)
	want := "https://example.com/files/public/share/abc123?download=true"
	if got != want {
		t.Fatalf("PublicShareURL() = %q, want %q", got, want)
	}
}

func TestPublicShareURLDirectDownloadWithoutPassword(t *testing.T) {
	orig := settings.Config.Http
	t.Cleanup(func() { settings.Config.Http = orig })
	settings.Config.Http.BaseURL = "/files/"

	got := PublicShareURL("example.com", "https", "abc123", true, false)
	want := "https://example.com/files/public/api/resources/download?hash=abc123"
	if got != want {
		t.Fatalf("PublicShareURL() = %q, want %q", got, want)
	}
}
