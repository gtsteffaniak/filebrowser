package index

import (
	"log"
	"os"
	"sync"
	"time"

	"github.com/gtsteffaniak/filebrowser/settings"
)

type Directory struct {
	Name  string
	Dirs  []*Directory
	Files []string
}

type Index struct {
	Root  *Directory
	mutex sync.RWMutex
}

var (
	rootPath    string = "/srv"
	indexes     Index
	lastIndexed time.Time
)

func GetIndex() *Index {
	return &indexes
}

func Initialize(intervalMinutes uint32) {
	// Initialize the index
	indexes = Index{
		Root: &Directory{Dirs: []*Directory{}},
	}
	rootPath = settings.GlobalConfiguration.Server.Root
	var numFiles, numDirs int
	log.Println("Indexing files...")
	lastIndexedStart := time.Now()
	// Call the function to index files and directories
	err := indexFiles(rootPath, indexes.Root, &numFiles, &numDirs)
	if err != nil {
		log.Fatal(err)
	}
	lastIndexed = lastIndexedStart
	go indexingScheduler(intervalMinutes)
	log.Println("Successfully indexed files.")
	log.Println("Files found       :", numFiles)
	log.Println("Directories found :", numDirs)
}

func indexingScheduler(intervalMinutes uint32) {
	log.Printf("Indexing scheduler will run every %v minutes", intervalMinutes)
	for {
		time.Sleep(time.Duration(intervalMinutes) * time.Minute)
		var numFiles, numDirs int
		lastIndexedStart := time.Now()
		err := indexFiles(rootPath, indexes.Root, &numFiles, &numDirs)
		if err != nil {
			log.Fatal(err)
		}
		lastIndexed = lastIndexedStart
		if numFiles+numDirs > 0 {
			log.Println("re-indexing found changes and updated the index.")
		}
	}
}

// Define a function to recursively index files and directories
func indexFiles(path string, node *Directory, numFiles *int, numDirs *int) error {
	// Check if the current directory has been modified since last indexing

	dir, err := os.Open(path)
	if err != nil {
		// Directory must have been deleted, remove from index
		node.Dirs = removeFromSlice(node.Dirs, node)
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

	// Separate slices for directories and files
	node.Files = []string{}

	// Iterate over the files and directories
	for _, file := range files {
		// Check if it's a directory or a file
		if file.IsDir() {
			*numDirs++
			dirName := file.Name()
			subDirectory := &Directory{
				Name:  file.Name(),
				Dirs:  []*Directory{},
				Files: []string{},
			}
			node.Dirs = append(node.Dirs, subDirectory)
			// Recursively index the directory
			err := indexFiles(path+"/"+dirName, subDirectory, numFiles, numDirs)
			if err != nil {
				log.Printf("Could not index \"%v\": %v", path, err)
			}
		} else {
			node.Files = append(node.Files, file.Name())
			*numFiles++
		}
	}

	return nil
}

func removeFromSlice(slice []*Directory, target *Directory) []*Directory {
	for i, d := range slice {
		if d == target {
			// Remove the element at index i by slicing the slice
			slice = append(slice[:i], slice[i+1:]...)
			break
		}
	}
	return slice
}
