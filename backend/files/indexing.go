package files

import (
	"bytes"
	"log"
	"os"
	"path/filepath"
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
	quickList   []File
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
	log.Printf("Indexing Files...")
	log.Printf("Configured to run every %v minutes", intervalMinutes)
	log.Printf("Indexing from root: %s", si.Root)
	for {
		startTime := time.Now()
		// Set the indexing flag to indicate that indexing is in progress
		si.resetCount()
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
<<<<<<< HEAD
	adjustedPath := si.makeIndexPath(path, true)
=======
	adjustedPath := si.makeIndexPath(path, false)
>>>>>>> main
	dir, err := os.Open(path)
	if err != nil {
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
		return err
	}
	dir.Close()
	si.UpdateQuickList(files)
	si.InsertFiles(path)
	// done separately for memory efficiency on recursion
	si.InsertDirs(path)
	return nil
}

func (si *Index) InsertFiles(path string) {
<<<<<<< HEAD
	adjustedPath := si.makeIndexPath(path, false)
=======
	adjustedPath := si.makeIndexPath(path, true)
>>>>>>> main
	subDirectory := Directory{}
	buffer := bytes.Buffer{}

	for _, f := range si.GetQuickList() {
		if !f.IsDir {
			buffer.WriteString(f.Name + ";")
			si.UpdateCount("files")
		}
	}
	// Use GetMetadataInfo and SetFileMetadata for safer read and write operations
	subDirectory.Files = buffer.String()
	si.SetDirectoryInfo(adjustedPath, subDirectory)
}

func (si *Index) InsertDirs(path string) {
	for _, f := range si.GetQuickList() {
		if f.IsDir {
			adjustedPath := si.makeIndexPath(path, false)
			if _, exists := si.Directories[adjustedPath]; exists {
				si.UpdateCount("dirs")
				// Add or update the directory in the map
				if adjustedPath == "/" {
					si.SetDirectoryInfo("/"+f.Name, Directory{})
				} else {
					si.SetDirectoryInfo(adjustedPath+"/"+f.Name, Directory{})
				}
			}
			err := si.indexFiles(path + "/" + f.Name)
			if err != nil {
				if err.Error() == "invalid argument" {
					log.Printf("Could not index \"%v\": %v \n", path, "Permission Denied")
				} else {
					log.Printf("Could not index \"%v\": %v \n", path, err)
				}
			}
		}
	}
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
	return adjustedPath
}
