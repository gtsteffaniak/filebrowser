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
}

type Index struct {
	Root  *TrieNode
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
		delete(node.Children, path)
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
		childNode := &TrieNode{
			Children: make(map[string]*TrieNode),
		}
		if file.IsDir() {
			childNode.IsDir = true
			*numDirs++
			addToIndex(node, path, file.Name())
			node.Children[path] = childNode
			err := indexFiles(path+"/"+file.Name(), node.Children[path], numFiles, numDirs) // recursive
			if err != nil {
				log.Println("Could not index:", err)
			}
		} else {
			childNode.IsDir = false
			*numFiles++
			addToIndex(node, path, file.Name())
		}
	}
	return nil
}

func addToIndex(node *TrieNode, path, fileName string) {
	if path != "" {
		pathComponents := strings.Split(path, "/")
		for _, component := range pathComponents {
			if node.Children[component] == nil {
				node.Children[component] = &TrieNode{Children: make(map[string]*TrieNode)}
			}
			node = node.Children[component]
		}
		node.IsDir = true
	}

	if node.IsDir {
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
