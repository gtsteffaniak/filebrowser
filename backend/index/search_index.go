package index

import (
	"math/rand"
	"mime"
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
)

func (si *Index) Search(search string, scope string, sourceSession string) ([]string, map[string]map[string]bool) {
	runningHash := generateRandomHash(4)
	sessionInProgress.Store(sourceSession, runningHash) // Store the value in the sync.Map

	searchOptions := ParseSearch(search)
	mutex.RLock()
	defer mutex.RUnlock()
	fileListTypes := make(map[string]map[string]bool)
	var matching []string
	maximum := 100

	for _, searchTerm := range searchOptions.Terms {
		if searchTerm == "" {
			continue
		}
		// Iterate over the embedded index.Index fields Dirs and Files
		for _, i := range []string{"Dirs", "Files"} {
			isDir := i == "Dirs"
			count := 0
			var paths []string

			switch i {
			case "Dirs":
				paths = si.Dirs
			case "Files":
				paths = si.Files
			}

			for _, path := range paths {
				value, found := sessionInProgress.Load(sourceSession)
				if !found || value != runningHash {
					return []string{}, map[string]map[string]bool{}
				}
				if count > maximum {
					break
				}
				pathName := scopedPathNameFilter(path, scope)
				if pathName == "" {
					continue
				}
				matches, fileType := containsSearchTerm(path, searchTerm, *searchOptions, isDir)
				if !matches {
					continue
				}
				if isDir {
					pathName = pathName + "/"
				}
				matching = append(matching, pathName)
				fileListTypes[pathName] = fileType
				count++
			}
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
	if strings.HasPrefix(pathName, scope) {
		pathName = strings.TrimPrefix(pathName, scope)
	} else {
		pathName = ""
	}
	return pathName
}

func containsSearchTerm(pathName string, searchTerm string, options SearchOptions, isDir bool) (bool, map[string]bool) {
	conditions := options.Conditions
	path := getLastPathComponent(pathName)
	// Convert to lowercase once
	lowerSearchTerm := searchTerm
	if !conditions["exact"] {
		path = strings.ToLower(path)
		lowerSearchTerm = strings.ToLower(searchTerm)
	}
	if strings.Contains(path, lowerSearchTerm) {
		// Reuse the fileTypes map and clear its values
		fileTypes := map[string]bool{
			"audio":   false,
			"image":   false,
			"video":   false,
			"doc":     false,
			"archive": false,
			"dir":     false,
		}
		// Calculate fileSize only if needed
		var fileSize int64
		if conditions["larger"] || conditions["smaller"] {
			fileSize = getFileSize(pathName)
		}
		matchesAllConditions := true
		extension := filepath.Ext(path)
		mimetype := mime.TypeByExtension(extension)
		fileTypes["audio"] = strings.HasPrefix(mimetype, "audio")
		fileTypes["image"] = strings.HasPrefix(mimetype, "image")
		fileTypes["video"] = strings.HasPrefix(mimetype, "video")
		fileTypes["doc"] = isDoc(extension)
		fileTypes["archive"] = isArchive(extension)
		fileTypes["dir"] = isDir
		for t, v := range conditions {
			if t == "exact" {
				continue
			}
			var matchesCondition bool
			switch t {
			case "larger":
				matchesCondition = fileSize > int64(options.LargerThan)*1000000
			case "smaller":
				matchesCondition = fileSize < int64(options.SmallerThan)*1000000
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

func isDoc(extension string) bool {
	for _, typefile := range documentTypes {
		if extension == typefile {
			return true
		}
	}
	return false
}

func getFileSize(filepath string) int64 {
	fileInfo, err := os.Stat(rootPath + "/" + filepath)
	if err != nil {
		return 0
	}
	return fileInfo.Size()
}

func isArchive(extension string) bool {
	for _, typefile := range compressedFile {
		if extension == typefile {
			return true
		}
	}
	return false
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
