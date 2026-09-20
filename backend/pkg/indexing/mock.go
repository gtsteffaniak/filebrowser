package indexing

import (
	"math/rand"
	"mime"
	"path/filepath"
	"strings"
	"time"

	"github.com/gtsteffaniak/filebrowser/backend/internal/utils"
	"github.com/gtsteffaniak/filebrowser/backend/pkg/indexing/iteminfo"
)

func (idx *Index) CreateMockData(numDirs, numFilesPerDir int) {
	r := rand.New(rand.NewSource(int64(numDirs*1_000_003 + numFilesPerDir)))
	for i := 0; i < numDirs; i++ {
		dirPath := utils.GenerateRandomPathSeeded(r, r.Intn(3)+1)
		files := []iteminfo.ExtendedItemInfo{}

		// Simulating files and directories with ExtendedItemInfo
		for j := 0; j < numFilesPerDir; j++ {
			name := "file-" + utils.GetRandomTermSeeded(r) + utils.GetRandomExtensionSeeded(r)
			newFile := iteminfo.ExtendedItemInfo{
				ItemInfo: iteminfo.ItemInfo{
					Name:    name,
					Size:    mockSizeFor(name, r),
					ModTime: time.Now().Add(-time.Duration(r.Intn(100)) * time.Hour),
					Type:    mockMimeType(name),
				},
			}
			files = append(files, newFile)
		}
		dirInfo := &iteminfo.FileInfo{
			Path:  dirPath,
			Files: files,
		}

		idx.UpdateMetadata(dirInfo, nil, true) // nil scanner for mock
	}
}

// CreateMockData builds a deterministic listing for performance measurement.
//
// Determinism matters: mock row names, sizes and mtimes feed directly into DOM
// text length and JSON payload size, so unseeded randomness added real variance
// between runs and made payload metrics impossible to baseline. The seed is
// derived from the requested shape, so the same numDirs/numFiles always yields
// identical output. Pass a non-zero seed to override.
func CreateMockData(numDirs, numFilesPerDir int) iteminfo.FileInfo {
	return CreateMockDataSeeded(numDirs, numFilesPerDir, 0)
}

// CreateMockDataSeeded is CreateMockData with an explicit seed.
func CreateMockDataSeeded(numDirs, numFilesPerDir int, seed int64) iteminfo.FileInfo {
	if seed == 0 {
		seed = MockDataSeed(numDirs, numFilesPerDir)
	}
	// Fixed time base: using time.Now() here made every row's ModTime differ
	// between runs, which changed formatted cell text and thus DOM payload.
	baseTime := time.Date(2024, time.January, 1, 0, 0, 0, 0, time.UTC)
	r := rand.New(rand.NewSource(seed))

	dir := iteminfo.FileInfo{}
	dir.Path = "/here/is/your/mock/dir"

	// Folders must genuinely be directories. The frontend decides whether a row
	// is navigable purely from `item.type === "directory"`, so emitting
	// file-shaped names with a file MIME type here produced rows that looked
	// like folders in the payload but behaved as files in the UI.
	for i := 0; i < numDirs; i++ {
		dir.Folders = append(dir.Folders, iteminfo.ItemInfo{
			Name:    "folder-" + utils.GetRandomTermSeeded(r),
			Size:    0,
			ModTime: baseTime.Add(-time.Duration(r.Intn(100)) * time.Hour),
			Type:    "directory",
		})
	}

	// Files carry their real MIME type so the listing exercises preview
	// eligibility, type-based icons and media handling.
	for j := 0; j < numFilesPerDir; j++ {
		name := "file-" + utils.GetRandomTermSeeded(r) + utils.GetRandomExtensionSeeded(r)
		newFile := iteminfo.ExtendedItemInfo{
			ItemInfo: iteminfo.ItemInfo{
				Name:    name,
				Size:    mockSizeFor(name, r),
				ModTime: baseTime.Add(-time.Duration(r.Intn(100)) * time.Hour),
				Type:    mockMimeType(name),
			},
		}
		dir.Files = append(dir.Files, newFile)
	}
	return dir
}

// mockMimeType resolves a MIME type from a filename's extension.
//
// Extension-only by design. Real indexing uses ItemInfo.DetectType, which
// additionally sniffs file headers to disambiguate extensions like `.ts`; mock
// rows have no file on disk, so sniffing would just return "blob" and undo the
// point of typing them at all. A plain extension lookup is enough to exercise
// the type-dependent UI paths (icons, preview eligibility, size formatting).
//
// The charset parameter is stripped so the values match what indexed items
// carry: mime.TypeByExtension(".txt") returns "text/plain; charset=utf-8",
// whereas a real indexed item has "text/plain".
func mockMimeType(name string) string {
	ext := strings.ToLower(filepath.Ext(name))
	if m := strings.Split(mime.TypeByExtension(ext), ";")[0]; m != "" {
		return m
	}
	return "blob"
}

// mockSizeFor returns a plausible size for a mock file.
//
// Sizes are drawn from the real size bucket for the extension rather than a
// flat 0-999 range, so the listing exercises realistic formatting: a `.txt`
// renders in bytes while a `.mp4` or `.zip` renders in MB/GB. A flat range made
// every cell show a near-identical tiny size.
func mockSizeFor(name string, r *rand.Rand) int64 {
	ext := strings.ToLower(filepath.Ext(name))
	switch ext {
	case ".jpg", ".mp3", ".doc":
		// Hundreds of KB to a few MB.
		return 50_000 + r.Int63n(4_000_000)
	case ".mp4", ".mov":
		// Tens to hundreds of MB.
		return 10_000_000 + r.Int63n(500_000_000)
	case ".zip", ".bak":
		// KB to tens of MB.
		return 1_000 + r.Int63n(20_000_000)
	default:
		// Small text-like files.
		return 10 + r.Int63n(50_000)
	}
}

// MockDataSeed derives a stable seed from the requested listing shape.
func MockDataSeed(numDirs, numFilesPerDir int) int64 {
	return int64(numDirs)*1_000_003 + int64(numFilesPerDir)
}
