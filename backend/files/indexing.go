package files

import (
	"bytes"
	"log"
	"os"
	"strings"
	"sync"
	"time"

	"github.com/gtsteffaniak/filebrowser/settings"
)

type Directory struct {
	Metadata map[string]FileInfo
	Files    string
}
type File struct {
	Name  string
	IsDir bool
}

type Index struct {
	Root        string
	Directories map[string]Directory
	NumDirs     int
	NumFiles    int
	inProgress  bool
	buffer      bytes.Buffer
	quickList   []File
	LastIndexed time.Time
	mu          sync.RWMutex
}

var (
	rootPath string = "/srv"
	indexes  []*Index
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
	log.Printf("Indexing Files...")
	log.Printf("Configured to run every %v minutes", intervalMinutes)
	log.Printf("Indexing from root: %s", si.Root)
	for {
		startTime := time.Now()
		// Set the indexing flag to indicate that indexing is in progress
		si.inProgress = true
		// Perform the indexing operation
		err := si.indexFiles(si.Root)
		si.quickList = []File{}
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
	// Check if the current directory has been modified since the last indexing
	path = strings.TrimSuffix(path, "/")
	dir, err := os.Open(path)
	if err != nil {
		adjustedPath := makeIndexPath(path, si.Root)
		// Directory must have been deleted, remove it from the index
		si.RemoveDirectory(adjustedPath)
	}
	dirInfo, err := dir.Stat()
	if err != nil {
		dir.Close()
		return err
	}

	// Compare the last modified time of the directory with the last indexed time
	lastIndexed := si.LastIndexed
	if dirInfo.ModTime().Before(lastIndexed) {
		dir.Close()
		return nil
	}

	// Read the directory contents
	files, err := dir.Readdir(-1)
	if err != nil {
		dir.Close()
		return err
	}
	si.UpdateQuickList(files)
	dir.Close()
	si.InsertFiles(path)
	// done separately for memory efficiency on recursion
	si.InsertDirs(path)
	return nil
}

//go:norace
func (si *Index) InsertFiles(path string) {
	adjustedPath := makeIndexPath(path, si.Root)
	subDirectory := Directory{}
	si.buffer = bytes.Buffer{}
	for _, f := range si.quickList {
		si.buffer.WriteString(f.Name + ";")
		si.UpdateCount("files")
	}
	// Use GetMetadataInfo and SetFileMetadata for safer read and write operations
	subDirectory.Files = si.buffer.String()
	si.SetDirectoryInfo(adjustedPath, subDirectory)
}

//go:norace
func (si *Index) InsertDirs(path string) {
	for _, f := range si.quickList {
		if f.IsDir {
			// Prevent data race
			si.UpdateCount("dirs")
			subDirectory := Directory{}
			if path != "" {
				si.SetDirectoryInfo(path+"/"+f.Name, subDirectory)
			} else {
				si.SetDirectoryInfo(f.Name, subDirectory)
			}
			err := si.indexFiles(path + "/" + f.Name)
			if err != nil {
				log.Printf("Could not index \"%v\": %v \n", path, err)
			}
		}
	}
}

func makeIndexPath(path string, root string) string {
	if path == root {
		return "/"
	}
	adjustedPath := strings.TrimPrefix(path, root+"/")
	adjustedPath = strings.TrimSuffix(adjustedPath, "/")
	if adjustedPath == "" {
		adjustedPath = "/"
	}
	return adjustedPath
}
