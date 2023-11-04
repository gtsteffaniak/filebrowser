package index

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
	Name     string
	Metadata map[string]meta
	Files    string
}
type meta struct {
	LastUpdated int
	Size        int
}

type Index struct {
	Root              string
	Directories       []Directory
	NumDirs           int
	NumFiles          int
	currentlyIndexing bool
	LastIndexed       time.Time
	mutex             sync.RWMutex
}

var (
	rootPath    string = "/srv"
	indexes     map[string]*Index
	lastIndexed time.Time
)

func GetIndex(root string) *Index {
	index, exists := indexes[root]
	if exists {
		return index
	}
	return &Index{}
}

func Initialize(intervalMinutes uint32, schedule bool) {
	// Initialize the index
	indexes = make(map[string]*Index)
	if settings.Config.Server.Root != "" {
		rootPath = settings.Config.Server.Root
	}
	indexes[rootPath] = &Index{
		Root:              rootPath,
		Directories:       []Directory{},
		NumDirs:           0,
		NumFiles:          0,
		currentlyIndexing: false,
	}
	if schedule {
		go indexingScheduler(intervalMinutes)
	}
}

func indexingScheduler(intervalMinutes uint32) {
	index := GetIndex(rootPath)
	log.Printf("Indexing Files...")
	log.Printf("Configured to run every %v minutes", intervalMinutes)
	log.Printf("Indexing from root: %s", index.Root)
	for {
		startTime := time.Now()
		// Check if the read lock is held by any goroutine
		if index.currentlyIndexing {
			continue
		}
		index.currentlyIndexing = true
		err := index.indexFiles(index.Root)
		if err != nil {
			log.Printf("Error during indexing: %v", err)
		}
		index.currentlyIndexing = false
		index.LastIndexed = time.Now()
		if index.NumFiles+index.NumDirs > 0 {
			timeIndexedInSeconds := int(time.Since(startTime).Seconds())
			log.Println("Successfully indexed files.")
			log.Printf("Time spent indexing: %v seconds\n", timeIndexedInSeconds)
			log.Printf("Files found: %v\n", index.NumFiles)
			log.Printf("Directories found: %v\n", index.NumDirs)
		}
		time.Sleep(time.Duration(intervalMinutes) * time.Minute)
	}
}

// Define a function to recursively index files and directories
func (si *Index) indexFiles(path string) error {
	// Check if the current directory has been modified since last indexing
	path = strings.TrimSuffix(path, "/")
	dir, err := os.Open(path)
	if err != nil {
		// Directory must have been deleted, remove from the index
		si.Directories = removeFromSlice(si.Directories, path)
	}
	defer dir.Close()
	dirInfo, err := dir.Stat()
	if err != nil {
		return err
	}
	// Compare the last modified time of the directory with the last indexed time
	if dirInfo.ModTime().Before(lastIndexed) {
		return nil
	}
	// Read the directory contents
	files, err := dir.Readdir(-1)
	if err != nil {
		return err
	}
	adjustedPath := strings.TrimPrefix(path, si.Root+"/")
	adjustedPath = strings.TrimSuffix(adjustedPath, "/")
	keyVal := -1
	// Find the key for the directory
	for k, v := range si.Directories {
		if v.Name == adjustedPath {
			keyVal = k
			break
		}
	}
	// Create a buffer for the directory
	var buffer bytes.Buffer
	// Iterate over the files and directories
	for _, file := range files {
		si.Insert(path, file.Name(), file.IsDir(), keyVal, &buffer)
	}
	if keyVal != -1 {
		// Store the buffer in the directory's Files field
		si.Directories[keyVal].Files = buffer.String()
	}
	return nil
}

func (si *Index) Insert(path string, fileName string, isDir bool, key int, buffer *bytes.Buffer) {
	adjustedPath := strings.TrimPrefix(path, si.Root+"/")
	adjustedPath = strings.TrimSuffix(adjustedPath, "/")
	// Check if it's a directory or a file
	if isDir {
		si.NumDirs++
		subDirectory := Directory{
			Name: adjustedPath + "/" + fileName,
		} // 48
		si.mutex.Lock()
		si.Directories = append(si.Directories, subDirectory)
		si.mutex.Unlock()

		// Recursively index the directory
		err := si.indexFiles(path + "/" + fileName)
		if err != nil {
			log.Printf("Could not index \"%v\": %v \n", path, err)
		}
	} else {
		if key != -1 {
			// Use the buffer for this directory to concatenate file names
			buffer.WriteString(fileName)
			buffer.WriteString(";")
			si.NumFiles++
		}
	}
}

func removeFromSlice(slice []Directory, path string) []Directory {
	for i, d := range slice {
		if d.Name == path {
			// Remove the element at index i by slicing the slice
			slice = append(slice[:i], slice[i+1:]...)
			break
		}
	}
	return slice
}
