package http

import (
	"crypto/md5"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"os"
	"path/filepath"
	"strings"
	"unicode"

	"github.com/gtsteffaniak/filebrowser/backend/common/settings"
	"github.com/gtsteffaniak/filebrowser/backend/indexing"
)

type duplicateGroup struct {
	Size  int64                    `json:"size"`
	Count int                      `json:"count"`
	Files []*indexing.SearchResult `json:"files"`
}

type duplicatesOptions struct {
	source       string
	searchScope  string
	combinedPath string
	minSize      int64
	useChecksum  bool
}

// duplicatesHandler handles requests to find duplicate files
//
// This endpoint finds files with the same size and uses different verification methods:
// - useChecksum=false: Fuzzy filename matching (fast, good for renamed duplicates)
// - useChecksum=true: Partial content checksums (very fast, high accuracy)
//
// It's optimized to handle large directories efficiently by:
// 1. Filtering by minimum size first
// 2. Grouping by size
// 3. Verifying matches with either filename similarity or partial checksums
//
// Partial checksums sample ~24KB per file (start, middle, end) regardless of file size,
// making it 10-100x faster than full file hashing while maintaining high accuracy.
//
// @Summary Find Duplicate Files
// @Description Finds duplicate files based on size and verification method (fuzzy filename or partial checksum)
// @Tags Duplicates
// @Accept json
// @Produce json
// @Param source query string true "Source name for the desired source"
// @Param scope query string false "path within user scope to search"
// @Param minSizeMb query int false "Minimum file size in megabytes (default: 1)"
// @Param useChecksum query bool false "Use partial MD5 checksum for verification (default: false, uses fuzzy filename matching)"
// @Success 200 {array} duplicateGroup "List of duplicate file groups"
// @Failure 400 {object} map[string]string "Bad Request"
// @Router /api/duplicates [get]
func duplicatesHandler(w http.ResponseWriter, r *http.Request, d *requestContext) (int, error) {
	opts, err := prepDuplicatesOptions(r, d)
	if err != nil {
		return http.StatusBadRequest, err
	}

	index := indexing.GetIndex(opts.source)
	if index == nil {
		return http.StatusBadRequest, fmt.Errorf("index not found for source %s", opts.source)
	}

	// Get all files using largest=true to bypass text search
	// We'll filter by minSize ourselves
	query := fmt.Sprintf("type:largerThan=%d type:file", opts.minSize/(1024*1024))

	// Use largest=true to get files sorted by size, which bypasses the need for text matching
	allFiles := index.Search(query, opts.combinedPath, "", true)

	// Group files by size first (efficient first pass)
	// Using pointers reduces memory usage significantly when dealing with many files
	sizeGroups := make(map[int64][]*indexing.SearchResult)
	for _, file := range allFiles {
		if file.Size >= opts.minSize {
			sizeGroups[file.Size] = append(sizeGroups[file.Size], file)
		}
	}

	// Find duplicates
	duplicateGroups := []duplicateGroup{}

	for size, files := range sizeGroups {
		if len(files) < 2 {
			continue // Skip files with no duplicates
		}

		if opts.useChecksum {
			// Pass the index to get its root path for filesystem access
			checksumGroups := groupByPartialChecksum(files, index, size)
			for _, group := range checksumGroups {
				if len(group) >= 2 {
					// Remove the user scope from paths (modifying in place is safe)
					for _, file := range group {
						file.Path = strings.TrimPrefix(file.Path, opts.combinedPath)
						if file.Path == "" {
							file.Path = "/"
						}
					}
					duplicateGroups = append(duplicateGroups, duplicateGroup{
						Size:  size,
						Count: len(group),
						Files: group,
					})
				}
			}
		} else {
			// Use fuzzy filename matching (faster than checksums, more accurate than size-only)
			filenameGroups := groupByFuzzyFilename(files, opts.combinedPath)
			for _, group := range filenameGroups {
				if len(group) >= 2 {
					// Remove the user scope from paths (modifying in place is safe)
					for _, file := range group {
						file.Path = strings.TrimPrefix(file.Path, opts.combinedPath)
						if file.Path == "" {
							file.Path = "/"
						}
					}
					duplicateGroups = append(duplicateGroups, duplicateGroup{
						Size:  size,
						Count: len(group),
						Files: group,
					})
				}
			}
		}
	}

	return renderJSON(w, r, duplicateGroups)
}

