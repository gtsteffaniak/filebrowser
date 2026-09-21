package web

import (
	"fmt"
	"net/http"
	"os"
	"path/filepath"
	"testing"

	libErrors "github.com/gtsteffaniak/filebrowser/backend/internal/errors"
	"github.com/gtsteffaniak/filebrowser/backend/pkg/indexing/iteminfo"
)

func TestRealPathErrStatus(t *testing.T) {
	if realPathErrStatus(libErrors.ErrPathEscapesScope) != http.StatusForbidden {
		t.Fatal("expected 403 for ErrPathEscapesScope")
	}
	if realPathErrStatus(os.ErrNotExist) != http.StatusNotFound {
		t.Fatal("expected 404 for ErrNotExist")
	}
	if realPathErrStatus(fmt.Errorf("disk failure")) != http.StatusInternalServerError {
		t.Fatal("expected 500 for unknown error")
	}

	dir := t.TempDir()
	missing := filepath.Join(dir, "gone")
	_, _, err := iteminfo.ResolveSymlinks(missing)
	if err == nil {
		t.Fatal("expected resolve error")
	}
	if realPathErrStatus(err) != http.StatusNotFound {
		t.Fatalf("expected 404 for ResolveSymlinks missing file, got %d (%v)", realPathErrStatus(err), err)
	}
}
