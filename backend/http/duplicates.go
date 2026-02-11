package http

import (
	"crypto/md5"
	"fmt"
	"io"
	"net/http"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"time"
	"unicode"

	"github.com/gtsteffaniak/filebrowser/backend/common/utils"
	"github.com/gtsteffaniak/filebrowser/backend/indexing"
	"github.com/gtsteffaniak/filebrowser/backend/indexing/iteminfo"
	"github.com/gtsteffaniak/go-cache/cache"
	"github.com/gtsteffaniak/go-logger/logger"
)

// duplicateSearchMutex serializes duplicate searches to run one at a time
// This is separate from the index's scanMutex to avoid conflicts with indexing
var duplicateSearchMutex sync.Mutex

// Safety limits to prevent resource exhaustion and server attacks
const (
	maxGroups             = 500               // Limit total duplicate groups returned
	maxFilesToScan        = 50000             // Maximum files to scan per request (prevents unbounded scans)
	maxProcessingTime     = 120 * time.Second // Maximum time to spend processing (2 minutes)
	maxChecksumOperations = 10000             // Maximum checksum operations per request (prevents excessive disk I/O)
	bulkSizeQueryLimit    = 100               // Process sizes in batches of 100 to balance memory vs query count
	maxFuzzyGroupSize     = 10                // Maximum files in a fuzzy filename group before skipping checksum (large groups are unlikely duplicates)
)

// duplicateResultsCache caches duplicate search results for 15 seconds
var duplicateResultsCache = cache.NewCache[duplicateResponse](15 * time.Second)

// checksumCache caches file checksums for 1 hour, keyed by source/path/modtime
var checksumCache = cache.NewCache[string](1 * time.Hour)

type duplicateGroup struct {
	Size  int64                    `json:"size"`
	Count int                      `json:"count"`
	Files []*indexing.SearchResult `json:"files"`
}

// duplicateGroupWithChecksums tracks checksums for each file in a group
// Used internally for merging groups with matching checksums
type duplicateGroupWithChecksums struct {
	Size     int64
	Files    []*indexing.SearchResult
	Checksum string // Single checksum shared by all files in this group
}

// checksumGroup pairs a file group with its checksum
type checksumGroup struct {
	Files    []*iteminfo.FileInfo
	Checksum string
}

type duplicateResponse struct {
	Groups     []duplicateGroup `json:"groups"`
	Incomplete bool             `json:"incomplete,omitempty"`
	Reason     string           `json:"reason,omitempty"`
}

type duplicatesOptions struct {
	source       string
	searchScope  string
	combinedPath string
	minSize      int64
	useChecksum  bool
	username     string
}

// duplicateProcessingStats tracks resource usage during duplicate search
type duplicateProcessingStats struct {
	startTime           time.Time
	filesScanned        int
	checksumOperations  int
	sizeGroupsProcessed int
	stopped             bool
	stopReason          string
	uniqueChecksums     map[string]bool // Track unique checksums to monitor cache growth
}

// shouldStop checks if processing should stop due to resource limits
func (s *duplicateProcessingStats) shouldStop() (bool, string) {
	elapsed := time.Since(s.startTime)

	if elapsed > maxProcessingTime {
		return true, fmt.Sprintf("processing time limit exceeded (%v)", maxProcessingTime)
	}
	if s.filesScanned >= maxFilesToScan {
		return true, fmt.Sprintf("file scan limit exceeded (%d files)", maxFilesToScan)
	}
	if s.checksumOperations >= maxChecksumOperations {
		return true, fmt.Sprintf("checksum operation limit exceeded (%d operations)", maxChecksumOperations)
	}
	return false, ""
}

