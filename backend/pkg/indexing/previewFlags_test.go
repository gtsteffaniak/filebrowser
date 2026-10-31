package indexing

import (
	"testing"

	"github.com/gtsteffaniak/filebrowser/backend/pkg/indexing/iteminfo"
)

func TestSetFilePreviewFlagsLargeImages(t *testing.T) {
	large := iteminfo.LargeFileSizeThreshold + 1
	tests := []struct {
		name string
		file string
		typ  string
		size int64
		want bool
	}{
		{"normal image under the limit", "a.jpg", "image/jpeg", 5 << 20, true},
		{"normal image over the limit", "a.jpg", "image/jpeg", int64(large), false},
		{"RAW under the limit", "a.raf", "image/x-fuji-raf", 26 << 20, true},
		{"large RAW keeps its preview, the embedded JPEG is read without decoding", "a.RAF", "image/x-fuji-raf", int64(large), true},
		{"large DNG keeps its preview", "a.dng", "image/x-adobe-dng", int64(large), true},
		{"empty file has no preview", "a.raf", "image/x-fuji-raf", 0, false},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			info := &iteminfo.ItemInfo{Name: tt.file, Type: tt.typ, Size: tt.size}
			setFilePreviewFlags(info, "/nonexistent/"+tt.file)
			if info.HasPreview != tt.want {
				t.Fatalf("HasPreview = %v, want %v", info.HasPreview, tt.want)
			}
		})
	}
}
