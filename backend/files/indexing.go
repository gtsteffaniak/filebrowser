package files

import (
	"log"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"time"

	"github.com/gtsteffaniak/filebrowser/settings"
)

type Index struct {
	Root        string
	Directories map[string]*FileInfo
	NumDirs     int
	NumFiles    int
	inProgress  bool
	LastIndexed time.Time
	mu          sync.RWMutex
}

var (
	rootPath     string = "/srv"
	indexes      []*Index
	indexesMutex sync.RWMutex
)

func InitializeIndex(intervalMinutes uint32, schedule bool) {
	if schedule {
		go indexingScheduler(intervalMinutes)
	}
}

func indexingScheduler(intervalMinutes uint32) {
	if settings.Config.Server.Root != "" {
		rootPath = settings.Config.Server.Root
	}
	si := GetIndex(rootPath)
	for {
		startTime := time.Now()
		// Set the indexing flag to indicate that indexing is in progress
		si.resetCount()
		// Perform the indexing operation
		err := si.indexFiles("/")
		// Reset the indexing flag to indicate that indexing has finished
		si.inProgress = false
		// Update the LastIndexed time
		si.LastIndexed = time.Now()
		if err != nil {
			log.Printf("Error during indexing: %v", err)
		}
		if si.NumFiles+si.NumDirs > 0 {
			timeIndexedInSeconds := int(time.Since(startTime).Seconds())
			log.Println("Successfully indexed files.")
			log.Printf("Time spent indexing: %v seconds\n", timeIndexedInSeconds)
			log.Printf("Files found: %v\n", si.NumFiles)
			log.Printf("Directories found: %v\n", si.NumDirs)
		}
		// Sleep for the specified interval
		time.Sleep(time.Duration(intervalMinutes) * time.Minute)
	}
}

// Define a function to recursively index files and directories
func (si *Index) indexFiles(adjustedPath string) error {
	realPath := strings.TrimRight(si.Root, "/") + adjustedPath

	// Open the directory
	dir, err := os.Open(realPath)
	if err != nil {
		si.RemoveDirectory(adjustedPath) // Remove if it can't be opened
		return err
	}
	defer dir.Close()

	dirInfo, err := dir.Stat()
	if err != nil {
		return err
	}

	// Skip directories that haven't been modified since the last index
	if dirInfo.ModTime().Before(si.LastIndexed) {
		return nil
	}

	// Read directory contents
	files, err := dir.Readdir(-1)
	if err != nil {
		return err
	}

	var totalSize int64
	var numDirs, numFiles int
	fileInfos := map[string]*FileInfo{}
	dirInfos := map[string]*FileInfo{}
	combinedPath := adjustedPath + "/"
	if adjustedPath == "/" {
		combinedPath = "/"
	}

	// Process each file and directory in the current directory
	for _, file := range files {
		itemInfo := &FileInfo{
			ModTime: file.ModTime(),
		}
		if file.IsDir() {
			itemInfo.Name = file.Name()
			itemInfo.Path = combinedPath + file.Name()
			// Recursively index the subdirectory
			err := si.indexFiles(itemInfo.Path)
			if err != nil {
				log.Printf("Failed to index directory %s: %v", itemInfo.Path, err)
				continue
			}
			// Fetch the metadata for the subdirectory after indexing
			subDirInfo, exists := si.GetMetadataInfo(itemInfo.Path, true)
			if exists {
				itemInfo.Size = subDirInfo.Size
				totalSize += subDirInfo.Size // Add subdirectory size to the total
			}
			dirInfos[itemInfo.Name] = itemInfo
			numDirs++
		} else {
			itemInfo.Type = "blob"
			itemInfo.Name = file.Name()
			// Process a file
			itemInfo.Size = file.Size()
			_ = itemInfo.detectType(combinedPath+file.Name(), true, false, false)
			fileInfos[itemInfo.Name] = itemInfo
			totalSize += itemInfo.Size
			numFiles++
		}
	}

	// Create FileInfo for the current directory
	dirFileInfo := &FileInfo{
		Path:    adjustedPath,
		Files:   fileInfos,
		Dirs:    dirInfos,
		Size:    totalSize,
		ModTime: dirInfo.ModTime(),
	}

	// Update the current directory metadata in the index
	si.UpdateMetadata(dirFileInfo)
	si.NumDirs += numDirs
	si.NumFiles += numFiles

	return nil
}

func (si *Index) makeIndexPath(subPath string) string {
	if strings.HasPrefix(subPath, "./") {
		subPath = strings.TrimPrefix(subPath, ".")
	}
	if strings.HasPrefix(subPath, ".") || si.Root == subPath {
		return "/"
	}
	// clean path
	subPath = strings.TrimSuffix(subPath, "/")
	// remove index prefix
	adjustedPath := strings.TrimPrefix(subPath, si.Root)
	// remove trailing slash
	adjustedPath = strings.TrimSuffix(adjustedPath, "/")
	if !strings.HasPrefix(adjustedPath, "/") {
		adjustedPath = "/" + adjustedPath
	}
	return adjustedPath
}

//func getParentPath(path string) string {
//	// Trim trailing slash for consistency
//	path = strings.TrimSuffix(path, "/")
//	if path == "" || path == "/" {
//		return "" // Root has no parent
//	}
//
//	lastSlash := strings.LastIndex(path, "/")
//	if lastSlash == -1 {
//		return "/" // Parent of a top-level directory
//	}
//	return path[:lastSlash]
//}

func (si *Index) recursiveUpdateDirSizes(parentDir string, childInfo *FileInfo, previousSize int64) {
	childDirName := filepath.Base(childInfo.Path)
	if parentDir == childDirName {
		return
	}
	dir, exists := si.GetMetadataInfo(parentDir, true)
	if !exists {
		return
	}
	dir.Dirs[childDirName] = childInfo
	newSize := dir.Size - previousSize + childInfo.Size
	dir.Size += newSize
	si.UpdateMetadata(dir)
	dir, _ = si.GetMetadataInfo(parentDir, true)
	si.recursiveUpdateDirSizes(filepath.Dir(parentDir), dir, newSize)
}