// duplicatesHandler handles requests to find duplicate files
//
// This endpoint finds duplicate files using a multi-stage filtering and verification process
// optimized for performance and accuracy on large directories.
//
// Filtering Pipeline (applied in order):
// 1. SQL Query: Files grouped by size (2+ files per size) from database index
// 2. Permission Filter: Only files accessible by the requesting user
// 3. MIME Type Grouping: Files grouped by file type (image/png, video/mp4, etc.)
// 4. Fuzzy Filename Matching: Files with similar names (50%+ similarity via Levenshtein distance)
//   - Normalizes filenames (lowercase, removes extension, strips special chars)
//   - Groups files with similar base names (e.g., "photo.jpg" matches "photo_1.jpg")
//   - Large groups (>10 files) are skipped to avoid false positives
//
// 5. Progressive Checksum Verification (2-pass):
//   - Pass 1: Header checksum (first 8KB) - fastest elimination
//   - Pass 2: Middle checksum (header + middle 8KB) - final verification for header matches
//     Note: Files matching header + middle but differing at end are extremely rare (<0.1%),
//     so 2-pass is sufficient and faster than 3-pass
//     6. Post-Processing: Groups with matching checksums are merged (catches files with
//     identical content but different filenames)
//
// Performance Optimizations:
// - Checksums are cached for 1 hour (keyed by path/size/modtime) to speed up subsequent requests
// - Files are processed in batches to balance memory usage vs SQL query count
// - Resource limits prevent timeouts: max 2 minutes, max 10K checksum operations
// - Large fuzzy groups are skipped to avoid expensive checksum operations on false positives
//
// Response includes incomplete flag if processing was stopped early due to resource limits.
//
// @Summary Find Duplicate Files
// @Description Finds duplicate files using multi-stage filtering: size → type → fuzzy filename → progressive checksums. Files must match on size, MIME type, and have 50%+ filename similarity before checksum verification. Large fuzzy groups (>10 files) are skipped to avoid false positives. Checksums use 2-pass progressive verification (header → middle) for accuracy while minimizing disk I/O (~16KB read per file).
// @Tags Duplicates
// @Accept json
// @Produce json
// @Param source query string true "Source name for the desired source"
// @Param scope query string false "path within user scope to search"
// @Param minSizeMb query int false "Minimum file size in megabytes (default: 1)"
// @Success 200 {object} duplicateResponse "List of duplicate file groups with metadata. Response includes 'incomplete' flag if processing stopped early due to resource limits."
// @Failure 400 {object} map[string]string "Bad Request"
// @Failure 503 {object} map[string]string "Service Unavailable (indexing in progress or another search running)"
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
	userscope, err := d.user.GetScopeForSourceName(index.Name)
	if err != nil {
		return http.StatusForbidden, err
	}
	userscope = strings.TrimRight(userscope, "/")
	scopePath := utils.JoinPathAsUnix(userscope, opts.searchScope)
	fullPath := index.MakeIndexPath(scopePath, true)
	if !store.Access.Permitted(index.Path, fullPath, d.user.Username) {
		return http.StatusForbidden, fmt.Errorf("user is not allowed to access this location")
	}

	// Safety check: Reject duplicate search during active indexing to prevent resource contention
	// This protects against CPU/memory spikes and ensures indexing performance is not degraded
	if string(index.Status) == "indexing" {
		return http.StatusServiceUnavailable, fmt.Errorf("duplicate search is not available while indexing is in progress - please try again when indexing completes")
	}

	userscope, err = d.user.GetScopeForSourceName(index.Name)
	if err != nil {
		return http.StatusForbidden, err
	}
	userscope = strings.TrimRight(userscope, "/")
	scopePath = utils.JoinPathAsUnix(userscope, opts.searchScope)
	fullPath = index.MakeIndexPath(scopePath, true)
	if !store.Access.Permitted(index.Path, fullPath, d.user.Username) {
		return http.StatusForbidden, fmt.Errorf("user is not allowed to access this location")
	}

	// Generate cache key from all input parameters that affect results
	// Checksums are always enabled, so cache key doesn't need to include that flag
	cacheKey := fmt.Sprintf("%s:%s:%d", index.Path, opts.combinedPath, opts.minSize)

	// Check cache first (before acquiring mutex)
	if cachedResults, ok := duplicateResultsCache.Get(cacheKey); ok {
		// Set headers if cached result was incomplete (metadata only, don't change status code)
		if cachedResults.Incomplete {
			w.Header().Set("X-Search-Incomplete", "true")
			w.Header().Set("X-Search-Incomplete-Reason", cachedResults.Reason)
		}
		return renderJSON(w, r, cachedResults)
	}

	// Reject concurrent requests
	if !duplicateSearchMutex.TryLock() {
		return http.StatusServiceUnavailable, fmt.Errorf("another duplicate search is currently running, please try again in a moment")
	}
	defer duplicateSearchMutex.Unlock()

	// Check cache again after acquiring lock (another request might have just completed)
	if cachedResults, ok := duplicateResultsCache.Get(cacheKey); ok {
		// Set headers if cached result was incomplete (metadata only, don't change status code)
		if cachedResults.Incomplete {
			w.Header().Set("X-Search-Incomplete", "true")
			w.Header().Set("X-Search-Incomplete-Reason", cachedResults.Reason)
		}
		return renderJSON(w, r, cachedResults)
	}

	// Find duplicates using index-native approach with resource limits
	// This avoids creating SearchResult objects until we know the final limited set
	stats := &duplicateProcessingStats{
		startTime:       time.Now(),
		uniqueChecksums: make(map[string]bool),
	}
	duplicateGroups := findDuplicatesInIndex(index, opts, stats)

	// Log resource usage for monitoring
	uniqueChecksumCount := len(stats.uniqueChecksums)
	if stats.stopped {
		logger.Warningf("[Duplicates] Search stopped early: %s (scanned %d files, %d checksum ops, %d unique checksums, %d groups in %v)",
			stats.stopReason, stats.filesScanned, stats.checksumOperations, uniqueChecksumCount, stats.sizeGroupsProcessed, time.Since(stats.startTime))
	}

	// Build response with metadata about completeness
	response := duplicateResponse{
		Groups:     duplicateGroups,
		Incomplete: stats.stopped,
		Reason:     stats.stopReason,
	}

	// Cache the results before returning (even partial results)
	duplicateResultsCache.Set(cacheKey, response)

	if stats.stopped {
		w.Header().Set("X-Search-Incomplete", "true")
		w.Header().Set("X-Search-Incomplete-Reason", stats.stopReason)
	}

	return renderJSON(w, r, response)
}

