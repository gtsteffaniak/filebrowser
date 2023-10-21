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
	Name  string
	Files []string
}

type Index struct {
	Dirs []Directory
}

var (
	rootPath    string = "/srv"
	indexes     Index
	indexMutex  sync.RWMutex
	lastIndexed time.Time
)

func GetIndex() *Index {
	return &indexes
}

func Initialize(intervalMinutes uint32) {
	// Initialize the index
	indexes = Index{
		Dirs: make([]Directory, 0, 1000),
	}
	rootPath = settings.GlobalConfiguration.Server.Root
	var numFiles, numDirs int
	log.Println("Indexing files...")
	lastIndexedStart := time.Now()
	// Call the function to index files and directories
	totalNumFiles, totalNumDirs, err := indexFiles(rootPath, &numFiles, &numDirs)
	if err != nil {
		log.Fatal(err)
	}
	lastIndexed = lastIndexedStart
	go indexingScheduler(intervalMinutes)
	log.Println("Successfully indexed files.")
	log.Println("Files found       :", totalNumFiles)
	log.Println("Directories found :", totalNumDirs)
}

func indexingScheduler(intervalMinutes uint32) {
	log.Printf("Indexing scheduler will run every %v minutes", intervalMinutes)
	for {
		time.Sleep(time.Duration(intervalMinutes) * time.Minute)
		var numFiles, numDirs int
		lastIndexedStart := time.Now()
		totalNumFiles, totalNumDirs, err := indexFiles(rootPath, &numFiles, &numDirs)
		if err != nil {
			log.Fatal(err)
		}
		lastIndexed = lastIndexedStart
		if totalNumFiles+totalNumDirs > 0 {
			log.Println("re-indexing found changes and updated the index.")
		}
	}
}

func removeFromSlice(slice []Directory, target string) []Directory {
	for i, d := range slice {
		if d.Name == target {
			// Swap the target element with the last element
			slice[i], slice[len(slice)-1] = slice[len(slice)-1], slice[i]
			// Resize the slice to exclude the last element
			slice = slice[:len(slice)-1]
			break // Exit the loop, assuming there's only one target element
		}
	}
	return slice
}

// Define a function to recursively index files and directories
func indexFiles(path string, numFiles *int, numDirs *int) (int, int, error) {
	// Check if the current directory has been modified since last indexing
	dir, err := os.Open(path)
	if err != nil {
		// directory must have been deleted, remove from index
		indexes.Dirs = removeFromSlice(indexes.Dirs, path)
	}
	defer dir.Close()
	dirInfo, err := dir.Stat()
	if err != nil {
		return *numFiles, *numDirs, err
	}
	// Compare the last modified time of the directory with the last indexed time
	if dirInfo.ModTime().Before(lastIndexed) {
		return *numFiles, *numDirs, nil
	}
	// Read the directory contents
	files, err := dir.Readdir(-1)
	if err != nil {
		return *numFiles, *numDirs, err
	}
	// Iterate over the files and directories
	for _, file := range files {
		if file.IsDir() {
			*numDirs++
			addToIndex(path, file.Name(), true)
			_, _, err := indexFiles(path+"/"+file.Name(), numFiles, numDirs) // recursive
			if err != nil {
				log.Println("Could not index :", err)
			}
		} else {
			*numFiles++
			addToIndex(path, file.Name(), false)
		}
	}
	return *numFiles, *numDirs, nil
}

func addToIndex(path string, fileName string, isDir bool) {
	indexMutex.Lock()
	defer indexMutex.Unlock()
	path = strings.TrimPrefix(path, rootPath+"/")
	path = strings.TrimSuffix(path, "/")
	adjustedPath := path + "/" + fileName
	if path == rootPath {
		adjustedPath = fileName
	}
	if isDir {
		indexes.Dirs = append(indexes.Dirs, Directory{Name: adjustedPath})
	} else {
		// Find the corresponding directory and append the file
		for i, dir := range indexes.Dirs {
			if dir.Name == path {
				indexes.Dirs[i].Files = append(indexes.Dirs[i].Files, fileName)
				return
			}
		}
	}
}
