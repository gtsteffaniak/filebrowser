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
	Root        string
	Directories []Directory
	LastIndexed time.Time
	mutex       sync.RWMutex
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
		Root:        strings.TrimSuffix(settings.GlobalConfiguration.Server.Root, "/"),
		Directories: []Directory{},
	}
	go indexingScheduler(intervalMinutes)
}

func indexingScheduler(intervalMinutes uint32) {
	log.Printf("Indexing Files. This will occur as configured: Every %v minutes", intervalMinutes)
	for {
		var numFiles, numDirs int
		startTime := time.Now()
		log.Println(index.Root)
		err := index.indexFiles(index.Root, &numFiles, &numDirs)
		if err != nil {
			log.Fatal(err)
		}
		index.LastIndexed = time.Now()
		if numFiles+numDirs > 0 {
			timeIndexedInSeconds := int(time.Since(startTime).Seconds())
			log.Println("Successfully indexed files.")
			log.Printf("Time spent indexing : %v seconds \n", timeIndexedInSeconds)
			log.Println("Files found       :", numFiles)
			log.Println("Directories found :", numDirs)

		}
		time.Sleep(time.Duration(intervalMinutes) * time.Minute)
	}
}

// Define a function to recursively index files and directories
func (si *Index) indexFiles(path string, numFiles *int, numDirs *int) error {
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
		adjustedPath := strings.TrimPrefix(path, si.Root+"/")
		adjustedPath = strings.TrimSuffix(adjustedPath, "/")
		// Check if it's a directory or a file
		if file.IsDir() {
			*numDirs++
			subDirectory := Directory{
				Name: adjustedPath + "/" + file.Name(),
			}
			index.Directories = append(index.Directories, subDirectory)
			// Recursively index the directory
			err := index.indexFiles(path+"/"+file.Name(), numFiles, numDirs)
			if err != nil {
				log.Printf("Could not index \"%v\": %v", path, err)
			}
		} else {
			for k, v := range index.Directories {

				if v.Name == adjustedPath {
					index.Directories[k].Files = append(index.Directories[k].Files, file.Name())
					*numFiles++
					continue
				}
			}
		}
	}

	return nil
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