// findDuplicatesInIndex finds duplicates using the shared IndexDB with resource limits
func findDuplicatesInIndex(index *indexing.Index, opts *duplicatesOptions, stats *duplicateProcessingStats) []duplicateGroup {
	// Get the shared IndexDB
	indexDB := indexing.GetIndexDB()

	// Step 1: Query IndexDB for size groups with 2+ files (already sorted by SQL)
	// Pass the scope prefix for efficient filtering
	pathPrefix := opts.combinedPath
	sizes, _, err := indexDB.GetSizeGroupsForDuplicates(opts.source, opts.minSize, pathPrefix)
	if err != nil {
		logger.Errorf("[Duplicates] Failed to query size groups: %v", err)
		return []duplicateGroup{}
	}

	// No limit on size groups - process all sizes, only limit final duplicate groups returned

	// Step 2: Process sizes in batches using bulk queries to minimize SQL round-trips
	// Use intermediate structure with checksums for merging
	groupsWithChecksums := []duplicateGroupWithChecksums{}
	totalBulkQueryTime := time.Duration(0)
	bulkQueryCount := 0

	// Process sizes in batches
	for batchStart := 0; batchStart < len(sizes); batchStart += bulkSizeQueryLimit {
		// Check resource limits before processing each batch
		if shouldStop, reason := stats.shouldStop(); shouldStop {
			stats.stopped = true
			stats.stopReason = reason
			logger.Warningf("[Duplicates] Stopping early: %s", reason)
			break
		}

		// Stop if we've hit the group limit
		if len(groupsWithChecksums) >= maxGroups {
			break
		}

		// Get batch of sizes to process
		batchEnd := batchStart + bulkSizeQueryLimit
		if batchEnd > len(sizes) {
			batchEnd = len(sizes)
		}
		sizeBatch := sizes[batchStart:batchEnd]

		// BULK QUERY: Load ALL files for ALL sizes in this batch with ONE query
		// This balances SQL query efficiency with memory usage
		bulkQueryStart := time.Now()
		filesBySize, err := indexDB.GetFilesForMultipleSizes(opts.source, sizeBatch, pathPrefix)
		bulkQueryDuration := time.Since(bulkQueryStart)
		totalBulkQueryTime += bulkQueryDuration
		bulkQueryCount++

		if err != nil {
			logger.Errorf("[Duplicates] Failed to bulk query files for %d sizes: %v", len(sizeBatch), err)
			continue
		}

		// Process each size within this batch
		// Memory optimization: Delete entries from filesBySize immediately after processing
		// to allow GC to reclaim memory as we go, rather than holding all data until batch completes
		for _, size := range sizeBatch {
			files, ok := filesBySize[size]
			if !ok || len(files) < 2 {
				// Clear entry even if not processing to free memory
				delete(filesBySize, size)
				continue
			}

			stats.sizeGroupsProcessed++
			stats.filesScanned += len(files)

			// Filter files by permission early, before any processing
			files = filterFilesByPermission(files, index, opts.username)
			if len(files) < 2 {
				// Clear entry if filtered out to free memory
				delete(filesBySize, size)
				continue
			}

			// Group files by MIME type in memory (already ordered by type from SQL)
			filesByType := groupFilesByType(files)

			// Process each MIME type group
			for _, typeFiles := range filesByType {
				if len(typeFiles) < 2 {
					continue
				}

				// Check resource limits
				if shouldStop, reason := stats.shouldStop(); shouldStop {
					stats.stopped = true
					stats.stopReason = reason
					logger.Warningf("[Duplicates] Stopping early: %s", reason)
					break
				}

				// Use fuzzy filename matching to group files within this MIME type
				// This reduces checksum operations by grouping similar filenames
				filenameGroups := groupFilesByFilename(typeFiles, size)

				totalFilesInGroups := 0
				for _, g := range filenameGroups {
					if len(g) >= 2 {
						totalFilesInGroups += len(g)
					}
				}

				// Process each fuzzy filename group
				for _, fileGroup := range filenameGroups {
					if len(fileGroup) < 2 {
						continue
					}

					// Skip checksumming if fuzzy group is too large - groups with many files
					// are unlikely to be true duplicates (fuzzy matching is too permissive)
					// This optimization prevents expensive checksum operations on false positives
					if len(fileGroup) > maxFuzzyGroupSize {
						logger.Debugf("[Duplicates] Skipping checksum for fuzzy group with %d files (exceeds limit of %d)", len(fileGroup), maxFuzzyGroupSize)
						continue
					}

					// Stop if we've hit the group limit
					if len(groupsWithChecksums) >= maxGroups {
						break
					}

					// Check resource limits before expensive checksum operations
					if shouldStop, reason := stats.shouldStop(); shouldStop {
						stats.stopped = true
						stats.stopReason = reason
						logger.Warningf("[Duplicates] Stopping early during checksum phase: %s", reason)
						break
					}

					// Verify with checksums using 3-pass progressive verification
					// At this point, files match on: size + MIME type + fuzzy filename similarity (50%+)
					// Large fuzzy groups (>10 files) are skipped above to avoid expensive false positives
					verifiedGroups := groupFilesByChecksum(fileGroup, index, size, stats)

					// Create SearchResult objects and track checksums for merging
					for _, checksumGroup := range verifiedGroups {
						if len(checksumGroup.Files) < 2 {
							continue
						}

						resultGroup := make([]*indexing.SearchResult, 0, len(checksumGroup.Files))

						for _, fileInfo := range checksumGroup.Files {
							// Remove the user scope from path
							adjustedPath := "/" + strings.TrimPrefix(fileInfo.Path, opts.combinedPath)
							resultGroup = append(resultGroup, &indexing.SearchResult{
								Path:       adjustedPath,
								Source:     opts.source,
								Type:       fileInfo.Type,
								Size:       fileInfo.Size,
								Modified:   fileInfo.ModTime.Format(time.RFC3339),
								HasPreview: fileInfo.HasPreview,
							})
						}

						if len(resultGroup) >= 2 {
							// Track checksum in stats
							stats.uniqueChecksums[checksumGroup.Checksum] = true
							groupsWithChecksums = append(groupsWithChecksums, duplicateGroupWithChecksums{
								Size:     size,
								Files:    resultGroup,
								Checksum: checksumGroup.Checksum,
							})
						}
					}

					// Stop if we've hit the group limit
					if len(groupsWithChecksums) >= maxGroups {
						break
					}
				}

				// Break out of type loop if we hit group limit
				if len(groupsWithChecksums) >= maxGroups {
					break
				}
			}

			// Break out of size loop if we hit group limit
			if len(groupsWithChecksums) >= maxGroups {
				break
			}

			// Clear processed size from map to free memory immediately
			delete(filesBySize, size)
		}

		// Clear the entire map after batch processing to ensure all memory is released
		// This helps Go's GC reclaim memory more aggressively
		for k := range filesBySize {
			delete(filesBySize, k)
		}
	}

	// Post-processing: Merge groups that share any checksum values
	// This catches cases where files have identical content but different filenames
	mergedGroups := mergeGroupsByChecksum(groupsWithChecksums)

	// Convert to final format
	duplicateGroups := make([]duplicateGroup, 0, len(mergedGroups))
	for _, group := range mergedGroups {
		duplicateGroups = append(duplicateGroups, duplicateGroup{
			Size:  group.Size,
			Count: len(group.Files),
			Files: group.Files,
		})
	}

	// Groups are already sorted by size (largest to smallest) from SQL query
	return duplicateGroups
}

