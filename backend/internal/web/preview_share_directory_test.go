package web

import (
	"context"
	"errors"
	"net/http/httptest"
	"os"
	"path/filepath"
	"testing"

	"github.com/gtsteffaniak/filebrowser/backend/internal/database/share"
	"github.com/gtsteffaniak/filebrowser/backend/internal/database/users"
	commonerrors "github.com/gtsteffaniak/filebrowser/backend/internal/errors"
	"github.com/gtsteffaniak/filebrowser/backend/pkg/indexing"
	"github.com/gtsteffaniak/filebrowser/backend/pkg/indexing/iteminfo"
)

func TestGetDirectoryPreview_ShareRejectsSymlinkChildOutsideShare(t *testing.T) {
	sourceRoot, adminUser := setupSubtitlesHTTPTest(t)
	sharedDir := filepath.Join(sourceRoot, "shared")
	privateDir := filepath.Join(sourceRoot, "private")
	for _, dir := range []string{sharedDir, privateDir} {
		if err := os.MkdirAll(dir, 0o755); err != nil {
			t.Fatal(err)
		}
	}
	if err := os.WriteFile(filepath.Join(privateDir, "secret.jpg"), []byte{0xff, 0xd8, 0xff, 0xd9}, 0o644); err != nil {
		t.Fatal(err)
	}
	if err := os.Symlink(filepath.Join("..", "private", "secret.jpg"), filepath.Join(sharedDir, "link.jpg")); err != nil {
		t.Fatal(err)
	}

	ctx := context.Background()
	req := httptest.NewRequest("GET", "/", nil)
	adminUser.BackendScopes = []users.BackendScope{{Path: sourceRoot, Scope: "/"}}
	d := &Context{
		User:      adminUser,
		ShareUser: adminUser,
		IndexPath: "/shared/",
		Share: share.Share{
			ShareColumns: share.ShareColumns{Hash: "h", Path: "/shared"},
			SourcePath:   sourceRoot,
		},
		FileInfo: iteminfo.ExtendedFileInfo{
			Source: "Downloads",
			FileInfo: iteminfo.FileInfo{
				Path: "/shared/",
				ItemInfo: iteminfo.ItemInfo{
					Type: "directory",
				},
				Files: []iteminfo.ExtendedItemInfo{
					{ItemInfo: iteminfo.ItemInfo{Name: "link.jpg", Type: "image/jpeg", HasPreview: true}},
				},
			},
		},
	}
	idx := indexing.GetIndex("Downloads")
	if idx == nil {
		t.Fatal("missing index")
	}

	_, err := getDirectoryPreview(ctx, req, d, 0)
	if err == nil {
		t.Fatal("expected scope error for share child symlink outside share root")
	}
	if !errors.Is(err, commonerrors.ErrPathEscapesScope) {
		t.Fatalf("error=%v want ErrPathEscapesScope", err)
	}
}
