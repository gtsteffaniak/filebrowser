package index

import (
	"log"
	"os"
	"strings"
	"sync"
	"time"

	"github.com/gtsteffaniak/filebrowser/settings"
)

const (
	maxIndexSize = 1000
)

type TrieNode struct {
	Children map[string]*TrieNode
	IsDir    bool
	mutex    sync.RWMutex
}

type Index struct {
	Root *TrieNode
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
		Root: &TrieNode{Children: make(map[string]*TrieNode)},
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
func indexFiles(path string, node *TrieNode, numFiles *int, numDirs *int) error {
	// Check if the current directory has been modified since last indexing

	dir, err := os.Open(path)
	if err != nil {
		// directory must have been deleted, remove from index
		node.mutex.Lock()
		delete(node.Children, path)
		node.mutex.Unlock()
	}
	defer dir.Close()
	dirInfo, err := dir.Stat()
	if err != nil {
		return err
	}
	log.Println(dirInfo)
	// Compare the last modified time of the directory with the last indexed time
	if dirInfo.ModTime().Before(lastIndexed) {
		return nil
	}

	// Read the directory contents
	files, err := dir.Readdir(-1)
	if err != nil {
		return err
	}
	log.Println(files)

	// Iterate over the files and directories
	for _, file := range files {

		if file.IsDir() {
			*numDirs++
			addToIndex(node, path, file.Name(), true)
			err := indexFiles(path+"/"+file.Name(), node.Children[path], numFiles, numDirs) // recursive
			if err != nil {
				log.Println("Could not index:", err)
			}
		} else {
			*numFiles++
			addToIndex(node, path, file.Name(), false)
		}
	}
	return nil
}

func addToIndex(node *TrieNode, path, fileName string, isDir bool) {
	node.mutex.Lock()
	defer node.mutex.Unlock()

	if path != "" {
		pathComponents := strings.Split(path, "/")
		currNode := node
		for _, component := range pathComponents {
			if currNode.Children[component] == nil {
				currNode.Children[component] = &TrieNode{Children: make(map[string]*TrieNode)}
			}
			currNode = currNode.Children[component]
		}
		currNode.IsDir = true
	}

	if isDir {
		if node.Children[fileName] == nil {
			node.Children[fileName] = &TrieNode{Children: make(map[string]*TrieNode)}
		}
		node.Children[fileName].IsDir = true
	} else {
		if node.Children[fileName] == nil {
			node.Children[fileName] = &TrieNode{Children: make(map[string]*TrieNode)}
		}
		node.Children[fileName].IsDir = false
	}
}
