package indexing

import (
	"sort"
	"strings"
	"sync"
	"time"

	"github.com/gtsteffaniak/filebrowser/backend/common/utils"
	"github.com/gtsteffaniak/filebrowser/backend/indexing/iteminfo"
	"github.com/gtsteffaniak/go-cache/cache"
	"github.com/gtsteffaniak/go-logger/logger"
)

var SearchResultsCache = cache.NewCache[[]string](15 * time.Second)

var (
	sessionInProgress    sync.Map
	DefaultSearchResults = 100
)

type SearchResult struct {
	Path       string `json:"path"`
	Type       string `json:"type"`
	Size       int64  `json:"size"`
	Modified   string `json:"modified,omitempty"`
	HasPreview bool   `json:"hasPreview"`
}

func (idx *Index) Search(search string, scope string, sourceSession string, largest bool, limit int) []*SearchResult {
	if idx.db == nil {
		return []*SearchResult{}
	}

	// Ensure scope has consistent trailing slash for directory matching
	if scope != "" && !strings.HasSuffix(scope, "/") {
		scope = scope + "/"
	}
	// originalScope := scope // Preserve original scope for largest mode exclusion check
	if search == "" {
		scope = ""
	}
	runningHash := utils.InsecureRandomIdentifier(4)
	sessionInProgress.Store(sourceSession, runningHash) // Store the value in the sync.Map
	searchOptions := iteminfo.ParseSearch(search)
	results := make(map[string]*SearchResult, 0)
	count := 0

	// When largest=true and no search terms, ensure we run at least once
	if largest && len(searchOptions.Terms) == 0 {
		searchOptions.Terms = []string{""}
	}

	// Construct base query
	query := `
		SELECT path, name, size, mod_time, type, is_dir, has_preview 
		FROM index_items 
	`
	args := []interface{}{}
	whereClauses := []string{}

	// Apply scope filter
	if scope != "" {
		// Use GLOB for prefix matching which is supported by SQLite and efficient
		whereClauses = append(whereClauses, "path GLOB ?")
		args = append(args, scope+"*")
	}

	if largest {
		query += " ORDER BY size DESC"
	}
	
	if len(whereClauses) > 0 {
		query = strings.Replace(query, "FROM index_items", "FROM index_items WHERE "+strings.Join(whereClauses, " AND "), 1)
	}

	// For simple searches, we could potentially add name filtering to SQL
	// But to maintain full compatibility with ParseSearch (fuzzy matching, case sensitivity, etc),
	// we'll stream results and filter in Go. 
	// SQLite is fast enough to stream thousands of rows.
	
	rows, err := idx.db.Query(query, args...)
	if err != nil {
		logger.Errorf("Search query failed: %v", err)
		return []*SearchResult{}
	}
	defer rows.Close()

	for rows.Next() {
		// Check for cancellation
		value, found := sessionInProgress.Load(sourceSession)
		if !found || value != runningHash {
			return []*SearchResult{}
		}

		if limit > 0 && count >= limit {
			break
		}

		var path string
		var name string
		var size int64
		var modTime int64
		var mimeType string
		var isDir bool
		var hasPreview bool

		if err := rows.Scan(&path, &name, &size, &modTime, &mimeType, &isDir, &hasPreview); err != nil {
			logger.Errorf("Failed to scan search result row: %v", err)
			continue
		}

		// Create ItemInfo for matching
		item := iteminfo.ItemInfo{
			Name:       name,
			Size:       size,
			ModTime:    time.Unix(modTime, 0),
			Type:       mimeType,
			HasPreview: hasPreview,
		}

		// Check against all search terms
		for _, searchTerm := range searchOptions.Terms {
			if searchTerm == "" && !largest {
				continue
			}

			var matches bool
			if largest {
				// When largest=true, check size and type conditions, skip name matching
				largerThan := int64(searchOptions.LargerThan) * 1024 * 1024
				sizeMatches := largerThan == 0 || item.Size > largerThan
				// Check if directories should be excluded (when type:file is specified)
				dirCondition, hasDirCondition := searchOptions.Conditions["dir"]
				
				// For directories: match if dir condition is explicitly true
				// For files: match if no dir condition, or if dir condition is false
				var typeMatches bool
				if isDir {
					typeMatches = hasDirCondition && dirCondition
				} else {
					typeMatches = !hasDirCondition || (hasDirCondition && !dirCondition)
				}
				
				matches = sizeMatches && typeMatches
			} else {
				matches = item.ContainsSearchTerm(searchTerm, searchOptions)
			}

			if matches {
				// Determine type string for response
				resType := mimeType
				if isDir {
					resType = "directory"
				}

				results[path] = &SearchResult{
					Path:       path,
					Type:       resType,
					Size:       size,
					Modified:   item.ModTime.Format(time.RFC3339),
					HasPreview: hasPreview,
				}
				count++
				// Break inner loop (terms) if matched, move to next row
				break
			}
		}
	}

	// Sort keys based on the number of elements in the path after splitting by "/"
	sortedKeys := make([]*SearchResult, 0, len(results))
	for _, v := range results {
		sortedKeys = append(sortedKeys, v)
	}
	// Sort the strings based on the number of elements after splitting by "/"
	sort.Slice(sortedKeys, func(i, j int) bool {
		parts1 := strings.Split(sortedKeys[i].Path, "/")
		parts2 := strings.Split(sortedKeys[j].Path, "/")
		return len(parts1) < len(parts2)
	})
	return sortedKeys
}
