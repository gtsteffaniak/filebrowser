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
	LastIndexed time.Time
	mu          sync.RWMutex
}

var (
	rootPath string = "/srv"
	indexes  []*Index
)

func GetIndex(root string) *Index {
	for _, index := range indexes {
		if index.Root == root {
			return index
		}
	}
	return &Index{}
}
func InitializeIndex(intervalMinutes uint32, schedule bool) {
	// Initialize the index
	if settings.Config.Server.Root != "" {
		rootPath = settings.Config.Server.Root
	}
	indexes = []*Index{
		&Index{
			Root:        rootPath,
			Directories: make(map[string]Directory), // Initialize the map
			NumDirs:     0,
			NumFiles:    0,
			inProgress:  false,
		},
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
		// Set the indexing flag to indicate that indexing is in progress
		si.inProgress = true
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
	// Check if the current directory has been modified since the last indexing
	path = strings.TrimSuffix(path, "/")
	dir, err := os.Open(path)
	if err != nil {
		adjustedPath := makeIndexPath(path, si.Root)
		// Directory must have been deleted, remove it from the index
		delete(si.Directories, adjustedPath)

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
	fileList := []File{}
	for _, file := range files {
		newFile := File{
			Name:  file.Name(),
			IsDir: file.IsDir(),
		}
		fileList = append(fileList, newFile)
	}
	dir.Close()
	si.PrepAndInsert(fileList, path)
	return nil
}

func (si *Index) PrepAndInsert(fileList []File, path string) {
	adjustedPath := makeIndexPath(path, si.Root)
	// Create a buffer for the directory
	var buffer bytes.Buffer
	// Iterate over the files and directories
	for _, f := range fileList {
		// Assuming si.Insert takes a pointer to bytes.Buffer
		si.Insert(adjustedPath, f.Name, f.IsDir, &buffer)
		if f.IsDir {
			err := si.indexFiles(path + "/" + f.Name)
			if err != nil {
				log.Printf("Could not index \"%v\": %v \n", path, err)
			}
		}
	}

	// Get the directory from the map
	directory := si.Directories[adjustedPath]
	// Store the buffer in the directory's Files field
	directory.Files = buffer.String()
	si.Directories[adjustedPath] = directory
}

func (si *Index) Insert(path string, fileName string, isDir bool, buffer *bytes.Buffer) {
	si.mu.Lock()
	defer si.mu.Unlock()
	if isDir {
		if _, exists := si.Directories[path]; !exists {
			si.NumDirs++
			subDirectory := Directory{} // No need for Name here
			// Add or update the directory in the map
			if path != "" {
				si.Directories[path+"/"+fileName] = subDirectory
			} else {
				si.Directories[fileName] = subDirectory
			}
		}
	} else {
		// Use the buffer for this directory to concatenate file names
		buffer.WriteString(fileName + ";")
		si.NumFiles++
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
