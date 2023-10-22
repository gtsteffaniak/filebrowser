package index

import (
	"math/rand"
	"os"
	"path/filepath"
	"sort"
	"strings"
	"sync"
	"time"
)

var (
	sessionInProgress sync.Map
	mutex             sync.RWMutex
	maxSearchResults        = 100
	bytesInMegabyte   int64 = 1000000
)

func (si *Index) Search(search string, scope string, sourceSession string) ([]string, map[string]map[string]bool) {
	if scope == "" {
		scope = "/"
	}
	isDir := false
	fileTypes := map[string]bool{}
	runningHash := generateRandomHash(4)
	sessionInProgress.Store(sourceSession, runningHash) // Store the value in the sync.Map
	searchOptions := ParseSearch(search)
	mutex.RLock()
	defer mutex.RUnlock()
	fileListTypes := make(map[string]map[string]bool)
	matching := []string{}
	for _, searchTerm := range searchOptions.Terms {
		if searchTerm == "" {
			continue
		}
		// Iterate over the embedded index.Index fields Dirs and Files
		for _, path := range si.Files {
			count := 0
			value, found := sessionInProgress.Load(sourceSession)
			if !found || value != runningHash {
				return []string{}, map[string]map[string]bool{}
			}
			if count > maxSearchResults {
				break
			}
			pathName := scopedPathNameFilter(path, scope)
			if pathName == "" {
				continue
			}

			matches, fileType := containsSearchTerm(path, searchTerm, *searchOptions, isDir, fileTypes, si.Root)
			if !matches {
				continue
			}
			if isDir {
				fileListTypes[pathName+"/"] = fileType
			} else {
				fileListTypes[pathName] = fileType
			}
			matching = append(matching, pathName)
			count++
		}
	}
	// Sort the strings based on the number of elements after splitting by "/"
	sort.Slice(matching, func(i, j int) bool {
		parts1 := strings.Split(matching[i], "/")
		parts2 := strings.Split(matching[j], "/")
		return len(parts1) < len(parts2)
	})
	return matching, fileListTypes
}

func scopedPathNameFilter(pathName string, scope string) string {
	scope = strings.TrimPrefix(scope, "/")
	pathName = strings.TrimPrefix(pathName, "/")
	if strings.HasPrefix(pathName, scope) {
		pathName = "/" + strings.TrimPrefix(pathName, scope)
	} else {
		pathName = ""
	}
	return pathName
}

func containsSearchTerm(pathName string, searchTerm string, options SearchOptions, isDir bool, fileTypes map[string]bool, rootPath string) (bool, map[string]bool) {
	conditions := options.Conditions
	path := getLastPathComponent(pathName)
	// Convert to lowercase once
	if !conditions["exact"] {
		path = strings.ToLower(path)
		searchTerm = strings.ToLower(searchTerm)
	}
	if strings.Contains(path, searchTerm) {
		// Calculate fileSize only if needed
		var fileSize int64
		matchesAllConditions := true
		extension := filepath.Ext(path)
		for _, k := range AllFiletypeOptions {
			if IsMatchingType(extension, k) {
				fileTypes[k] = true
			}
		}
		fileTypes["dir"] = isDir
		for t, v := range conditions {
			if t == "exact" {
				continue
			}
			var matchesCondition bool
			switch t {
			case "larger":
				if fileSize == 0 {
					fileSize = getFileSize(rootPath, pathName)
				}
				matchesCondition = fileSize > int64(options.LargerThan)*bytesInMegabyte
			case "smaller":
				if fileSize == 0 {
					fileSize = getFileSize(rootPath, pathName)
				}
				matchesCondition = fileSize < int64(options.SmallerThan)*bytesInMegabyte
			default:
				matchesCondition = v == fileTypes[t]
			}
			if !matchesCondition {
				matchesAllConditions = false
			}
		}
		return matchesAllConditions, fileTypes
	}
	// Clear variables and return
	return false, map[string]bool{}
}

func getFileSize(rootPath string, filepath string) int64 {
	fileInfo, err := os.Stat(rootPath + "/" + filepath)
	if err != nil {
		return 0
	}
	return fileInfo.Size()
}

func getLastPathComponent(path string) string {
	// Use filepath.Base to extract the last component of the path
	return filepath.Base(path)
}

func generateRandomHash(length int) string {
	const charset = "abcdefghijklmnopqrstuvwxyz0123456789"
	rand.New(rand.NewSource(time.Now().UnixNano()))
	result := make([]byte, length)
	for i := range result {
		result[i] = charset[rand.Intn(len(charset))]
	}
	return string(result)
}

func (node *TrieNode) SearchTrie(pattern string, currentPath string, results *[]string) {
	if node == nil {
		return
	}

	// Construct the current path by appending the node's name
	if currentPath != "" {
		currentPath += "/"
	}
	currentPath += pattern

	// Check if the pattern matches the end of the node
	if strings.HasSuffix(node.nodeName(), currentPath) {
		*results = append(*results, node.nodeName())
	}

	// Recursively search the children
	for _, child := range node.Children {
		child.SearchTrie(pattern, currentPath, results)
	}
}

func (node *TrieNode) nodeName() string {
	if node.IsDir {
		return node.Children[node.nodeNameKey()].nodeName()
	}
	return node.nodeNameKey()
}

func (node *TrieNode) nodeNameKey() string {
	for k := range node.Children {
		return k
	}
	return ""
}
