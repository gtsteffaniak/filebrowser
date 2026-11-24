package web

import (
	"context"
	"fmt"
	"os"
	"path/filepath"
	"testing"
	"time"

	"github.com/gtsteffaniak/filebrowser/backend/pkg/indexing"
	"github.com/gtsteffaniak/filebrowser/backend/pkg/indexing/iteminfo"
)

type duplicateCorpus struct {
	dir   string
	files []*iteminfo.FileInfo
}

func buildDuplicateCorpus(b *testing.B) *duplicateCorpus {
	b.Helper()
	dir := b.TempDir()
	now := time.Now()

	write := func(name string, payload []byte, mutateTail bool) {
		if mutateTail && len(payload) > 32 {
			dup := append([]byte(nil), payload...)
			dup[len(dup)-1] ^= 0xff
			payload = dup
		}
		if err := os.WriteFile(filepath.Join(dir, name), payload, 0o600); err != nil {
			b.Fatal(err)
		}
	}

	jpegHeader := []byte{0xff, 0xd8, 0xff, 0xe0}
	pngHeader := []byte{0x89, 0x50, 0x4e, 0x47}
	pdfHeader := []byte("%PDF-1.4\n")
	mp4Header := []byte{0x00, 0x00, 0x00, 0x18, 'f', 't', 'y', 'p'}

	mkPayload := func(header []byte, size int) []byte {
		out := make([]byte, size)
		copy(out, header)
		for i := len(header); i < len(out); i++ {
			out[i] = byte(i)
		}
		return out
	}

	baseJPEG := mkPayload(jpegHeader, 65536)
	basePNG := mkPayload(pngHeader, 65536)
	basePDF := mkPayload(pdfHeader, 65536)
	baseMP4 := mkPayload(mp4Header, 65536)

	entries := []struct {
		name string
		typ  string
		data []byte
	}{
		{"exact-a.jpg", "image/jpeg", baseJPEG},
		{"exact-b.jpg", "image/jpeg", baseJPEG},
		{"renamed-copy.jpeg", "image/jpeg", baseJPEG},
		{"user_exact-a.jpg", "image/jpeg", baseJPEG},
		{"png-a.png", "image/png", basePNG},
		{"png-b.png", "image/png", basePNG},
		{"doc-a.pdf", "application/pdf", basePDF},
		{"doc-b.pdf", "application/pdf", basePDF},
		{"clip-a.mp4", "video/mp4", baseMP4},
		{"clip-b.mp4", "video/mp4", baseMP4},
		{"sized-a.bin", "application/octet-stream", baseJPEG},
		{"sized-b.bin", "application/octet-stream", mkPayload(jpegHeader, 65536)},
	}

	files := make([]*iteminfo.FileInfo, 0, len(entries)+50)
	for _, e := range entries {
		write(e.name, e.data, false)
		files = append(files, &iteminfo.FileInfo{
			ItemInfo: iteminfo.ItemInfo{Name: e.name, Size: int64(len(e.data)), ModTime: now, Type: e.typ},
			Path:     "/" + e.name,
		})
	}
	write("tail-b.jpg", baseJPEG, true)

	for i := 0; i < 12; i++ {
		name := fmt.Sprintf("fuzzy_photo_%d.jpg", i)
		write(name, baseJPEG, false)
		files = append(files, &iteminfo.FileInfo{
			ItemInfo: iteminfo.ItemInfo{Name: name, Size: int64(len(baseJPEG)), ModTime: now, Type: "image/jpeg"},
			Path:     "/" + name,
		})
	}

	for i := 0; i < 30; i++ {
		name := fmt.Sprintf("bucket_%04d.jpg", i)
		write(name, mkPayload(jpegHeader, 65536), i%7 == 0)
		files = append(files, &iteminfo.FileInfo{
			ItemInfo: iteminfo.ItemInfo{Name: name, Size: 65536, ModTime: now, Type: "image/jpeg"},
			Path:     "/" + name,
		})
	}

	return &duplicateCorpus{dir: dir, files: files}
}

func BenchmarkDuplicateTypeBuckets(b *testing.B) {
	corpus := buildDuplicateCorpus(b)
	b.ResetTimer()
	b.ReportAllocs()
	for i := 0; i < b.N; i++ {
		_ = groupFilesByTypeBucket(corpus.files)
	}
}

func BenchmarkDuplicateFilenameGroups(b *testing.B) {
	corpus := buildDuplicateCorpus(b)
	subset := corpus.files[len(corpus.files)-30:]
	ctx := context.Background()
	stats := &duplicateProcessingStats{startTime: time.Now()}
	b.ResetTimer()
	b.ReportAllocs()
	for i := 0; i < b.N; i++ {
		_ = groupFilesByFilename(ctx, subset, 65536, stats)
	}
}

func BenchmarkDuplicateChecksumGrouping(b *testing.B) {
	corpus := buildDuplicateCorpus(b)
	pair := []*iteminfo.FileInfo{corpus.files[0], corpus.files[1]}
	idx := &indexing.Index{Path: corpus.dir}
	ctx := context.Background()
	stats := &duplicateProcessingStats{startTime: time.Now(), uniqueChecksums: map[string]bool{}}
	b.ResetTimer()
	b.ReportAllocs()
	for i := 0; i < b.N; i++ {
		_ = groupFilesByChecksum(ctx, pair, idx, pair[0].Size, stats)
	}
}

func BenchmarkDuplicateMerge(b *testing.B) {
	mkFiles := func(n int, prefix string) []*indexing.SearchResult {
		out := make([]*indexing.SearchResult, n)
		for i := range out {
			out[i] = &indexing.SearchResult{Path: fmt.Sprintf("/%s/%d", prefix, i)}
		}
		return out
	}
	groups := []duplicateGroupWithChecksums{
		{Size: 100, Checksum: "d1", Files: mkFiles(4, "a")},
		{Size: 100, Checksum: "d1", Files: mkFiles(3, "b")},
		{Size: 200, Checksum: "d2", Files: mkFiles(2, "c")},
	}
	b.ResetTimer()
	b.ReportAllocs()
	for i := 0; i < b.N; i++ {
		_ = mergeGroupsByChecksum(groups)
	}
}
