package files

import (
	"fmt"
	"log"
	"os"
	"strings"
	"sync"
	"time"

	"github.com/gtsteffaniak/filebrowser/settings"
)

type Index struct {
	Root        string
	Directories map[string]*FileInfo // top-level master list of directories
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
		if si.Directories["/"].Path == "" {
			si.Directories["/"].Path = "/" // first time indexing needs path set
		}
		// Perform the indexing operation
		_, err := si.indexFiles(si.Directories["/"]) // start at root adjusted path
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
func (si *Index) indexFiles(parentInfo *FileInfo) (*FileInfo, error) {
	realPath := strings.TrimRight(si.Root, "/") + parentInfo.Path + parentInfo.Name
	if parentInfo.Path == "" {
		realPath = si.Root
	}
	// Open the directory
	dir, err := os.Open(realPath)
	if err != nil {
		// If the directory can't be opened (e.g., deleted), remove it from the index
		si.RemoveDirectory(parentInfo.Path)
		return parentInfo, err
	}
	defer dir.Close()

	dirInfo, err := dir.Stat()
	if err != nil {
		return parentInfo, err
	}

	// Check if the directory is already up-to-date
	if dirInfo.ModTime().Before(si.LastIndexed) {
		return parentInfo, nil
	}

	// Read directory contents
	files, err := dir.Readdir(-1)
	if err != nil {
		return parentInfo, err
	}

	// Recursively process files and directories
	fileInfos := map[string]*FileInfo{}
	dirInfos := map[string]*FileInfo{}

	var totalSize int64
	var numDirs, numFiles int

	for _, item := range files {
		itemInfo := &FileInfo{
			Name:    item.Name(),
			Size:    item.Size(),
			ModTime: item.ModTime(),
			Path:    parentInfo.Path + parentInfo.Name,
		}
		fmt.Println("Indexing: ", itemInfo.Path, itemInfo.Name)
		if item.IsDir() {
			itemInfo.Type = "directory"
		}
		childInfo, err := si.InsertInfo(itemInfo)
		if err != nil {
			// Log error, but continue processing other files
			continue
		}

		// Accumulate directory size and items
		totalSize += childInfo.Size
		if childInfo.Type == "directory" {
			dirInfos[item.Name()] = childInfo
			numDirs++
		} else {
			fmt.Println("adding file", childInfo.Path, childInfo.Name)
			_ = childInfo.detectType(childInfo.Name, true, false, false)
			fileInfos[item.Name()] = childInfo
			numFiles++
		}
	}

	// Create FileInfo for the current directory
	dirFileInfo := &FileInfo{
		Name:      dirInfo.Name(),
		Files:     fileInfos,
		Dirs:      dirInfos,
		Size:      totalSize,
		ModTime:   dirInfo.ModTime(),
		CacheTime: time.Now(),
		Type:      "directory",
		NumDirs:   numDirs,
		NumFiles:  numFiles,
	}
	si.UpdateFileMetadata(parentInfo.Path, dirFileInfo)
	// Add directory to index
	si.mu.Lock()
	si.NumDirs += numDirs
	si.NumFiles += numFiles
	si.mu.Unlock()
	return dirFileInfo, nil
}

// InsertInfo function to handle adding a file or directory into the index
func (si *Index) InsertInfo(itemInfo *FileInfo) (*FileInfo, error) {
	// Check if it's a directory and recursively index it
	if itemInfo.Type == "directory" {
		// Recursively index directory
		fullInfo, err := si.indexFiles(itemInfo)
		if err != nil {
			return nil, err
		}
		si.UpdateFileMetadata(itemInfo.Path, fullInfo)
		return itemInfo, nil
	}
	return itemInfo, nil
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
