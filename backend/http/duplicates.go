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
	"sync"
	"time"
	"unicode"

	"github.com/gtsteffaniak/filebrowser/backend/common/settings"
	"github.com/gtsteffaniak/filebrowser/backend/common/utils"
	"github.com/gtsteffaniak/filebrowser/backend/indexing"
	"github.com/gtsteffaniak/filebrowser/backend/indexing/iteminfo"
	"github.com/gtsteffaniak/go-cache/cache"
	"github.com/gtsteffaniak/go-logger/logger"
)

// duplicateSearchMutex serializes duplicate searches to run one at a time
// This is separate from the index's scanMutex to avoid conflicts with indexing
var duplicateSearchMutex sync.Mutex

const maxGroups = 500 // Limit total duplicate groups

// duplicateResultsCache caches duplicate search results for 15 seconds
var duplicateResultsCache = cache.NewCache[[]duplicateGroup](15 * time.Second)

// checksumCache caches file checksums for 1 hour, keyed by source/path/modtime
var checksumCache = cache.NewCache[string](1 * time.Hour)

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
	username     string
}

// duplicatesHandler handles requests to find duplicate files
//
// This endpoint finds files with the same size, uses fuzzy filename matching
// for initial grouping (fast), then verifies all final groups with checksums.
//
// It's optimized to handle large directories efficiently by:
// 1. Filtering by minimum size first
// 2. Grouping by size
// 3. Initial grouping with fuzzy filename similarity (fast)
// 4. Final verification with checksums on all groups (accurate)
//
// @Summary Find Duplicate Files
// @Description Finds duplicate files based on size and fuzzy filename matching
// @Tags Duplicates
// @Accept json
// @Produce json
// @Param source query string true "Source name for the desired source"
// @Param scope query string false "path within user scope to search"
// @Param minSizeMb query int false "Minimum file size in megabytes (default: 1)"
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
	userscope, err := settings.GetScopeFromSourceName(d.user.Scopes, index.Name)
	if err != nil {
		return http.StatusForbidden, err
	}
	userscope = strings.TrimRight(userscope, "/")
	scopePath := utils.JoinPathAsUnix(userscope, opts.searchScope)
	fullPath := index.MakeIndexPath(scopePath, true)
	if !store.Access.Permitted(index.Path, fullPath, d.user.Username) {
		return http.StatusForbidden, fmt.Errorf("user is not allowed to access this location")
	}

	// Generate cache key from all input parameters that affect results
	// Checksums are always enabled, so cache key doesn't need to include that flag
	cacheKey := fmt.Sprintf("%s:%s:%d", index.Path, opts.combinedPath, opts.minSize)

	// Check cache first (before acquiring mutex)
	if cachedResults, ok := duplicateResultsCache.Get(cacheKey); ok {
		return renderJSON(w, r, cachedResults)
	}

	// Reject concurrent requests
	if !duplicateSearchMutex.TryLock() {
		return http.StatusServiceUnavailable, fmt.Errorf("another duplicate search is currently running, please try again in a moment")
	}
	defer duplicateSearchMutex.Unlock()

	// Check cache again after acquiring lock (another request might have just completed)
	if cachedResults, ok := duplicateResultsCache.Get(cacheKey); ok {
		return renderJSON(w, r, cachedResults)
	}

	// Find duplicates using index-native approach to minimize memory allocation
	// This avoids creating SearchResult objects until we know the final limited set
	duplicateGroups := findDuplicatesInIndex(index, opts)

	// Cache the results before returning
	duplicateResultsCache.Set(cacheKey, duplicateGroups)

	return renderJSON(w, r, duplicateGroups)
}

