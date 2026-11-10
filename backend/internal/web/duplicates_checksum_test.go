package web

import (
	"context"
	"crypto/md5"
	"encoding/hex"
	"os"
	"path/filepath"
	"testing"
	"time"

	"github.com/cespare/xxhash/v2"
)

func TestReadFileSampleDigestsSmallFileUsesWholeFile(t *testing.T) {
	dir := t.TempDir()
	data := make([]byte, 12_000)
	path := filepath.Join(dir, "small.bin")
	if err := os.WriteFile(path, data, 0o600); err != nil {
		t.Fatal(err)
	}
	stats := &duplicateProcessingStats{startTime: time.Now()}
	d, err := readFileSampleDigests(context.Background(), dir, path, int64(len(data)), time.Now(), stats)
	if err != nil {
		t.Fatal(err)
	}
	if d.header != d.middle {
		t.Fatalf("small file header and middle digest should match: %q %q", d.header, d.middle)
	}
}

func TestHasherUsesXXHash(t *testing.T) {
	if _, ok := any(duplicateChecksumHasher()).(*xxhash.Digest); !ok {
		t.Fatal("expected xxhash digest")
	}
}

func BenchmarkDuplicateHashers(b *testing.B) {
	data := make([]byte, checksumSampleSize)
	for i := range data {
		data[i] = byte(i)
	}
	b.Run("md5", func(b *testing.B) {
		b.ReportAllocs()
		for i := 0; i < b.N; i++ {
			h := md5.New()
			_, _ = h.Write(data)
			hex.EncodeToString(h.Sum(nil))
		}
	})
	b.Run("xxhash", func(b *testing.B) {
		b.ReportAllocs()
		for i := 0; i < b.N; i++ {
			h := xxhash.New()
			_, _ = h.Write(data)
			hex.EncodeToString(h.Sum(nil))
		}
	})
}
