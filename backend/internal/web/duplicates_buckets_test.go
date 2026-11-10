package web

import (
	"testing"

	"github.com/gtsteffaniak/filebrowser/backend/pkg/indexing/iteminfo"
)

func TestTypeBucketKeyJPEGAliases(t *testing.T) {
	jpg := &iteminfo.FileInfo{ItemInfo: iteminfo.ItemInfo{Name: "a.jpg", Type: "image/jpeg"}}
	jpeg := &iteminfo.FileInfo{ItemInfo: iteminfo.ItemInfo{Name: "b.jpeg", Type: "image/jpeg"}}
	if typeBucketKey(jpg) != typeBucketKey(jpeg) {
		t.Fatalf("jpg and jpeg should share bucket: %q vs %q", typeBucketKey(jpg), typeBucketKey(jpeg))
	}
}

func TestTypeBucketKeyAmbiguousMIMEUsesExtension(t *testing.T) {
	a := &iteminfo.FileInfo{ItemInfo: iteminfo.ItemInfo{Name: "a.bin", Type: "application/octet-stream"}}
	b := &iteminfo.FileInfo{ItemInfo: iteminfo.ItemInfo{Name: "b.dat", Type: "application/octet-stream"}}
	if typeBucketKey(a) == typeBucketKey(b) {
		t.Fatal("different extensions should not share bucket for ambiguous MIME")
	}
}
