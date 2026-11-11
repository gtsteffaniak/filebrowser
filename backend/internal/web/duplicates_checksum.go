package web

import (
	"context"
	"encoding/hex"
	"fmt"
	"hash"
	"io"
	"os"
	"path/filepath"
	"sync"
	"time"

	"github.com/cespare/xxhash/v2"
	"github.com/gtsteffaniak/filebrowser/backend/pkg/indexing"
	"github.com/gtsteffaniak/filebrowser/backend/pkg/indexing/iteminfo"
	"github.com/gtsteffaniak/go-cache/cache"
)

const (
	checksumSampleSize = 8192
	checksumSmallFile  = 16384
	checksumCacheTTL   = time.Hour
)

type checksumSampleKind uint8

const (
	checksumKindHeader checksumSampleKind = iota
	checksumKindMiddle
)

// checksumCacheKey identifies a cached sample digest for a file snapshot.
type checksumCacheKey struct {
	sourcePath string
	filePath   string
	size       int64
	modTime    int64
	kind       checksumSampleKind
}

var (
	// checksumTypedCache stores sample digests keyed by string-encoded checksumCacheKey.
	// The go-cache backend sweeps expired entries on an hourly ticker, so stale keys from
	// renamed/deleted/modified files are evicted without a lookup — unlike a bare sync.Map
	// which would grow without bound. Cleanup interval is the second arg (1h, matching TTL).
	checksumTypedCache = cache.NewCache[string](checksumCacheTTL, checksumCacheTTL)
	checksumBufPool    = sync.Pool{
		New: func() any {
			buf := make([]byte, checksumSampleSize)
			return &buf
		},
	}
)

type fileSampleDigests struct {
	header string
	middle string
}

func duplicateChecksumHasher() hash.Hash {
	return xxhash.New()
}

func digestHex(h hash.Hash) string {
	sum := h.Sum(nil)
	dst := make([]byte, hex.EncodedLen(len(sum)))
	hex.Encode(dst, sum)
	return string(dst)
}

// checksumCacheKeyString encodes a typed cache key as a single string so it can
// be stored in the TTL-sweeping go-cache. Length prefixes keep fields unambiguous.
func checksumCacheKeyString(key checksumCacheKey) string {
	return fmt.Sprintf("%d|%d|%d|%d|%s|%s",
		key.kind, key.size, key.modTime, len(key.sourcePath),
		key.sourcePath, key.filePath)
}

func getChecksumFromCache(key checksumCacheKey) (string, bool) {
	return checksumTypedCache.Get(checksumCacheKeyString(key))
}

func setChecksumCache(key checksumCacheKey, digest string) {
	checksumTypedCache.Set(checksumCacheKeyString(key), digest)
}

