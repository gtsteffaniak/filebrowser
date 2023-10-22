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
	rootPath = strings.TrimSuffix(settings.GlobalConfiguration.Server.Root, "/")
	var numFiles, numDirs int
	log.Println("Indexing files...")
	lastIndexedStart := time.Now()
	// Call the function to index files and directories
	err := indexFiles(rootPath, &numFiles, &numDirs)
	if err != nil {
		log.Fatal(err)
	}
	lastIndexed = lastIndexedStart
	timeIndexedInSeconds := int(time.Since(lastIndexedStart).Seconds())
	go indexingScheduler(intervalMinutes)
	log.Println("Successfully indexed files.")
	log.Printf("Time spent indexing : %v seconds \n", timeIndexedInSeconds)
	log.Println("Files found         :", numFiles)
	log.Println("Directories found   :", numDirs)
}

func indexingScheduler(intervalMinutes uint32) {
	log.Printf("Indexing scheduler will run every %v minutes", intervalMinutes)
	for {
		time.Sleep(time.Duration(intervalMinutes) * time.Minute)
		var numFiles, numDirs int
		lastIndexedStart := time.Now()
		err := indexFiles(rootPath, &numFiles, &numDirs)
		if err != nil {
			log.Fatal(err)
		}
		lastIndexed = lastIndexedStart
		if numFiles+numDirs > 0 {
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
func indexFiles(path string, numFiles *int, numDirs *int) error {
	path = strings.TrimSuffix(path, "/")
	// Check if the current directory has been modified since last indexing
	dir, err := os.Open(path)
	if err != nil {
		// directory must have been deleted, remove from index
		indexes.Dirs = removeFromSlice(indexes.Dirs, path)
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
	// Iterate over the files and directories
	for _, file := range files {
		if file.IsDir() {
			addToIndex(path+"/"+file.Name(), "", numFiles, numDirs)
			err := indexFiles(path+"/"+file.Name(), numFiles, numDirs) // recursive
			if err != nil {
				errMsg := err.Error()
				if errMsg == "invalid argument" {
					errMsg = "Permission Denied"
				}
				log.Printf("Could not index \"%v\" : %v", path+"/"+file.Name(), errMsg)
			}
		} else {
			addToIndex(path, file.Name(), numFiles, numDirs)
		}
	}
	return nil
}

func addToIndex(path string, fileName string, numFiles *int, numDirs *int) {
	indexMutex.Lock()
	defer indexMutex.Unlock()
	path = strings.TrimPrefix(path, rootPath+"/")
	path = strings.TrimSuffix(path, "/")
	// Flag to check if the directory exists in the slice
	exists := false
	// Iterate over the slice to find the directory
	for i, dir := range indexes.Dirs {
		if dir.Name == path {
			// If the directory exists, append the file to it
			if fileName != "" {
				*numFiles++
				indexes.Dirs[i].Files = append(indexes.Dirs[i].Files, fileName)
			}
			exists = true
			break
		}
	}
	// If the directory doesn't exist, add it to the slice
	if !exists {
		*numDirs++
		if fileName == "" {
			indexes.Dirs = append(indexes.Dirs, Directory{Name: path, Files: []string{}})
		} else {
			*numFiles++
			indexes.Dirs = append(indexes.Dirs, Directory{Name: path, Files: []string{fileName}})
		}
	}
}