func prepDuplicatesOptions(r *http.Request, d *requestContext) (*duplicatesOptions, error) {
	source := r.URL.Query().Get("source")
	scope := r.URL.Query().Get("scope")
	minSizeMbStr := r.URL.Query().Get("minSizeMb")
	useChecksum := r.URL.Query().Get("useChecksum") == "true"

	// Default minimum size: 1MB
	minSizeMb := int64(1)
	if minSizeMbStr != "" {
		if _, err := fmt.Sscanf(minSizeMbStr, "%d", &minSizeMb); err != nil {
			return nil, fmt.Errorf("invalid minSizeMb parameter: %w", err)
		}
	}
	// Convert MB to bytes
	minSize := minSizeMb * 1024 * 1024

	unencodedScope, err := url.PathUnescape(scope)
	if err != nil {
		return nil, fmt.Errorf("invalid path encoding: %v", err)
	}

	searchScope := strings.TrimPrefix(unencodedScope, ".")

	index := indexing.GetIndex(source)
	if index == nil {
		return nil, fmt.Errorf("index not found for source %s", source)
	}

	userscope, err := settings.GetScopeFromSourceName(d.user.Scopes, source)
	if err != nil {
		return nil, err
	}

	combinedPath := index.MakeIndexPath(filepath.Join(userscope, searchScope))

	return &duplicatesOptions{
		source:       source,
		searchScope:  searchScope,
		combinedPath: combinedPath,
		minSize:      minSize,
		useChecksum:  useChecksum,
	}, nil
}

// groupByPartialChecksum computes partial MD5 checksums by sampling key portions of files
// This is much faster than full checksums while providing high accuracy
// Uses pointers to avoid copying large structs during grouping
func groupByPartialChecksum(files []*indexing.SearchResult, index *indexing.Index, fileSize int64) map[string][]*indexing.SearchResult {
	checksumGroups := make(map[string][]*indexing.SearchResult)

	for _, file := range files {
		// file.Path is relative to the index root, need to prepend index.Path
		// index.Path is the absolute filesystem root for this index
		fullPath := filepath.Join(index.Path, file.Path)
		checksum, err := computePartialChecksum(fullPath, fileSize)
		if err != nil {
			continue
		}
		checksumGroups[checksum] = append(checksumGroups[checksum], file)
	}

	return checksumGroups
}

// computePartialChecksum calculates MD5 hash by sampling key portions of a file
// This is 10-100x faster than full file checksums while maintaining high accuracy
// Strategy:
// - Always read first 8KB (header/metadata)
// - For files > 24KB: sample middle 8KB and last 8KB
// - Total read: ~24KB max per file regardless of file size
func computePartialChecksum(path string, size int64) (string, error) {
	file, err := os.Open(path)
	if err != nil {
		return "", err
	}
	defer file.Close()

	hash := md5.New()
	buf := make([]byte, 8192) // 8KB buffer

	// Always read first 8KB (or entire file if smaller)
	n, err := io.ReadFull(file, buf)
	if err != nil && err != io.EOF && err != io.ErrUnexpectedEOF {
		return "", err
	}
	hash.Write(buf[:n])

	// For larger files, sample middle and end portions
	if size > 24576 { // 24KB
		// Sample middle 8KB
		middleOffset := size / 2
		if _, err := file.Seek(middleOffset, 0); err == nil {
			n, err := io.ReadFull(file, buf)
			if err != nil && err != io.EOF && err != io.ErrUnexpectedEOF {
				return "", err
			}
			hash.Write(buf[:n])
		}

		// Sample last 8KB
		endOffset := size - 8192
		if endOffset > 8192 { // Don't re-read start
			if _, err := file.Seek(endOffset, 0); err == nil {
				n, err := io.ReadFull(file, buf)
				if err != nil && err != io.EOF && err != io.ErrUnexpectedEOF {
					return "", err
				}
				hash.Write(buf[:n])
			}
		}
	}

	return fmt.Sprintf("%x", hash.Sum(nil)), nil
}

