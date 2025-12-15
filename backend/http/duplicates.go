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

// Safety limits to prevent resource exhaustion and server attacks
const (
	maxGroups              = 500               // Limit total duplicate groups returned
	maxFilesToScan         = 50000             // Maximum files to scan per request (prevents unbounded scans)
	maxProcessingTime      = 120 * time.Second // Maximum time to spend processing (2 minutes)
	maxChecksumOperations  = 10000             // Maximum checksum operations per request (prevents excessive disk I/O)
	maxSizeGroupsToProcess = 1000              // Maximum size groups to process (prevents memory exhaustion)
	bulkSizeQueryLimit     = 100               // Process sizes in batches of 100 to balance memory vs query count
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
	if s.sizeGroupsProcessed >= maxSizeGroupsToProcess {
		return true, fmt.Sprintf("size group limit exceeded (%d groups)", maxSizeGroupsToProcess)
	}
	return false, ""
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

	// Safety check: Reject duplicate search during active indexing to prevent resource contention
	// This protects against CPU/memory spikes and ensures indexing performance is not degraded
	if string(index.Status) == "indexing" {
		return http.StatusServiceUnavailable, fmt.Errorf("duplicate search is not available while indexing is in progress - please try again when indexing completes")
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
		// Set headers if cached result was incomplete
		if cachedResults.Incomplete {
			w.Header().Set("X-Search-Incomplete", "true")
			w.Header().Set("X-Search-Incomplete-Reason", cachedResults.Reason)
			w.WriteHeader(http.StatusPartialContent)
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
		// Set headers if cached result was incomplete
		if cachedResults.Incomplete {
			w.Header().Set("X-Search-Incomplete", "true")
			w.Header().Set("X-Search-Incomplete-Reason", cachedResults.Reason)
			w.WriteHeader(http.StatusPartialContent)
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
	} else {
		logger.Infof("[Duplicates] Search completed: scanned %d files, %d checksum ops, %d unique checksums added to cache, %d groups in %v",
			stats.filesScanned, stats.checksumOperations, uniqueChecksumCount, stats.sizeGroupsProcessed, time.Since(stats.startTime))
	}

	// Build response with metadata about completeness
	response := duplicateResponse{
		Groups:     duplicateGroups,
		Incomplete: stats.stopped,
		Reason:     stats.stopReason,
	}

	// Cache the results before returning (even partial results)
	duplicateResultsCache.Set(cacheKey, response)

	// Return 206 Partial Content if search was stopped early due to resource limits
	// This allows the frontend to distinguish between complete and incomplete results
	if stats.stopped {
		w.Header().Set("X-Search-Incomplete", "true")
		w.Header().Set("X-Search-Incomplete-Reason", stats.stopReason)
		w.WriteHeader(http.StatusPartialContent)
	}

	return renderJSON(w, r, response)
}

// findDuplicatesInIndex finds duplicates using the shared IndexDB with resource limits
func findDuplicatesInIndex(index *indexing.Index, opts *duplicatesOptions, stats *duplicateProcessingStats) []duplicateGroup {
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

	// Safety check: Limit the number of size groups to prevent memory exhaustion
	if len(sizes) > maxSizeGroupsToProcess {
		logger.Warningf("[Duplicates] Limiting size groups from %d to %d for safety", len(sizes), maxSizeGroupsToProcess)
		sizes = sizes[:maxSizeGroupsToProcess]
	}

	// Step 2: Process sizes in batches using bulk queries to minimize SQL round-trips
	duplicateGroups := []duplicateGroup{}
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
		if len(duplicateGroups) >= maxGroups {
			break
		}

		// Get batch of sizes to process
		batchEnd := batchStart + bulkSizeQueryLimit
		if batchEnd > len(sizes) {
			batchEnd = len(sizes)
		}
		sizeBatch := sizes[batchStart:batchEnd]

		// BULK QUERY: Load ALL files for ALL sizes in this batch with ONE query
		bulkQueryStart := time.Now()
		filesBySize, err := indexDB.GetFilesForMultipleSizes(opts.source, sizeBatch, pathPrefix)
		bulkQueryDuration := time.Since(bulkQueryStart)
		totalBulkQueryTime += bulkQueryDuration
		bulkQueryCount++

		if err != nil {
			logger.Errorf("[Duplicates] Failed to bulk query files for %d sizes: %v", len(sizeBatch), err)
			continue
		}

		logger.Debugf("[Duplicates] Bulk query %d: fetched files for %d sizes in %v (%d files total)",
			bulkQueryCount, len(sizeBatch), bulkQueryDuration, getTotalFileCount(filesBySize))

		// Process each size within this batch
		for _, size := range sizeBatch {
			files, ok := filesBySize[size]
			if !ok || len(files) < 2 {
				continue
			}

			stats.sizeGroupsProcessed++
			stats.filesScanned += len(files)

			// Filter files by permission early, before any processing
			files = filterFilesByPermission(files, index, opts.username)
			if len(files) < 2 {
				continue
			}

			// Group files by MIME type in memory (already ordered by type from SQL)
			filesByType := groupFilesByType(files)

			// Process each MIME type group
			for mimeType, typeFiles := range filesByType {
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

				if totalFilesInGroups > 0 {
					logger.Debugf("[Duplicates] Size %d, type '%s': %d files → %d fuzzy filename groups with %d files eligible for checksumming",
						size, mimeType, len(typeFiles), len(filenameGroups), totalFilesInGroups)
				}

				// Process each fuzzy filename group
				for _, fileGroup := range filenameGroups {
					if len(fileGroup) < 2 {
						continue
					}

					// Stop if we've hit the group limit
					if len(duplicateGroups) >= maxGroups {
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
					// At this point, files match on: size + MIME type + fuzzy filename similarity
					verifiedGroups := groupFilesByChecksum(fileGroup, index, size, stats)

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

				// Break out of type loop if we hit group limit
				if len(duplicateGroups) >= maxGroups {
					break
				}
			}

			// Break out of size loop if we hit group limit
			if len(duplicateGroups) >= maxGroups {
				break
			}
		}
	}

	// Log aggregate query performance
	if bulkQueryCount > 0 {
		avgBulkQueryTime := totalBulkQueryTime / time.Duration(bulkQueryCount)
		logger.Infof("[Duplicates] Bulk query performance: %d queries (was %d+ individual queries), total %v, avg %v per batch",
			bulkQueryCount, len(sizes), totalBulkQueryTime, avgBulkQueryTime)
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
func groupFilesByChecksum(files []*iteminfo.FileInfo, index *indexing.Index, fileSize int64, stats *duplicateProcessingStats) [][]*iteminfo.FileInfo {
	if len(files) < 2 {
		return nil
	}

	initialFileCount := len(files)

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
		logger.Debugf("[Duplicates] Header pass eliminated all %d files (no header matches)", initialFileCount)
		return nil
	}

	logger.Debugf("[Duplicates] Header pass: %d→%d files (%d eliminated, %d groups)",
		initialFileCount, filesAfterHeaderPass, initialFileCount-filesAfterHeaderPass, len(headerCandidates))

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

	// Filter to only groups with 2+ files
	middleCandidates := make([][]*iteminfo.FileInfo, 0)
	filesAfterMiddlePass := 0
	for _, group := range middleGroups {
		if len(group) >= 2 {
			middleCandidates = append(middleCandidates, group)
			filesAfterMiddlePass += len(group)
		}
	}

	if len(middleCandidates) == 0 {
		logger.Debugf("[Duplicates] Middle pass eliminated all %d files (no middle matches)", filesAfterHeaderPass)
		return nil
	}

	logger.Debugf("[Duplicates] Middle pass: %d→%d files (%d eliminated, %d groups)",
		filesAfterHeaderPass, filesAfterMiddlePass, filesAfterHeaderPass-filesAfterMiddlePass, len(middleCandidates))

	// PASS 3: Three-part checksums (header + middle + end) for middle-matched groups
	// This is the final verification to ensure files are truly identical
	// Still only reads ~24KB max per file (not full file)
	threePartGroups := make(map[string][]*iteminfo.FileInfo)

	for _, middleGroup := range middleCandidates {
		if len(middleGroup) < 2 {
			continue
		}

		for _, file := range middleGroup {
			// Check if we've hit the checksum operation limit
			if stats.checksumOperations >= maxChecksumOperations {
				logger.Warningf("[Duplicates] Reached checksum operation limit (%d) in three-part pass", maxChecksumOperations)
				break
			}

			filePath := filepath.Join(index.Path, file.Path)
			threePartChecksum, err := computeThreePartChecksum(index.Path, filePath, fileSize, file.ModTime)
			if err != nil {
				continue
			}
			stats.checksumOperations++
			stats.uniqueChecksums[threePartChecksum] = true
			threePartGroups[threePartChecksum] = append(threePartGroups[threePartChecksum], file)
		}
	}

	// Convert final groups to slice
	groups := make([][]*iteminfo.FileInfo, 0, len(threePartGroups))
	filesAfterThreePartPass := 0
	for _, group := range threePartGroups {
		if len(group) >= 2 {
			groups = append(groups, group)
			filesAfterThreePartPass += len(group)
		}
	}

	if len(groups) > 0 {
		logger.Debugf("[Duplicates] Three-part pass: %d→%d files (%d eliminated, %d verified groups) [%d→%d→%d progressive elimination]",
			filesAfterMiddlePass, filesAfterThreePartPass, filesAfterMiddlePass-filesAfterThreePartPass, len(groups),
			initialFileCount, filesAfterHeaderPass, filesAfterThreePartPass)
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

// computeThreePartChecksum calculates MD5 hash by sampling all three key portions
// Only called when both header and middle checksums match (final verification)
// This is still a PARTIAL checksum, NOT a full file checksum:
// - First 8KB (header/metadata)
// - Middle 8KB
// - Last 8KB
// Total read: ~24KB max per file regardless of file size
func computeThreePartChecksum(sourcePath, filePath string, size int64, modTime time.Time) (string, error) {
	// Generate cache key from source path, file path, and modification time
	cacheKey := fmt.Sprintf("%s:%s:%d:%d:threepart", sourcePath, filePath, size, modTime.Unix())

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

// getTotalFileCount counts total files across all sizes in the map
func getTotalFileCount(filesBySize map[int64][]*iteminfo.FileInfo) int {
	total := 0
	for _, files := range filesBySize {
		total += len(files)
	}
	return total
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
