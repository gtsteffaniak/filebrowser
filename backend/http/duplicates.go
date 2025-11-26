package http

import (
	"crypto/md5"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"os"
	"path/filepath"
	"sort"
	"strings"
	"sync"
	"time"
	"unicode"

	"github.com/gtsteffaniak/filebrowser/backend/common/settings"
	"github.com/gtsteffaniak/filebrowser/backend/database/sql"
	"github.com/gtsteffaniak/filebrowser/backend/indexing"
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

// findDuplicatesInIndex finds duplicates using SQLite streaming approach
// This minimizes memory allocation by:
// 1. Streaming files into a temporary SQLite database (no in-memory map)
// 2. Querying SQLite for size groups with 2+ files
// 3. Processing each size group sequentially (only one in memory at a time)
// 4. Only creating SearchResult objects for final verified duplicates
func findDuplicatesInIndex(index *indexing.Index, opts *duplicatesOptions) []duplicateGroup {

	// Create temporary SQLite database for streaming in cache directory
	tempDB, err := sql.NewTempDB("duplicates", &sql.TempDBConfig{
		CacheSizeKB:   -2000,     // ~8MB, sufficient for small-medium datasets
		MmapSize:      104857600, // 100MB - reasonable limit
		Synchronous:   "OFF",     // Maximum write performance for temp DB
		EnableLogging: false,
	})
	if err != nil {
		// Return empty results if SQLite fails
		return []duplicateGroup{}
	}
	defer tempDB.Close()

	// Create the duplicates table with indexes
	// Indexes are created before insert so they're immediately available for queries
	tableStart := time.Now()
	if err := tempDB.CreateDuplicatesTable(); err != nil {
		return []duplicateGroup{}
	}
	if time.Since(tableStart) > 10*time.Millisecond {
		logger.Debugf("[Duplicates] Table and index creation took %v", time.Since(tableStart))
	}

	// Step 1: Stream files into SQLite database
	// This avoids building a huge map in memory
	tx, err := tempDB.BeginTransaction()
	if err != nil {
		return []duplicateGroup{}
	}

	// Prepare statement for bulk inserts
	stmt, err := tx.Prepare("INSERT OR IGNORE INTO files (dir_path, file_idx, size, name, normalized_name, extension) VALUES (?, ?, ?, ?, ?, ?)")
	if err != nil {
		tx.Rollback()
		return []duplicateGroup{}
	}
	defer stmt.Close()

	insertErr := error(nil)
	index.ReadOnlyOperation(func() {
		for dirPath, dir := range index.GetDirectories() {
			// Skip directories not in scope
			if opts.combinedPath != "" && !strings.HasPrefix(dirPath, opts.combinedPath) {
				continue
			}

			for i := range dir.Files {
				file := &dir.Files[i]
				if file.Size >= opts.minSize {
					// Normalize filename for efficient matching
					normalizedName := normalizeFilename(file.Name)
					extension := strings.ToLower(filepath.Ext(file.Name))

					// Insert into SQLite (will be committed in batch)
					_, err := stmt.Exec(dirPath, i, file.Size, file.Name, normalizedName, extension)
					if err != nil && insertErr == nil {
						insertErr = err
					}
				}
			}
		}
	})

	// Commit the transaction
	if insertErr != nil || tx.Commit() != nil {
		return []duplicateGroup{}
	}

	// Step 2: Query SQLite for size groups with 2+ files (already sorted by SQL)
	sizes, _, err := tempDB.GetSizeGroupsForDuplicates(opts.minSize)
	if err != nil {
		return []duplicateGroup{}
	}
	// Step 3: Process each size group sequentially to minimize memory
	duplicateGroups := []duplicateGroup{}
	totalFileQueryTime := time.Duration(0)
	fileQueryCount := 0

	for _, size := range sizes {
		// Stop if we've hit the group limit
		if len(duplicateGroups) >= maxGroups {
			break
		}

		// Get files for this size from SQLite
		fileQueryStart := time.Now()
		locations, err := tempDB.GetFilesBySizeForDuplicates(size)
		fileQueryDuration := time.Since(fileQueryStart)
		totalFileQueryTime += fileQueryDuration
		fileQueryCount++

		if err != nil || len(locations) < 2 {
			continue
		}

		// Use filename matching for initial grouping (fast)
		// Work directly with sql.FileLocation - no conversion needed
		groups := groupLocationsByFilenameWithMetadata(locations, index, size)

		// Process candidate groups up to the limit
		for _, locGroup := range groups {
			if len(locGroup) < 2 {
				continue
			}

			// Stop if we've hit the group limit
			if len(duplicateGroups) >= maxGroups {
				break
			}

			// Verify with checksums
			verifiedGroups := groupLocationsByChecksumFromSQL(locGroup, index, size)

			// Create SearchResult objects only for verified duplicates
			for _, verifiedGroup := range verifiedGroups {
				if len(verifiedGroup) < 2 {
					continue
				}

				resultGroup := make([]*indexing.SearchResult, 0, len(verifiedGroup))
				index.ReadOnlyOperation(func() {
					dirs := index.GetDirectories()
					for _, loc := range verifiedGroup {
						dir := dirs[loc.DirPath]
						if dir == nil {
							continue
						}
						if loc.FileIdx >= len(dir.Files) {
							continue
						}
						file := &dir.Files[loc.FileIdx]

						// CRITICAL: Verify size matches the expected size for this group
						if file.Size != size {
							continue
						}

						// Construct full path
						fullPath := filepath.Join(loc.DirPath, file.Name)

						// Remove the user scope from path
						adjustedPath := strings.TrimPrefix(fullPath, opts.combinedPath)
						if adjustedPath == "" {
							adjustedPath = "/"
						}

						resultGroup = append(resultGroup, &indexing.SearchResult{
							Path:       adjustedPath,
							Type:       file.Type,
							Size:       file.Size,
							Modified:   file.ModTime.Format(time.RFC3339),
							HasPreview: file.HasPreview,
						})
					}
				})

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

	combinedPath := index.MakeIndexPath(filepath.Join(userscope, searchScope))

	return &duplicatesOptions{
		source:       source,
		searchScope:  searchScope,
		combinedPath: combinedPath,
		minSize:      minSize,
		useChecksum:  useChecksum,
	}, nil
}

// groupLocationsByChecksumFromSQL groups SQLite file locations by partial checksum
// Works directly with sql.FileLocation instead of converting to fileLocation
func groupLocationsByChecksumFromSQL(locations []sql.FileLocation, index *indexing.Index, fileSize int64) [][]sql.FileLocation {
	checksumMap := make(map[string][]sql.FileLocation)

	// Build checksum groups
	for _, loc := range locations {
		// Construct filesystem path for checksum computation
		// index.Path is the absolute filesystem root, loc.DirPath is index-relative
		fullPath := filepath.Join(index.Path, loc.DirPath)

		// Get the filename and modtime from the index and verify size still matches
		var fileName string
		var fileModTime time.Time
		var sizeMatches bool
		index.ReadOnlyOperation(func() {
			if dir := index.GetDirectories()[loc.DirPath]; dir != nil {
				if loc.FileIdx < len(dir.Files) {
					file := &dir.Files[loc.FileIdx]
					// CRITICAL: Verify size matches expected size for this group
					if file.Size == fileSize {
						fileName = file.Name
						fileModTime = file.ModTime
						sizeMatches = true
					}
				}
			}
		})

		if fileName == "" || !sizeMatches {
			continue
		}

		filePath := filepath.Join(fullPath, fileName)
		checksum, err := computePartialChecksum(index.Path, filePath, fileSize, fileModTime)
		if err != nil {
			continue
		}
		checksumMap[checksum] = append(checksumMap[checksum], loc)
	}

	// Convert map to slice of groups
	groups := make([][]sql.FileLocation, 0, len(checksumMap))
	for _, group := range checksumMap {
		if len(group) >= 2 {
			groups = append(groups, group)
		}
	}

	return groups
}

// computePartialChecksum calculates MD5 hash by sampling key portions of a file
// This is 10-100x faster than full file checksums while maintaining high accuracy
// Strategy:
// - Always read first 8KB (header/metadata)
// - For files > 24KB: sample middle 8KB and last 8KB
// - Total read: ~24KB max per file regardless of file size
// Checksums are cached for 1 hour based on source/path/modtime
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

// groupLocationsByFilenameWithMetadata uses pre-computed normalized names from SQLite
// Works directly with sql.FileLocation - no conversion needed
func groupLocationsByFilenameWithMetadata(locations []sql.FileLocation, index *indexing.Index, expectedSize int64) [][]sql.FileLocation {
	if len(locations) == 0 {
		return nil
	}

	// Verify sizes from index (still need to check index is up to date)
	type fileMetadata struct {
		name string
		size int64
	}
	metadata := make([]fileMetadata, len(locations))

	index.ReadOnlyOperation(func() {
		dirs := index.GetDirectories()
		for i, loc := range locations {
			if dir := dirs[loc.DirPath]; dir != nil {
				if loc.FileIdx < len(dir.Files) {
					file := &dir.Files[loc.FileIdx]
					if file.Size == expectedSize {
						metadata[i] = fileMetadata{
							name: file.Name,
							size: file.Size,
						}
					}
				}
			}
		}
	})

	groups := [][]sql.FileLocation{}
	used := make(map[int]bool)

	for i := 0; i < len(locations); i++ {
		if used[i] || metadata[i].name == "" {
			continue
		}

		group := []sql.FileLocation{locations[i]}
		used[i] = true
		baseSize := metadata[i].size

		// Use pre-computed normalized name and extension from SQLite
		filename1 := locations[i].NormalizedName
		ext1 := locations[i].Extension

		for j := i + 1; j < len(locations); j++ {
			if used[j] || metadata[j].name == "" {
				continue
			}

			if metadata[j].size != baseSize {
				continue
			}

			ext2 := locations[j].Extension
			if ext1 != ext2 {
				continue
			}

			filename2 := locations[j].NormalizedName

			// Check if filenames are similar enough
			if filenamesSimilar(filename1, filename2) {
				group = append(group, locations[j])
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