// groupByFuzzyFilename groups files with similar filenames
// Uses fuzzy matching to avoid false positives from size-only matching
// Uses pointers to avoid copying large structs during grouping
func groupByFuzzyFilename(files []*indexing.SearchResult, basePath string) [][]*indexing.SearchResult {
	if len(files) == 0 {
		return nil
	}

	groups := [][]*indexing.SearchResult{}
	used := make(map[int]bool)

	// Compare each file with every other file
	for i := 0; i < len(files); i++ {
		if used[i] {
			continue
		}

		group := []*indexing.SearchResult{files[i]}
		used[i] = true

		filename1 := normalizeFilename(filepath.Base(files[i].Path))

		for j := i + 1; j < len(files); j++ {
			if used[j] {
				continue
			}

			filename2 := normalizeFilename(filepath.Base(files[j].Path))

			// Check if filenames are similar enough
			if filenamesSimilar(filename1, filename2) {
				group = append(group, files[j])
				used[j] = true
			}
		}

		if len(group) >= 2 {
			groups = append(groups, group)
		}
	}

	return groups
}

// normalizeFilename converts filename to lowercase and removes special characters
// to make fuzzy matching more effective
func normalizeFilename(filename string) string {
	// Convert to lowercase
	filename = strings.ToLower(filename)

	// Remove file extension for comparison
	if idx := strings.LastIndex(filename, "."); idx > 0 {
		filename = filename[:idx]
	}

	// Remove non-alphanumeric characters except spaces and hyphens
	var result strings.Builder
	for _, r := range filename {
		if unicode.IsLetter(r) || unicode.IsDigit(r) || r == ' ' || r == '-' {
			result.WriteRune(r)
		}
	}

	return strings.TrimSpace(result.String())
}

// filenamesSimilar checks if two normalized filenames are similar enough to be considered duplicates
func filenamesSimilar(name1, name2 string) bool {
	// Exact match
	if name1 == name2 {
		return true
	}

	// Empty names don't match
	if len(name1) == 0 || len(name2) == 0 {
		return false
	}

	// Calculate Levenshtein distance
	distance := levenshteinDistance(name1, name2)
	maxLen := max(len(name1), len(name2))

	// Require at least 70% similarity
	similarity := 1.0 - float64(distance)/float64(maxLen)
	return similarity >= 0.7
}

// levenshteinDistance calculates the edit distance between two strings
func levenshteinDistance(s1, s2 string) int {
	if len(s1) == 0 {
		return len(s2)
	}
	if len(s2) == 0 {
		return len(s1)
	}

	// Create matrix
	matrix := make([][]int, len(s1)+1)
	for i := range matrix {
		matrix[i] = make([]int, len(s2)+1)
	}

	// Initialize first row and column
	for i := 0; i <= len(s1); i++ {
		matrix[i][0] = i
	}
	for j := 0; j <= len(s2); j++ {
		matrix[0][j] = j
	}

	// Fill matrix
	for i := 1; i <= len(s1); i++ {
		for j := 1; j <= len(s2); j++ {
			cost := 0
			if s1[i-1] != s2[j-1] {
				cost = 1
			}

			matrix[i][j] = min(
				matrix[i-1][j]+1,      // deletion
				matrix[i][j-1]+1,      // insertion
				matrix[i-1][j-1]+cost, // substitution
			)
		}
	}

	return matrix[len(s1)][len(s2)]
}

// min returns the minimum of three integers
func min(a, b, c int) int {
	if a < b {
		if a < c {
			return a
		}
		return c
	}
	if b < c {
		return b
	}
	return c
}

// max returns the maximum of two integers
func max(a, b int) int {
	if a > b {
		return a
	}
	return b
}
