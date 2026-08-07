package files

import (
	"encoding/json"
	"strings"
	"testing"

	"github.com/gtsteffaniak/filebrowser/backend/pkg/indexing/iteminfo"
)

func TestNormalizeFileInfoListing_fileOmitsChildren(t *testing.T) {
	info := &iteminfo.ExtendedFileInfo{
		FileInfo: iteminfo.FileInfo{
			ItemInfo: iteminfo.ItemInfo{
				Name: "readme.txt",
				Type: "text/plain; charset=utf-8",
			},
			Files:   []iteminfo.ExtendedItemInfo{{}},
			Folders: []iteminfo.ItemInfo{{Name: "docs", Type: "directory"}},
		},
	}
	normalizeFileInfoListing(info)
	if info.Files != nil || info.Folders != nil {
		t.Fatalf("expected nil files/folders for non-directory, got files=%v folders=%v", info.Files, info.Folders)
	}
	raw, err := json.Marshal(info)
	if err != nil {
		t.Fatalf("marshal: %v", err)
	}
	body := string(raw)
	if strings.Contains(body, `"files"`) || strings.Contains(body, `"folders"`) {
		t.Fatalf("expected files/folders omitted for file response, got %s", body)
	}
}

func TestNormalizeFileInfoListing_directoryUsesEmptySlicesNotNull(t *testing.T) {
	info := &iteminfo.ExtendedFileInfo{
		FileInfo: iteminfo.FileInfo{
			ItemInfo: iteminfo.ItemInfo{
				Name: "docs",
				Type: "directory",
			},
		},
	}
	normalizeFileInfoListing(info)
	if info.Files == nil || info.Folders == nil {
		t.Fatal("expected non-nil empty slices for directory")
	}
	if len(info.Files) != 0 || len(info.Folders) != 0 {
		t.Fatalf("expected empty slices, got files=%d folders=%d", len(info.Files), len(info.Folders))
	}
}
