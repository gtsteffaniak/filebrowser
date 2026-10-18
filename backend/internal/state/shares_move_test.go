package state

import (
	"strings"
	"testing"

	"github.com/gtsteffaniak/filebrowser/backend/internal/utils"
)

func rewriteSharePathAfterMove(sharePath, oldPath, newPath string) string {
	oldPath = utils.AddTrailingSlashIfNotExists(oldPath)
	newPath = utils.AddTrailingSlashIfNotExists(newPath)
	norm := utils.AddTrailingSlashIfNotExists(sharePath)
	if !strings.HasPrefix(norm, oldPath) {
		return sharePath
	}
	suffix := strings.TrimPrefix(norm, oldPath)
	updated := strings.TrimSuffix(newPath+suffix, "/")
	if strings.HasSuffix(suffix, "/") {
		updated = utils.AddTrailingSlashIfNotExists(updated)
	}
	return updated
}

func TestRewriteSharePathAfterMove(t *testing.T) {
	got := rewriteSharePathAfterMove("/photos/2024/one.jpg", "/photos/", "/archive/photos/")
	want := "/archive/photos/2024/one.jpg"
	if got != want {
		t.Fatalf("got %q want %q", got, want)
	}
	if rewriteSharePathAfterMove("/backup/photos/", "/photos/", "/archive/photos/") != "/backup/photos/" {
		t.Fatal("substring match must not retarget unrelated share")
	}
}