// findDuplicatesInIndex finds duplicates using the shared IndexDB
func findDuplicatesInIndex(index *indexing.Index, opts *duplicatesOptions) []duplicateGroup {
	// Get the shared IndexDB
	indexDB := indexing.GetIndexDB()
	if indexDB == nil {
		logger.Errorf("[Duplicates] Index DB not available")
		return []duplicateGroup{}
	}

	// Step 1: Query IndexDB for size groups with 2+ files (already sorted by SQL)
	// Pass the scope prefix for efficient filtering
	pathPrefix := opts.combinedPath
	sizes, _, err := indexDB.GetSizeGroupsForDuplicates(opts.source, opts.minSize, pathPrefix)
	if err != nil {
		logger.Errorf("[Duplicates] Failed to query size groups: %v", err)
		return []duplicateGroup{}
	}

	// Step 2: Process each size group sequentially to minimize memory
	duplicateGroups := []duplicateGroup{}
	totalFileQueryTime := time.Duration(0)
	fileQueryCount := 0

	for _, size := range sizes {
		// Stop if we've hit the group limit
		if len(duplicateGroups) >= maxGroups {
			break
		}

		// Get files for this size from IndexDB
		fileQueryStart := time.Now()
		files, err := indexDB.GetFilesBySize(opts.source, size, pathPrefix)
		fileQueryDuration := time.Since(fileQueryStart)
		totalFileQueryTime += fileQueryDuration
		fileQueryCount++

		if err != nil || len(files) < 2 {
			continue
		}

		// Filter files by permission early, before any processing
		// This ensures only files the user is permitted to access are included
		files = filterFilesByPermission(files, index, opts.username)
		if len(files) < 2 {
			continue
		}

		// Use filename matching for initial grouping (fast)
		groups := groupFilesByFilename(files, size)

		// Process candidate groups up to the limit
		for _, fileGroup := range groups {
			if len(fileGroup) < 2 {
				continue
			}

			// Stop if we've hit the group limit
			if len(duplicateGroups) >= maxGroups {
				break
			}

			// Verify with checksums
			verifiedGroups := groupFilesByChecksum(fileGroup, index, size)

			// Create SearchResult objects only for verified duplicates
			for _, verifiedGroup := range verifiedGroups {
				if len(verifiedGroup) < 2 {
					continue
				}

				resultGroup := make([]*indexing.SearchResult, 0, len(verifiedGroup))
				for _, fileInfo := range verifiedGroup {
					// Remove the user scope from path
					adjustedPath := strings.TrimPrefix(fileInfo.Path, opts.combinedPath)
					if adjustedPath == "" {
						adjustedPath = "/"
					}

					resultGroup = append(resultGroup, &indexing.SearchResult{
						Path:       adjustedPath,
						Type:       fileInfo.Type,
						Size:       fileInfo.Size,
						Modified:   fileInfo.ModTime.Format(time.RFC3339),
						HasPreview: fileInfo.HasPreview,
					})
				}

				if len(resultGroup) >= 2 {
					duplicateGroups = append(duplicateGroups, duplicateGroup{
						Size:  size,
						Count: len(resultGroup),
						Files: resultGroup,
					})
				}
			}

			// Stop if we've hit the group limit
			if len(duplicateGroups) >= maxGroups {
				break
			}
		}
	}

	// Log aggregate query performance
	if fileQueryCount > 0 {
		avgFileQueryTime := totalFileQueryTime / time.Duration(fileQueryCount)
		logger.Debugf("[Duplicates] File-by-size queries: %d queries, total %v, avg %v per query",
			fileQueryCount, totalFileQueryTime, avgFileQueryTime)
	}

	// Groups are already sorted by size (largest to smallest) from SQL query
	return duplicateGroups
}

func prepDuplicatesOptions(r *http.Request, d *requestContext) (*duplicatesOptions, error) {
	source := r.URL.Query().Get("source")
	scope := r.URL.Query().Get("scope")

	minSizeMbStr := r.URL.Query().Get("minSizeMb")
	// Checksums are always enabled by default for final verification
	// First pass uses filename matching for speed, then checksums verify final groups
	useChecksum := true

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

	combinedPath := index.MakeIndexPath(filepath.Join(userscope, searchScope), true) // searchScope is a directory

	return &duplicatesOptions{
		source:       source,
		searchScope:  searchScope,
		combinedPath: combinedPath,
		minSize:      minSize,
		useChecksum:  useChecksum,
		username:     d.user.Username,
	}, nil
}

