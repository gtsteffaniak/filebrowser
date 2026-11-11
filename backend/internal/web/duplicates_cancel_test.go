package web

import (
	"context"
	"testing"
	"time"
)

func TestChecksumCacheRejectsExpiredDigest(t *testing.T) {
	key := checksumCacheKey{
		sourcePath: "/src",
		filePath:   "/file-a",
		size:       4096,
		modTime:    42,
		kind:       checksumKindHeader,
	}
	encoded := checksumCacheKeyString(key)
	checksumTypedCache.SetWithExp(encoded, "digest", time.Millisecond)
	time.Sleep(5 * time.Millisecond)
	if _, ok := getChecksumFromCache(key); ok {
		t.Fatal("expired digest should not be returned")
	}
}

func TestChecksumCacheMissAfterFileIdentityChange(t *testing.T) {
	modA := checksumCacheKey{
		sourcePath: "/src",
		filePath:   "/file-a",
		size:       4096,
		modTime:    100,
		kind:       checksumKindHeader,
	}
	modB := modA
	modB.modTime = 200

	checksumTypedCache.Set(checksumCacheKeyString(modA), "digest-a")
	if _, ok := getChecksumFromCache(modB); ok {
		t.Fatal("modified file identity must not hit the prior cache key")
	}
}

func TestCacheDuplicateResponseSkipsCanceledContext(t *testing.T) {
	ctx, cancel := context.WithCancel(context.Background())
	cancel()

	cacheKey := "cancel-test:" + time.Now().Format(time.RFC3339Nano)
	cacheDuplicateResponse(ctx, cacheKey, duplicateResponse{
		Groups: []duplicateGroup{{Size: 1, Count: 2}},
	})
	if _, ok := duplicateResultsCache.Get(cacheKey); ok {
		t.Fatal("canceled context must not cache duplicate results")
	}

	cacheDuplicateResponse(context.Background(), cacheKey, duplicateResponse{
		Groups: []duplicateGroup{{Size: 2, Count: 2}},
	})
	if _, ok := duplicateResultsCache.Get(cacheKey); !ok {
		t.Fatal("active context should cache duplicate results")
	}
}
