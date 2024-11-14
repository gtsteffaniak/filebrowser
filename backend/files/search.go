package files

import (
	"path/filepath"
	"sort"
	"strings"
	"sync"

	"github.com/gtsteffaniak/filebrowser/utils"
)

var (
	sessionInProgress sync.Map
	maxSearchResults  = 100
)

type searchResult struct {
	Path string `json:"path"`
	Type string `json:"type"`
	Size int64  `json:"size"`
}

func (si *Index) Search(search string, scope string, sourceSession string) []searchResult {
	// Remove slashes
	scope = strings.TrimLeft(scope, "/")
	scope = strings.TrimRight(scope, "/")
	adjustedPath := si.makeIndexPath(scope, true)
	runningHash := utils.GenerateRandomHash(4)
	sessionInProgress.Store(sourceSession, runningHash) // Store the value in the sync.Map
	searchOptions := ParseSearch(search)
	results := make([]searchResult, 0)
	count := 0

	directories := si.getSearchableDirs(adjustedPath)
	for _, searchTerm := range searchOptions.Terms {
		if searchTerm == "" {
			continue
		}
		si.mu.Lock()
		for _, dir := range directories {
			// search files first
			for fileName, item := range dir.Files {
				value, found := sessionInProgress.Load(sourceSession)
				if !found || value != runningHash {
					return []searchResult{}
				}
				if count > maxSearchResults {
					return results
				}
				matches, fileType, fileSize := item.containsSearchTerm(fileName, searchTerm, searchOptions)
				if matches {
					scopedPath := strings.TrimPrefix(strings.TrimPrefix(item.Path, scope), "/")
					results = append(results, searchResult{Path: scopedPath, Type: fileType, Size: fileSize})
					count++
				}
			}
			for dirName, item := range dir.Dirs {
				value, found := sessionInProgress.Load(sourceSession)
				if !found || value != runningHash {
					return []searchResult{}
				}
				if count > maxSearchResults {
					return results
				}
				matches, fileType, fileSize := item.containsSearchTerm(dirName, searchTerm, searchOptions)
				if matches {
					scopedPath := strings.TrimPrefix(strings.TrimPrefix(item.Path, scope), "/")
					results = append(results, searchResult{Path: scopedPath, Type: fileType, Size: fileSize})
					count++
				}
			}
		}
		si.mu.Unlock()
	}

	// Sort by the number of elements in Path after splitting by "/"
	sort.Slice(results, func(i, j int) bool {
		parts1 := strings.Split(results[i].Path, "/")
		parts2 := strings.Split(results[j].Path, "/")
		return len(parts1) < len(parts2)
	})

	return results
}

// returns true if the file name contains the search term
// returns file type if the file name contains the search term
// returns size of file/dir if the file name contains the search term
func (fi FileInfo) containsSearchTerm(fileName string, searchTerm string, options *SearchOptions) (bool, string, int64) {
	fileTypes := map[string]bool{}
	largerThan := int64(options.LargerThan) * 1024 * 1024
	smallerThan := int64(options.SmallerThan) * 1024 * 1024
	conditions := options.Conditions
	lowerFileName := strings.ToLower(fileName)

	// Convert to lowercase if not exact match
	if !conditions["exact"] {
		searchTerm = strings.ToLower(searchTerm)
	}

	// Check if the file name contains the search term
	if !strings.Contains(lowerFileName, searchTerm) {
		return false, "", 0
	}

	// Initialize file size and fileTypes map
	var fileSize int64
	extension := filepath.Ext(lowerFileName)

	fileType := "directory"
	// Collect file types
	for _, k := range AllFiletypeOptions {
		if IsMatchingType(extension, k) {
			fileTypes[k] = true
			fileType = k
		}
	}
	isDir := fi.Type == "directory"
	fileTypes["dir"] = isDir
	fileSize = fi.Size

	// Evaluate all conditions
	for t, v := range conditions {
		if t == "exact" {
			continue
		}
		switch t {
		case "larger":
			if largerThan > 0 {
				if fileSize <= largerThan {
					return false, "", 0
				}
			}
		case "smaller":
			if smallerThan > 0 {
				if fileSize >= smallerThan {
					return false, "", 0
				}
			}
		default:
			// Handle other file type conditions
			notMatchType := v != fileTypes[t]
			if notMatchType {
				return false, "", 0
			}
		}
	}

	return true, fileType, fileSize
}

func (si *Index) getSearchableDirs(adjustedPath string) []*FileInfo {
	dirs := []*FileInfo{}
	dirName := filepath.Base(adjustedPath)
	parentDir := filepath.Dir(adjustedPath)
	info, exists := si.GetMetadataInfo(parentDir, dirName)
	if !exists {
		return dirs
	}
	si.mu.RLock()
	defer si.mu.RUnlock()
	dirs = getDirsRecursively(dirName, &info, dirs)
	for dirName, dir := range info.Dirs {
		if dirName == adjustedPath {
			dirs = append(dirs, dir)
		}
	}
	return dirs
}

func getDirsRecursively(dirName string, dir *FileInfo, dirList []*FileInfo) []*FileInfo {
	dirList = append(dirList, dir)
	for name, item := range dir.Dirs {
		dirList = getDirsRecursively(name, item, dirList)
	}
	return dirList
}