// groupFilesByChecksum groups files by partial checksum
// Works with iteminfo.FileInfo from the main IndexDB
func groupFilesByChecksum(files []*iteminfo.FileInfo, index *indexing.Index, fileSize int64) [][]*iteminfo.FileInfo {
	checksumMap := make(map[string][]*iteminfo.FileInfo)

	// Build checksum groups
	for _, file := range files {
		// Verify size matches expected size (should match as we queried by size)
		if file.Size != fileSize {
			continue
		}

		// Construct filesystem path for checksum computation
		// index.Path is the absolute filesystem root, file.Path is index-relative
		filePath := filepath.Join(index.Path, file.Path)
		checksum, err := computePartialChecksum(index.Path, filePath, fileSize, file.ModTime)
		if err != nil {
			continue
		}
		checksumMap[checksum] = append(checksumMap[checksum], file)
	}

	// Convert map to slice of groups
	groups := make([][]*iteminfo.FileInfo, 0, len(checksumMap))
	for _, group := range checksumMap {
		if len(group) >= 2 {
			groups = append(groups, group)
		}
	}

	return groups
}

// computePartialChecksum calculates MD5 hash by sampling key portions of a file
// - Always read first 8KB (header/metadata)
// - For files > 24KB: sample middle 8KB and last 8KB
// - Total read: ~24KB max per file regardless of file size
func computePartialChecksum(sourcePath, filePath string, size int64, modTime time.Time) (string, error) {
	// Generate cache key from source path, file path, and modification time
	cacheKey := fmt.Sprintf("%s:%s:%d:%d", sourcePath, filePath, size, modTime.Unix())

	// Check cache first
	if cachedChecksum, ok := checksumCache.Get(cacheKey); ok {
		return cachedChecksum, nil
	}

	file, err := os.Open(filePath)
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

	checksum := fmt.Sprintf("%x", hash.Sum(nil))

	// Cache the result
	checksumCache.Set(cacheKey, checksum)

	return checksum, nil
}

// groupFilesByFilename groups files by fuzzy filename matching
// Works with iteminfo.FileInfo from the main IndexDB
// Normalizes filenames on-the-fly for comparison
func groupFilesByFilename(files []*iteminfo.FileInfo, expectedSize int64) [][]*iteminfo.FileInfo {
	if len(files) == 0 {
		return nil
	}

	groups := [][]*iteminfo.FileInfo{}
	used := make(map[int]bool)

	for i := 0; i < len(files); i++ {
		if used[i] || files[i].Name == "" {
			continue
		}

		group := []*iteminfo.FileInfo{files[i]}
		used[i] = true

		// Normalize filename and extract extension on-the-fly
		filename1 := normalizeFilename(files[i].Name)
		ext1 := strings.ToLower(filepath.Ext(files[i].Name))

		for j := i + 1; j < len(files); j++ {
			if used[j] || files[j].Name == "" {
				continue
			}

			// Check size (should match, but safe to check)
			if files[j].Size != expectedSize {
				continue
			}

			ext2 := strings.ToLower(filepath.Ext(files[j].Name))
			if ext1 != ext2 {
				continue
			}

			filename2 := normalizeFilename(files[j].Name)

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

// filterFilesByPermission filters files to only include those the user is permitted to access
// This is called early in the duplicate search process to avoid processing files the user can't see
func filterFilesByPermission(files []*iteminfo.FileInfo, index *indexing.Index, username string) []*iteminfo.FileInfo {
	if store.Access == nil {
		// No access control configured, return all files
		return files
	}

	filtered := make([]*iteminfo.FileInfo, 0, len(files))
	for _, file := range files {
		// Check permission using index.Path (source root) and file.Path (index-relative path)
		if store.Access.Permitted(index.Path, file.Path, username) {
			filtered = append(filtered, file)
		}
	}
	return filtered
}