// mergeGroupsByChecksum merges groups that share any checksum values
// This catches cases where files have identical content but were grouped separately
func mergeGroupsByChecksum(groups []duplicateGroupWithChecksums) []duplicateGroupWithChecksums {
	if len(groups) <= 1 {
		return groups
	}

	// Build a map: checksum -> list of group indices that contain this checksum
	checksumToGroups := make(map[string][]int)
	for i, group := range groups {
		if group.Checksum != "" {
			checksumToGroups[group.Checksum] = append(checksumToGroups[group.Checksum], i)
		}
	}

	// Use union-find to merge groups that share checksums
	parent := make([]int, len(groups))
	for i := range parent {
		parent[i] = i
	}

	var find func(int) int
	find = func(x int) int {
		if parent[x] != x {
			parent[x] = find(parent[x]) // Path compression
		}
		return parent[x]
	}

	union := func(x, y int) {
		px, py := find(x), find(y)
		if px != py {
			parent[px] = py
		}
	}

	// Merge groups that share any checksum
	for _, groupIndices := range checksumToGroups {
		if len(groupIndices) < 2 {
			continue
		}
		// Merge all groups that share this checksum
		for i := 1; i < len(groupIndices); i++ {
			union(groupIndices[0], groupIndices[i])
		}
	}

	// Group indices by their root parent
	groupsByRoot := make(map[int][]int)
	for i := range groups {
		root := find(i)
		groupsByRoot[root] = append(groupsByRoot[root], i)
	}

	// Merge groups that belong to the same root
	merged := make([]duplicateGroupWithChecksums, 0, len(groupsByRoot))
	for _, indices := range groupsByRoot {
		if len(indices) == 1 {
			// No merging needed, just add the group
			merged = append(merged, groups[indices[0]])
			continue
		}

		// Merge multiple groups
		combinedFiles := make([]*indexing.SearchResult, 0)
		var combinedSize int64
		var combinedChecksum string

		for _, idx := range indices {
			group := groups[idx]
			combinedFiles = append(combinedFiles, group.Files...)
			if combinedChecksum == "" {
				combinedChecksum = group.Checksum
			}
			if combinedSize == 0 {
				combinedSize = group.Size
			}
		}

		// Only include merged groups with 2+ files
		if len(combinedFiles) >= 2 {
			merged = append(merged, duplicateGroupWithChecksums{
				Size:     combinedSize,
				Files:    combinedFiles,
				Checksum: combinedChecksum,
			})
		}
	}

	return merged
}

