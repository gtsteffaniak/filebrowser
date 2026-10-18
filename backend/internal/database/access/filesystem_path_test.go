package access

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/gtsteffaniak/filebrowser/backend/internal/utils"
)

func TestResolveFilesystemIndexPathFollowsExistingSegment(t *testing.T) {
	root := t.TempDir()
	privateDir := filepath.Join(root, "privatestorage")
	if err := os.MkdirAll(privateDir, 0o755); err != nil {
		t.Fatal(err)
	}
	secret := filepath.Join(privateDir, "secret.txt")
	if err := os.WriteFile(secret, []byte("x"), 0o644); err != nil {
		t.Fatal(err)
	}

	got := resolveFilesystemIndexPath(root, utils.IndexPathFromNormalized("/privatestorage/secret.txt", false))
	if got.String() != "/privatestorage/secret.txt/" && got.String() != "/privatestorage/secret.txt" {
		t.Fatalf("resolved %q unexpectedly", got)
	}
}
