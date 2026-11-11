package web

import (
	"context"
	"fmt"
	"net/http"
	"path/filepath"
	"strings"
	"sync"
	"time"

	"github.com/gtsteffaniak/filebrowser/backend/internal/activity"
	activitydb "github.com/gtsteffaniak/filebrowser/backend/internal/database/activity"
	"github.com/gtsteffaniak/filebrowser/backend/internal/state"
	"github.com/gtsteffaniak/filebrowser/backend/internal/utils"
	"github.com/gtsteffaniak/filebrowser/backend/pkg/indexing"
	"github.com/gtsteffaniak/filebrowser/backend/pkg/indexing/iteminfo"
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
//   The checksum budget counts only cache misses (real disk I/O); cached digests are free,
//   so a re-run proceeds past the previously-examined files instead of stalling at the limit.
// - Files are processed in batches to balance memory usage vs SQL query count
// - Resource limits prevent timeouts: max 2 minutes, max 10K checksum operations
// - Large fuzzy groups are skipped to avoid expensive checksum operations on false positives
//
// Response includes incomplete flag if processing was stopped early due to resource limits.
//
// @Summary Find Duplicate Files
// @Description Finds duplicate files using multi-stage filtering: size → type → fuzzy filename → progressive checksums. Files must match on size, MIME type, and have 50%+ filename similarity before checksum verification. Large fuzzy groups (>10 files) are skipped to avoid false positives. Checksums use 2-pass progressive verification (header → middle) for accuracy while minimizing disk I/O (~16KB read per file).
// @Tags Tools
// @Accept json
// @Produce json
// @Param source query string true "Source name for the desired source"
// @Param scope query string false "path within user scope to search"
// @Param minSizeMb query int false "Minimum file size in megabytes (default: 1)"
// @Success 200 {object} duplicateResponse "List of duplicate file groups with metadata. Response includes 'incomplete' flag if processing stopped early due to resource limits."
// @Failure 400 {object} map[string]string "Bad Request"
// @Failure 503 {object} map[string]string "Service Unavailable (indexing in progress or another search running)"
// @Router /api/tools/duplicate-finder [get]
func duplicatesHandler(w http.ResponseWriter, r *http.Request, d *Context) (int, error) {
	started := time.Now()
	opts, err := prepDuplicatesOptions(r, d)
	if err != nil {
		return http.StatusBadRequest, err
	}

	index := indexing.GetIndex(opts.source)
	if index == nil {
		return http.StatusBadRequest, fmt.Errorf("index not found for source %s", opts.source)
	}
	userscope, err := d.User.GetScopeForSourceName(index.Name)
	if err != nil {
		return http.StatusForbidden, err
	}
	userscope = strings.TrimRight(userscope, "/")
	scopePath := utils.JoinPathAsUnix(userscope, opts.searchScope)
	fullPath := index.MakeIndexPath(scopePath, true)
	if !state.AccessPermitted(index.Path, fullPath, d.User.Username) {
		return http.StatusForbidden, fmt.Errorf("user is not allowed to access this location")
	}

	// Safety check: Reject duplicate search during active indexing to prevent resource contention
	// This protects against CPU/memory spikes and ensures indexing performance is not degraded
	// Use GetStatus() (computed from active scanners and persisted lastScanned) rather than
	// the raw Status field, which is only set to "ready" by PostScan and would otherwise
	// block searches indefinitely after a restart until the first scheduled scan runs.
	if index.GetStatus() == indexing.INDEXING {
		return http.StatusServiceUnavailable, fmt.Errorf("duplicate search is not available while indexing is in progress - please try again when indexing completes")
	}

	userscope, err = d.User.GetScopeForSourceName(index.Name)
	if err != nil {
		return http.StatusForbidden, err
	}
	userscope = strings.TrimRight(userscope, "/")
	scopePath = utils.JoinPathAsUnix(userscope, opts.searchScope)
	fullPath = index.MakeIndexPath(scopePath, true)
	if !state.AccessPermitted(index.Path, fullPath, d.User.Username) {
		return http.StatusForbidden, fmt.Errorf("user is not allowed to access this location")
	}

	logDuplicateFinder := func(response duplicateResponse, cached bool) {
		details := activitydb.Details{
			Source:     index.Name,
			Path:       scopePath,
			FileCount:  len(response.Groups),
			DurationMs: time.Since(started).Milliseconds(),
			Truncated:  response.Incomplete,
			Cached:     cached,
		}
		if response.Reason != "" {
			details.Error = response.Reason
		}
		activity.RecordTool(r, toActor(d), activitydb.EventDuplicateFinder, details)
	}

	// Generate cache key from all input parameters that affect results
	// Checksums are always enabled, so cache key doesn't need to include that flag
	cacheKey := fmt.Sprintf("%s:%s:%s:%d", index.Path, opts.combinedPath, opts.username, opts.minSize)

	// Check cache first (before acquiring mutex)
	if cachedResults, ok := duplicateResultsCache.Get(cacheKey); ok {
		// Set headers if cached result was incomplete (metadata only, don't change status code)
		if cachedResults.Incomplete {
			w.Header().Set("X-Search-Incomplete", "true")
			w.Header().Set("X-Search-Incomplete-Reason", cachedResults.Reason)
		}
		logDuplicateFinder(cachedResults, true)
		return RenderJSON(w, r, cachedResults)
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
		logDuplicateFinder(cachedResults, true)
		return RenderJSON(w, r, cachedResults)
	}

	// Find duplicates using index-native approach with resource limits
	// This avoids creating SearchResult objects until we know the final limited set
	stats := &duplicateProcessingStats{
		startTime:       time.Now(),
		uniqueChecksums: make(map[string]bool),
	}
	duplicateGroups := findDuplicatesInIndex(r.Context(), index, opts, stats)

	// The client went away — do not cache or record a truncated result. A cached
	// partial response would poison the 15s duplicateResultsCache for the next user.
	if err := r.Context().Err(); err != nil {
		return http.StatusRequestTimeout, err
	}

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

	logDuplicateFinder(response, false)
	return RenderJSON(w, r, response)
}

// findDuplicatesInIndex finds duplicates using the shared IndexDB with resource limits
func findDuplicatesInIndex(ctx context.Context, index *indexing.Index, opts *duplicatesOptions, stats *duplicateProcessingStats) []duplicateGroup {
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
		if ctx.Err() != nil {
			stats.stopped = true
			stats.stopReason = ctx.Err().Error()
			break
		}
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
			if ctx.Err() != nil {
				stats.stopped = true
				stats.stopReason = ctx.Err().Error()
				break
			}
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

			// Group files by MIME (+ extension when MIME is ambiguous) within this size bucket.
			filesByType := groupFilesByTypeBucket(files)

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
				filenameGroups := groupFilesByFilename(ctx, typeFiles, size, stats)

				for _, fileGroup := range filenameGroups {
					if len(fileGroup) < 2 {
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
					verifiedGroups := groupFilesByChecksum(ctx, fileGroup, index, size, stats)

					// Create SearchResult objects and track checksums for merging
					for _, checksumGroup := range verifiedGroups {
						if len(checksumGroup.Files) < 2 {
							continue
						}

						resultGroup := make([]*indexing.SearchResult, 0, len(checksumGroup.Files))

						for _, fileInfo := range checksumGroup.Files {
							// Remove the user scope from path
							adjustedPath := "/" + strings.TrimPrefix(fileInfo.Path, opts.combinedPath)
							result := &indexing.SearchResult{
								Path:       adjustedPath,
								Source:     opts.source,
								Type:       fileInfo.Type,
								Size:       fileInfo.Size,
								Modified:   fileInfo.ModTime.Format(time.RFC3339),
								HasPreview: fileInfo.HasPreview,
							}
							if fileInfo.Created != nil {
								result.Created = fileInfo.Created.Format(time.RFC3339)
							}
							resultGroup = append(resultGroup, result)
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

	return duplicateGroups
}

func prepDuplicatesOptions(r *http.Request, d *Context) (*duplicatesOptions, error) {
	source := r.URL.Query().Get("source")
	scope, err := utils.SanitizePath(r.URL.Query().Get("scope"))
	if err != nil {
		return nil, fmt.Errorf("invalid scope: %v", err)
	}

	minSizeMbStr := r.URL.Query().Get("minSizeMb")
	// First pass uses filename matching for speed, then checksums verify final groups
	useChecksum := true

	// Default minimum size: 1MB
	minSizeMb := int64(1)
	if minSizeMbStr != "" {
		if _, err = fmt.Sscanf(minSizeMbStr, "%d", &minSizeMb); err != nil {
			return nil, fmt.Errorf("invalid minSizeMb parameter: %w", err)
		}
	}
	// Convert MB to bytes
	minSize := minSizeMb * 1024 * 1024

	searchScope := strings.TrimPrefix(scope, ".")

	index := indexing.GetIndex(source)
	if index == nil {
		return nil, fmt.Errorf("index not found for source %s", source)
	}

	userscope, err := d.User.GetScopeForSourceName(source)
	if err != nil {
		return nil, err
	}

	combinedPath := index.MakeIndexPath(filepath.Join(userscope, searchScope), true) // searchScope is a directory

	return &duplicatesOptions{
		source:       source,
		searchScope:  searchScope,
		combinedPath: combinedPath.String(),
		minSize:      minSize,
		useChecksum:  useChecksum,
		username:     d.User.Username,
	}, nil
}

// filterFilesByPermission filters files to only include those the user is permitted to access
// This is called early in the duplicate search process to avoid processing files the user can't see
func filterFilesByPermission(files []*iteminfo.FileInfo, index *indexing.Index, username string) []*iteminfo.FileInfo {
	if false {
		// No access control configured, return all files
		return files
	}

	filtered := make([]*iteminfo.FileInfo, 0, len(files))
	for _, file := range files {
		// Check permission using index.Path (source root) and file.Path (index-relative path)
		if state.AccessPermitted(index.Path, utils.IndexPathFromNormalized(file.Path, true), username) {
			filtered = append(filtered, file)
		}
	}
	return filtered
}
