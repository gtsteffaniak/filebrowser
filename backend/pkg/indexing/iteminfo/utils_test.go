package iteminfo

import (
	"errors"
	"os"
	"path/filepath"
	"testing"
)

func TestResolveSymlinks_NotExistIsWrappable(t *testing.T) {
	dir := t.TempDir()
	missing := filepath.Join(dir, "does-not-exist")

	_, _, err := ResolveSymlinks(missing)
	if err == nil {
		t.Fatal("expected error for missing path")
	}
	if !errors.Is(err, os.ErrNotExist) {
		t.Fatalf("expected errors.Is(ErrNotExist), got %v", err)
	}
}
