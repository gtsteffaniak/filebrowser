package index

import (
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
	Files    []string
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
	index       Index
	lastIndexed time.Time
)

func GetIndex(root string) *Index {
	root = strings.TrimSuffix(root, "/")
	log.Println("getting index for ", root)
	return &index
}

func Initialize(intervalMinutes uint32) {
	// Initialize the index
	index = Index{
		Root:              strings.TrimSuffix(settings.GlobalConfiguration.Server.Root, "/"),
		Directories:       []Directory{},
		NumDirs:           0,
		NumFiles:          0,
		currentlyIndexing: false,
	}
	go indexingScheduler(intervalMinutes)
}

func indexingScheduler(intervalMinutes uint32) {
	log.Printf("Indexing Files...")
	log.Printf("Configured to run every %v minutes", intervalMinutes)
	log.Printf("Indexing from root: %s", index.Root)
	for {
		startTime := time.Now()
		// Check if the read lock is held by any goroutine
		if index.currentlyIndexing {
			continue
		}
		err := index.indexFiles(index.Root)
		if err != nil {
			log.Printf("Error during indexing: %v", err)
		}
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

	si.currentlyIndexing = true
	// Check if the current directory has been modified since last indexing
	path = strings.TrimSuffix(path, "/")

	dir, err := os.Open(path)

	if err != nil {
		// Directory must have been deleted, remove from index
		si.Directories = removeFromSlice(si.Directories, path)
	}
	defer dir.Close()
	dirInfo, err := dir.Stat()
	if err != nil {
		si.currentlyIndexing = false
		return err
	}

	// Compare the last modified time of the directory with the last indexed time
	if dirInfo.ModTime().Before(lastIndexed) {
		si.currentlyIndexing = false
		return nil
	}

	// Read the directory contents
	files, err := dir.Readdir(-1)
	if err != nil {
		si.currentlyIndexing = false
		return err
	}
	adjustedPath := strings.TrimPrefix(path, si.Root+"/")
	adjustedPath = strings.TrimSuffix(adjustedPath, "/")
	keyVal := -1
	//exists := false
	for k, v := range index.Directories {
		if v.Name == adjustedPath {
			//exists := true
			keyVal = k
			continue
		}
	}
	// Iterate over the files and directories
	for _, file := range files {
		si.Insert(path, file.Name(), file.IsDir(), keyVal)
	}
	si.currentlyIndexing = false
	return nil
}

func (si *Index) Insert(path string, fileName string, isDir bool, key int) {
	adjustedPath := strings.TrimPrefix(path, si.Root+"/")
	adjustedPath = strings.TrimSuffix(adjustedPath, "/")
	// Check if it's a directory or a file
	if isDir {
		si.NumDirs++
		subDirectory := Directory{
			Name: adjustedPath + "/" + fileName,
		}
		si.mutex.Lock()
		index.Directories = append(index.Directories, subDirectory)
		si.mutex.Unlock()

		// Recursively index the directory
		err := index.indexFiles(path + "/" + fileName)
		if err != nil {
			log.Printf("Could not index \"%v\": %v", path, err)
		}
	} else {
		if key != -1 {
			si.mutex.Lock()
			index.Directories[key].Files = append(index.Directories[key].Files, fileName)
			si.mutex.Unlock()
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
