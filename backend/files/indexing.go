package files

import (
	"bytes"
	"log"
	"os"
	"strings"
	"sync"
	"time"
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
	rootPath   string = "/srv"
	indexMap          = make(map[string]*Index)
	indexMutex sync.Mutex
)

func InitializeIndex(intervalMinutes uint32, schedule bool) {
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
	fileList := []File{}
	for _, file := range files {
		newFile := File{
			Name:  file.Name(),
			IsDir: file.IsDir(),
		}
		fileList = append(fileList, newFile)
	}
	dir.Close()
	si.InsertFiles(fileList, path)
	// done separately for memory efficiency on recursion
	si.InsertDirs(fileList, path)
	return nil
}

func (si *Index) InsertFiles(fileList []File, path string) {
	adjustedPath := makeIndexPath(path, si.Root)
	var buffer bytes.Buffer
	subDirectory := Directory{}
	for _, f := range fileList {
		buffer.WriteString(f.Name + ";")
		si.UpdateCount("files")
	}
	// Use GetMetadataInfo and SetFileMetadata for safer read and write operations
	subDirectory.Files = buffer.String()
	si.SetDirectoryInfo(adjustedPath, subDirectory)
}

func (si *Index) InsertDirs(fileList []File, path string) {
	for _, f := range fileList {
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
