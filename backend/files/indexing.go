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

type Index struct {
	Root              string
	Directories       map[string]Directory // Change from []Directory to map[string]Directory
	NumDirs           int
	NumFiles          int
	currentlyIndexing bool
	LastIndexed       time.Time
	mutex             sync.RWMutex
}

var (
	rootPath     string = "/srv"
	indexes      map[string]*Index
	indexesMutex sync.RWMutex
	lastIndexed  time.Time
)

func GetIndex(root string) *Index {
	indexesMutex.RLock()
	defer indexesMutex.RUnlock()
	index, exists := indexes[root]
	if exists {
		return index
	}
	return &Index{}
}

func InitializeIndex(intervalMinutes uint32, schedule bool) {
	// Initialize the index
	indexes = make(map[string]*Index)
	if settings.Config.Server.Root != "" {
		rootPath = settings.Config.Server.Root
	}
	indexes[rootPath] = &Index{
		Root:              rootPath,
		Directories:       make(map[string]Directory), // Initialize the map
		NumDirs:           0,
		NumFiles:          0,
		currentlyIndexing: false,
	}
	if schedule {
		go indexingScheduler(intervalMinutes)
	}
}

func indexingScheduler(intervalMinutes uint32) {
	si := GetIndex(rootPath)
	log.Printf("Indexing Files...")
	log.Printf("Configured to run every %v minutes", intervalMinutes)
	log.Printf("Indexing from root: %s", si.Root)
	for {
		startTime := time.Now()
		// Check if the read lock is held by any goroutine
		if si.currentlyIndexing {
			continue
		}
		si.mutex.Lock()
		si.currentlyIndexing = true
		si.mutex.Unlock()

		err := si.indexFiles(si.Root)
		if err != nil {
			log.Printf("Error during indexing: %v", err)
		}
		si.mutex.Lock()
		si.currentlyIndexing = false
		si.LastIndexed = time.Now()
		si.mutex.Unlock()
		if si.NumFiles+si.NumDirs > 0 {
			timeIndexedInSeconds := int(time.Since(startTime).Seconds())
			log.Println("Successfully indexed files.")
			log.Printf("Time spent indexing: %v seconds\n", timeIndexedInSeconds)
			log.Printf("Files found: %v\n", si.NumFiles)
			log.Printf("Directories found: %v\n", si.NumDirs)
		}
		time.Sleep(time.Duration(intervalMinutes) * time.Minute)
	}
}

// Define a function to recursively index files and directories
func (si *Index) indexFiles(path string) error {
	// Check if the current directory has been modified since the last indexing
	path = strings.TrimSuffix(path, "/")
	dir, err := os.Open(path)

	if err != nil {
		// Directory must have been deleted, remove it from the index
		adjustedPath := strings.TrimPrefix(path, si.Root+"/")
		adjustedPath = strings.TrimSuffix(adjustedPath, "/")
		si.mutex.Lock()
		delete(si.Directories, adjustedPath)
		si.mutex.Unlock()
	}
	defer dir.Close()
	dirInfo, err := dir.Stat()
	if err != nil {
		return err
	}

	// Compare the last modified time of the directory with the last indexed time
	si.mutex.RLock()
	lastIndexed := si.LastIndexed
	si.mutex.RUnlock()
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

	// Create a buffer for the directory
	var buffer bytes.Buffer
	// Iterate over the files and directories
	for _, file := range files {
		si.Insert(adjustedPath, file.Name(), file.IsDir(), &buffer)
		if file.IsDir() {
			// Recursively index the directory
			err := si.indexFiles(path + "/" + file.Name())
			if err != nil {
				log.Printf("Could not index \"%v\": %v \n", path, err)
			}
		}
	}

	si.mutex.Lock()
	// Get the directory from the map
	directory := si.Directories[adjustedPath]
	// Store the buffer in the directory's Files field
	directory.Files = buffer.String()
	si.Directories[adjustedPath] = directory
	si.mutex.Unlock()

	return nil
}

func (si *Index) Insert(path string, fileName string, isDir bool, buffer *bytes.Buffer) {
	si.mutex.Lock()
	adjustedPath := strings.TrimPrefix(path, si.Root+"/")
	adjustedPath = strings.TrimSuffix(adjustedPath, "/")

	if isDir {
		if _, exists := si.Directories[adjustedPath]; !exists {
			si.NumDirs++
			subDirectory := Directory{} // No need for Name here
			// Add or update the directory in the map
			si.Directories[adjustedPath+"/"+fileName] = subDirectory
		}
		si.mutex.Unlock()

	} else {
		// Use the buffer for this directory to concatenate file names
		buffer.WriteString(fileName + ";")
		si.NumFiles++
	}

}
