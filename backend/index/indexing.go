package index

import (
	"log"
	"os"
	"strings"
	"sync"
	"time"

	"github.com/gtsteffaniak/filebrowser/settings"
)

type Index struct {
	Root        string
	Metadata    map[string]meta
	Files       []string
	Mutex       sync.RWMutex
	LastIndexed time.Time
}

var (
	index = Index{}
)

type meta struct {
	LastUpdated int
	Size        int
}

func GetIndex(root string) *Index {
	root = strings.TrimSuffix(root, "/")
	log.Println("getting index for ", root)
	return &index
}

func Initialize(intervalMinutes uint32) {
	// Initialize the index
	index = Index{
		Root:  strings.TrimSuffix(settings.GlobalConfiguration.Server.Root, "/"),
		Files: []string{},
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
		index.Files = slices.Compact(index.Files)
		time.Sleep(time.Duration(intervalMinutes) * time.Minute)
	}
}

// Define a function to recursively index files and directories
func (si *Index) indexFiles(path string, numFiles *int, numDirs *int) error {
	// Check if the current directory has been modified since last indexing
	path = strings.TrimSuffix(path, "/")
	// Check if the current directory has been modified since last indexing

	dir, err := os.Open(path)
	if err != nil {
		// directory must have been deleted, remove from index
		//indexes.Dirs = removeFromSlice(indexes.Dirs, path)
		log.Println("error")
	}
	defer dir.Close()
	dirInfo, err := dir.Stat()
	if err != nil {
		return err
	}
	// Compare the last modified time of the directory with the last indexed time
	if dirInfo.ModTime().Before(si.LastIndexed) {
		return nil
	}

	// Read the directory contents
	files, err := dir.Readdir(-1)
	if err != nil {
		return err
	}
	// Iterate over the files and directories
	for _, file := range files {
		childNode := &TrieNode{
			Children: make(map[string]*TrieNode),
		}
		node.Children[path] = childNode
		if file.IsDir() {
			//si.addToIndex(path+"/"+file.Name(), "", numFiles, numDirs)
			err := si.indexFiles(path+"/"+file.Name(), numFiles, numDirs) // recursive
			if err != nil {
				log.Printf("Could not index \"%v\": %v", path, err)
			}
		} else {
			si.addToIndex(path, file.Name(), numFiles, numDirs)
		}
	}
	return nil
}

func (si *Index) addToIndex(path string, fileName string, numFiles *int, numDirs *int) {
	si.Mutex.Lock()
	defer si.Mutex.Unlock()
	path = strings.TrimPrefix(path, si.Root+"/")
	path = strings.TrimSuffix(path, "/")
	adjustedPath := path + "/" + fileName
	if path == si.Root {
		adjustedPath = fileName
	}
	if fileName != "" {
		*numFiles++
		si.Files = append(si.Files, adjustedPath)
	}
}
