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

func TestCachedDigestsDoNotConsumeChecksumBudget(t *testing.T) {
	dir := t.TempDir()
	dataA := make([]byte, 65536)
	for i := range dataA {
		dataA[i] = byte(i % 251)
	}
	// Use a distinct payload so its digests differ from other tests' files.
	pathA := filepath.Join(dir, "budget-a.bin")
	pathB := filepath.Join(dir, "budget-b.bin")
	if err := os.WriteFile(pathA, dataA, 0o600); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(pathB, dataA, 0o600); err != nil {
		t.Fatal(err)
	}
	modTime := time.Date(2026, 11, 10, 0, 0, 0, 0, time.UTC)

	// Prime the cache with one op-worth of budget.
	prime := &duplicateProcessingStats{startTime: time.Now(), uniqueChecksums: map[string]bool{}}
	d1, err := readFileSampleDigests(context.Background(), dir, pathA, int64(len(dataA)), modTime, prime)
	if err != nil {
		t.Fatal(err)
	}
	d2, err := readFileSampleDigests(context.Background(), dir, pathB, int64(len(dataA)), modTime, prime)
	if err != nil {
		t.Fatal(err)
	}
	if d1.header != d2.header || d1.middle != d2.middle {
		t.Fatal("expected equal digests for identical files")
	}
	if prime.checksumOperations != 2 {
		t.Fatalf("expected 2 disk operations for two cache-miss reads, got %d", prime.checksumOperations)
	}

	// Exhaust the budget: any further uncached file should be refused.
	prime.checksumOperations = maxChecksumOperations
	uncached := filepath.Join(dir, "budget-c.bin")
	if err = os.WriteFile(uncached, dataA, 0o600); err != nil {
		t.Fatal(err)
	}
	if _, err = readFileSampleDigests(context.Background(), dir, uncached, int64(len(dataA)), modTime, prime); err == nil {
		t.Fatal("expected budget-exhausted error for uncached file")
	}

	// But the cached files must still be served, for free.
	cached, err := readFileSampleDigests(context.Background(), dir, pathA, int64(len(dataA)), modTime, prime)
	if err != nil {
		t.Fatalf("cached digest should be served despite exhausted budget: %v", err)
	}
	if cached.header != d1.header || cached.middle != d1.middle {
		t.Fatal("cached digest mismatch")
	}
	if prime.checksumOperations != maxChecksumOperations {
		t.Fatalf("cached digest must not consume budget, got %d", prime.checksumOperations)
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
