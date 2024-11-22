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
	NumDirs     uint64
	NumFiles    uint64
	inProgress  bool
	LastIndexed time.Time
	mu          sync.RWMutex
}

var (
	rootPath     string = "/srv"
	indexes      []*Index
	indexesMutex sync.RWMutex
)

func InitializeIndex(enabled bool) {
	if enabled {
		go indexingScheduler(60)
	}
}

func indexingScheduler(intervalMinutes uint32) {
	if settings.Config.Server.Root != "" {
		rootPath = settings.Config.Server.Root
	}
	si := GetIndex(rootPath)
	firstRun := true
	for {
		startDirs := si.NumDirs
		startFiles := si.NumFiles
		si.NumDirs = 0
		si.NumFiles = 0
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
			log.Printf("Time Spent Indexing      : %v seconds\n", timeIndexedInSeconds)
			if firstRun {
				log.Printf("Files Found              : %v\n", si.NumFiles)
				log.Printf("Directories found        : %v\n", si.NumDirs)
			} else {
				log.Printf("Files Updated            : %v\n", si.NumFiles-startFiles)
				log.Printf("Directories Updated      : %v\n", si.NumDirs-startDirs)
			}

		}
		firstRun = false
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
		si.RemoveDirectory(adjustedPath) // Remove, must have been deleted
		return err
	}
	defer dir.Close()

	dirInfo, err := dir.Stat()
	if err != nil {
		return err
	}
	//
	//// Skip directories that haven't been modified since the last index
	//if dirInfo.ModTime().Before(si.LastIndexed) {
	//	return nil
	//}

	// Read directory contents
	files, err := dir.Readdir(-1)
	if err != nil {
		return err
	}

	var totalSize int64
	var numDirs, numFiles uint64
	fileInfos := []ReducedItem{}
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
			itemInfo := &ReducedItem{
				Name:    file.Name(),
				ModTime: file.ModTime(),
				Size:    file.Size(),
				Mode:    file.Mode(),
			}
			_ = itemInfo.detectType(combinedPath+file.Name(), true, false, false)
			fileInfos = append(fileInfos, *itemInfo)
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