func readFileSampleDigests(ctx context.Context, sourcePath, filePath string, size int64, modTime time.Time, stats *duplicateProcessingStats) (fileSampleDigests, error) {
	headerKey := checksumCacheKey{sourcePath: sourcePath, filePath: filePath, size: size, modTime: modTime.UnixNano(), kind: checksumKindHeader}
	middleKey := checksumCacheKey{sourcePath: sourcePath, filePath: filePath, size: size, modTime: modTime.UnixNano(), kind: checksumKindMiddle}

	var out fileSampleDigests
	if h, ok := getChecksumFromCache(headerKey); ok {
		out.header = h
	}
	if m, ok := getChecksumFromCache(middleKey); ok {
		out.middle = m
	}
	if out.header != "" && out.middle != "" {
		return out, nil
	}

	if ctx.Err() != nil {
		return fileSampleDigests{}, ctx.Err()
	}
	// Only count operations that actually touch disk. Cache hits are O(1) lookups
	// and must not consume the checksum budget — otherwise a second search over
	// the same files stalls at the limit without reading a single new byte.
	// The budget check lives here (post-cache) so cached digests are always served.
	if shouldStop, _ := stats.shouldStop(); shouldStop {
		return fileSampleDigests{}, fmt.Errorf("duplicate search budget exhausted")
	}

	file, err := os.Open(filePath)
	if err != nil {
		return fileSampleDigests{}, err
	}
	defer file.Close()

	// A disk read is happening — charge it against the checksum budget.
	stats.checksumOperations++

	raw := checksumBufPool.Get()
	bufPtr, ok := raw.(*[]byte)
	if !ok {
		return fileSampleDigests{}, fmt.Errorf("invalid checksum buffer pool type")
	}
	buf := *bufPtr
	defer checksumBufPool.Put(bufPtr)

	if size <= checksumSmallFile {
		h := duplicateChecksumHasher()
		remain := size
		for remain > 0 {
			if ctx.Err() != nil {
				return fileSampleDigests{}, ctx.Err()
			}
			toRead := int64(len(buf))
			if toRead > remain {
				toRead = remain
			}
			n, readErr := file.Read(buf[:toRead])
			if n > 0 {
				_, _ = h.Write(buf[:n])
				remain -= int64(n)
			}
			if readErr == io.EOF {
				break
			}
			if readErr != nil {
				return fileSampleDigests{}, readErr
			}
			if n == 0 {
				break
			}
		}
		d := digestHex(h)
		out.header = d
		out.middle = d
	} else if out.header == "" || out.middle == "" {
		n, readErr := io.ReadFull(file, buf)
		if readErr != nil && readErr != io.EOF && readErr != io.ErrUnexpectedEOF {
			return fileSampleDigests{}, readErr
		}

		if out.header == "" {
			h := duplicateChecksumHasher()
			_, _ = h.Write(buf[:n])
			out.header = digestHex(h)
		}
		if out.middle == "" {
			middle := duplicateChecksumHasher()
			_, _ = middle.Write(buf[:n])
			middleOffset := size / 2
			if _, err := file.Seek(middleOffset, io.SeekStart); err != nil {
				return fileSampleDigests{}, err
			}
			n, readErr := io.ReadFull(file, buf)
			if readErr != nil && readErr != io.EOF && readErr != io.ErrUnexpectedEOF {
				return fileSampleDigests{}, readErr
			}
			_, _ = middle.Write(buf[:n])
			out.middle = digestHex(middle)
		}
	}

	if out.header != "" {
		setChecksumCache(headerKey, out.header)
	}
	if out.middle != "" {
		setChecksumCache(middleKey, out.middle)
	}
	return out, nil
}

func groupFilesByChecksum(ctx context.Context, files []*iteminfo.FileInfo, index *indexing.Index, fileSize int64, stats *duplicateProcessingStats) []checksumGroup {
	if len(files) < 2 {
		return nil
	}

	headerGroups := make(map[string][]*iteminfo.FileInfo)
	digestByPath := make(map[string]fileSampleDigests, len(files))

	for _, file := range files {
		if ctx.Err() != nil {
			return nil
		}
		// Budget checks live inside readFileSampleDigests (after the cache lookup)
		// so cached digests still count toward grouping without consuming budget.
		// Checking here would stall on cached files and look stuck.

		if file.Size != fileSize {
			continue
		}

		filePath := filepath.Join(index.Path, file.Path)
		digests, err := readFileSampleDigests(ctx, index.Path, filePath, fileSize, file.ModTime, stats)
		if err != nil {
			continue
		}
		digestByPath[file.Path] = digests
		stats.uniqueChecksums[digests.header] = true
		headerGroups[digests.header] = append(headerGroups[digests.header], file)
	}

	headerCandidates := make([][]*iteminfo.FileInfo, 0)
	for _, group := range headerGroups {
		if len(group) >= 2 {
			headerCandidates = append(headerCandidates, group)
		}
	}
	if len(headerCandidates) == 0 {
		return nil
	}

	middleGroups := make(map[string][]*iteminfo.FileInfo)
	for _, headerGroup := range headerCandidates {
		for _, file := range headerGroup {
			if ctx.Err() != nil {
				return nil
			}
			if shouldStop, _ := stats.shouldStop(); shouldStop {
				break
			}
			digests := digestByPath[file.Path]
			if digests.middle == "" {
				continue
			}
			stats.uniqueChecksums[digests.middle] = true
			middleGroups[digests.middle] = append(middleGroups[digests.middle], file)
		}
	}

	groups := make([]checksumGroup, 0, len(middleGroups))
	for checksum, group := range middleGroups {
		if len(group) >= 2 {
			groups = append(groups, checksumGroup{
				Files:    group,
				Checksum: checksum,
			})
		}
	}
	return groups
}
