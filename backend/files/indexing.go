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
	adjustedPath := si.makeIndexPath(path)

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
	fileInfos := map[string]*FileInfo{}
	dirInfos := map[string]*FileInfo{}

	var totalSize int64
	var numDirs, numFiles int

	for _, item := range files {
		parentInfo := &FileInfo{
			Size:    item.Size(),
			ModTime: item.ModTime(),
			Path:    adjustedPath,
		}
		if item.IsDir() {
			parentInfo.Type = "directory"
		}
		childInfo, err := si.InsertInfo(path, parentInfo, item.Name())
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
	si.UpdateFileMetadata(adjustedPath, dirFileInfo)
	// Add directory to index
	si.mu.Lock()
	si.NumDirs += numDirs
	si.NumFiles += numFiles
	si.mu.Unlock()
	return nil
}

// InsertInfo function to handle adding a file or directory into the index
func (si *Index) InsertInfo(parentPath string, file *FileInfo, name string) (*FileInfo, error) {
	filePath := filepath.Join(parentPath, name)

	// Check if it's a directory and recursively index it
	if file.Type == "directory" {
		// Recursively index directory
		err := si.indexFiles(filePath)
		if err != nil {
			return nil, err
		}
		si.UpdateFileMetadata(parentPath, file)
		return file, nil
	}
	// Create FileInfo for regular files
	fileInfo := &FileInfo{
		Name:    name,
		Size:    file.Size,
		ModTime: file.ModTime,
	}

	return fileInfo, nil
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
