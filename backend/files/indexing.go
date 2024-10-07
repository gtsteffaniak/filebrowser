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
	Directories map[string]FileInfo
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
		err := si.indexFiles(si.Root)
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
func (si *Index) indexFiles(path string) error {
	// Ensure path is cleaned and normalized
	adjustedPath := si.makeIndexPath(path, true)

	// Open the directory
	dir, err := os.Open(path)
	if err != nil {
		// If the directory can't be opened (e.g., deleted), remove it from the index
		si.RemoveDirectory(adjustedPath)
		return err
	}
	defer dir.Close()

	dirInfo, err := dir.Stat()
	if err != nil {
		return err
	}

	// Check if the directory is already up-to-date
	if dirInfo.ModTime().Before(si.LastIndexed) {
		return nil
	}

	// Read directory contents
	files, err := dir.Readdir(-1)
	if err != nil {
		return err
	}

	// Recursively process files and directories
	fileInfos := []*FileInfo{}
	var totalSize int64
	var numDirs, numFiles int

	for _, file := range files {
		parentInfo := &FileInfo{
			Name:    file.Name(),
			Size:    file.Size(),
			ModTime: file.ModTime(),
			IsDir:   file.IsDir(),
		}
		childInfo, err := si.InsertInfo(path, parentInfo)
		if err != nil {
			// Log error, but continue processing other files
			continue
		}

		// Accumulate directory size and items
		totalSize += childInfo.Size
		if childInfo.IsDir {
			numDirs++
		} else {
			numFiles++
		}
		_ = childInfo.detectType(path, true, false, false)
		fileInfos = append(fileInfos, childInfo)
	}

	// Create FileInfo for the current directory
	dirFileInfo := &FileInfo{
		Items:     fileInfos,
		Name:      filepath.Base(path),
		Size:      totalSize,
		ModTime:   dirInfo.ModTime(),
		CacheTime: time.Now(),
		IsDir:     true,
		NumDirs:   numDirs,
		NumFiles:  numFiles,
	}

	// Add directory to index
	si.mu.Lock()
	si.Directories[adjustedPath] = *dirFileInfo
	si.NumDirs += numDirs
	si.NumFiles += numFiles
	si.mu.Unlock()
	return nil
}

// InsertInfo function to handle adding a file or directory into the index
func (si *Index) InsertInfo(parentPath string, file *FileInfo) (*FileInfo, error) {
	filePath := filepath.Join(parentPath, file.Name)

	// Check if it's a directory and recursively index it
	if file.IsDir {
		// Recursively index directory
		err := si.indexFiles(filePath)
		if err != nil {
			return nil, err
		}

		// Return directory info from the index
		adjustedPath := si.makeIndexPath(filePath, true)
		si.mu.RLock()
		dirInfo := si.Directories[adjustedPath]
		si.mu.RUnlock()
		return &dirInfo, nil
	}

	// Create FileInfo for regular files
	fileInfo := &FileInfo{
		Path:    filePath,
		Name:    file.Name,
		Size:    file.Size,
		ModTime: file.ModTime,
		IsDir:   false,
	}

	return fileInfo, nil
}

func (si *Index) makeIndexPath(subPath string, isDir bool) string {
	if si.Root == subPath {
		return "/"
	}
	// clean path
	subPath = strings.TrimSuffix(subPath, "/")
	// remove index prefix
	adjustedPath := strings.TrimPrefix(subPath, si.Root)
	// remove trailing slash
	adjustedPath = strings.TrimSuffix(adjustedPath, "/")
	// add leading slash for root of index
	if adjustedPath == "" {
		adjustedPath = "/"
	} else if !isDir {
		adjustedPath = filepath.Dir(adjustedPath)
	}
	if !strings.HasPrefix(adjustedPath, "/") {
		adjustedPath = "/" + adjustedPath
	}
	return adjustedPath
}