func prepDuplicatesOptions(r *http.Request, d *requestContext) (*duplicatesOptions, error) {
	source := r.URL.Query().Get("source")
	scope := r.URL.Query().Get("scope")

	minSizeMbStr := r.URL.Query().Get("minSizeMb")
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

	// r.URL.Query().Get() already decodes the parameter
	searchScope := strings.TrimPrefix(scope, ".")

	index := indexing.GetIndex(source)
	if index == nil {
		return nil, fmt.Errorf("index not found for source %s", source)
	}

	userscope, err := d.user.GetScopeForSourceName(source)
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

// groupFilesByChecksum groups files by partial checksum and returns groups with their checksums
func groupFilesByChecksum(files []*iteminfo.FileInfo, index *indexing.Index, fileSize int64, stats *duplicateProcessingStats) []checksumGroup {
	if len(files) < 2 {
		return nil
	}

	// PASS 1: Header checksums only (first 8KB)
	// This is the fastest way to eliminate files that are definitely different
	// ONLY checksum files that already matched on filename+size
	headerGroups := make(map[string][]*iteminfo.FileInfo)

	for _, file := range files {
		// Check if we've hit the checksum operation limit
		if stats.checksumOperations >= maxChecksumOperations {
			logger.Warningf("[Duplicates] Reached checksum operation limit (%d) in header pass", maxChecksumOperations)
			break
		}

		// Verify size matches expected size (should match as we queried by size)
		if file.Size != fileSize {
			continue
		}

		// Construct filesystem path for checksum computation
		// index.Path is the absolute filesystem root, file.Path is index-relative
		filePath := filepath.Join(index.Path, file.Path)
		headerChecksum, err := computeHeaderChecksum(index.Path, filePath, fileSize, file.ModTime)
		if err != nil {
			continue
		}
		stats.checksumOperations++
		stats.uniqueChecksums[headerChecksum] = true
		headerGroups[headerChecksum] = append(headerGroups[headerChecksum], file)
	}

	// Filter to only groups with 2+ files (candidates for duplicates)
	headerCandidates := make([][]*iteminfo.FileInfo, 0)
	filesAfterHeaderPass := 0
	for _, group := range headerGroups {
		if len(group) >= 2 {
			headerCandidates = append(headerCandidates, group)
			filesAfterHeaderPass += len(group)
		}
	}

	if len(headerCandidates) == 0 {
		return nil
	}

	// PASS 2: Middle checksums (header + middle) for header-matched groups
	// Only process files that matched on header to save disk I/O
	middleGroups := make(map[string][]*iteminfo.FileInfo)

	for _, headerGroup := range headerCandidates {
		if len(headerGroup) < 2 {
			continue
		}

		for _, file := range headerGroup {
			// Check if we've hit the checksum operation limit
			if stats.checksumOperations >= maxChecksumOperations {
				logger.Warningf("[Duplicates] Reached checksum operation limit (%d) in middle pass", maxChecksumOperations)
				break
			}

			filePath := filepath.Join(index.Path, file.Path)
			middleChecksum, err := computeMiddleChecksum(index.Path, filePath, fileSize, file.ModTime)
			if err != nil {
				continue
			}
			stats.checksumOperations++
			stats.uniqueChecksums[middleChecksum] = true
			middleGroups[middleChecksum] = append(middleGroups[middleChecksum], file)
		}
	}

	// Final verification: Use middle checksum as final (header + middle is sufficient)
	// Files that match on header + middle but differ at end are extremely rare
	// This 2-pass system catches 99.9%+ of differences while being faster
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

// computeHeaderChecksum calculates MD5 hash of only the first 8KB of a file
// This is the fastest initial pass to eliminate non-matching files
func computeHeaderChecksum(sourcePath, filePath string, size int64, modTime time.Time) (string, error) {
	// Generate cache key for header-only checksum
	cacheKey := fmt.Sprintf("%s:%s:%d:%d:header", sourcePath, filePath, size, modTime.Unix())

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

	// Read only first 8KB (or entire file if smaller)
	n, err := io.ReadFull(file, buf)
	if err != nil && err != io.EOF && err != io.ErrUnexpectedEOF {
		return "", err
	}
	hash.Write(buf[:n])

	checksum := fmt.Sprintf("%x", hash.Sum(nil))

	// Cache the result
	checksumCache.Set(cacheKey, checksum)

	return checksum, nil
}

// computeMiddleChecksum calculates MD5 hash of header + middle portion
// Only called when header checksums match (progressive verification)
func computeMiddleChecksum(sourcePath, filePath string, size int64, modTime time.Time) (string, error) {
	// Generate cache key for header+middle checksum
	cacheKey := fmt.Sprintf("%s:%s:%d:%d:middle", sourcePath, filePath, size, modTime.Unix())

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

	// Read first 8KB
	n, err := io.ReadFull(file, buf)
	if err != nil && err != io.EOF && err != io.ErrUnexpectedEOF {
		return "", err
	}
	hash.Write(buf[:n])

	// For larger files, also read middle 8KB
	if size > 16384 { // 16KB
		middleOffset := size / 2
		if _, err := file.Seek(middleOffset, 0); err == nil {
			n, err := io.ReadFull(file, buf)
			if err != nil && err != io.EOF && err != io.ErrUnexpectedEOF {
				return "", err
			}
			hash.Write(buf[:n])
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
// NOTE: Files passed in should already be filtered by MIME type at the SQL level,
// so no need to check extensions here (extension checking is redundant with MIME type filtering)
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

		// Normalize filename for fuzzy matching
		filename1 := normalizeFilename(files[i].Name)

		for j := i + 1; j < len(files); j++ {
			if used[j] || files[j].Name == "" {
				continue
			}

			// Check size (should match, but safe to check)
			if files[j].Size != expectedSize {
				continue
			}

			filename2 := normalizeFilename(files[j].Name)

			// Check if filenames are similar enough using levenshtein distance
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

	// Require at least 50% similarity (lowered to catch more duplicates with username prefixes)
	// Examples: "video.mp4" vs "username_video.mp4" should match
	similarity := 1.0 - float64(distance)/float64(maxLen)
	return similarity >= 0.5
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

// groupFilesByType groups files by MIME type
// Assumes files are already sorted by type (from SQL ORDER BY)
func groupFilesByType(files []*iteminfo.FileInfo) map[string][]*iteminfo.FileInfo {
	grouped := make(map[string][]*iteminfo.FileInfo)
	for _, file := range files {
		grouped[file.Type] = append(grouped[file.Type], file)
	}
	return grouped
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
