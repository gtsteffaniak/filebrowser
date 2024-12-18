package files

import (
	"fmt"
	"log"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"time"

	"github.com/gtsteffaniak/filebrowser/backend/settings"
	"github.com/gtsteffaniak/filebrowser/backend/utils"
)

type Index struct {
	Name                       string
	Root                       string
	Directories                map[string]*FileInfo
	NumDirs                    uint64
	NumFiles                   uint64
	NumDeleted                 uint64
	FilesChangedDuringIndexing bool
	currentSchedule            int
	assessment                 string
	indexingTime               int
	LastIndexed                time.Time
	SmartModifier              time.Duration
	mu                         sync.RWMutex
	scannerMu                  sync.Mutex
}

var (
	indexes      map[string]*Index
	indexesMutex sync.RWMutex
)

func InitializeIndex(Source settings.Source) {
	if !Source.Index.Disabled {
		time.Sleep(time.Second)
		si := GetIndex(Source.Path)
		log.Println("Initializing index and assessing file system complexity")
		si.RunIndexing("/", false)
		go si.setupIndexingScanners()
	}
}

// Define a function to recursively index files and directories
func (si *Index) indexDirectory(adjustedPath string, quick, recursive bool) error {
	realPath := strings.TrimRight(si.Root, "/") + adjustedPath

	// Open the directory
	dir, err := os.Open(realPath)
	if err != nil {
		si.RemoveDirectory(adjustedPath) // Remove, must have been deleted
		return err
	}
	defer dir.Close()

	dirInfo, err := dir.Stat()
	if err != nil {
		return err
	}
	combinedPath := adjustedPath + "/"
	if adjustedPath == "/" {
		combinedPath = "/"
	}
	// get whats currently in cache
	si.mu.RLock()
	cacheDirItems := []ItemInfo{}
	modChange := true // default to true
	cachedDir, exists := si.Directories[adjustedPath]
	if exists && quick {
		modChange = dirInfo.ModTime() != cachedDir.ModTime
		cacheDirItems = cachedDir.Folders
	}
	si.mu.RUnlock()

	// If the directory has not been modified since the last index, skip expensive readdir
	// recursively check cached dirs for mod time changes as well
	if !modChange && recursive {
		for _, item := range cacheDirItems {
			err = si.indexDirectory(combinedPath+item.Name, quick, true)
			if err != nil {
				fmt.Printf("error indexing directory %v : %v", combinedPath+item.Name, err)
			}
		}
		return nil
	}

	if quick {
		si.mu.Lock()
		si.FilesChangedDuringIndexing = true
		si.mu.Unlock()
	}

	// Read directory contents
	files, err := dir.Readdir(-1)
	if err != nil {
		return err
	}

	var totalSize int64
	fileInfos := []ItemInfo{}
	dirInfos := []ItemInfo{}

	// Process each file and directory in the current directory
	for _, file := range files {
		itemInfo := &ItemInfo{
			Name:    file.Name(),
			ModTime: file.ModTime(),
		}
		if file.IsDir() {
			dirPath := combinedPath + file.Name()
			if recursive {
				// Recursively index the subdirectory
				err = si.indexDirectory(dirPath, quick, recursive)
				if err != nil {
					log.Printf("Failed to index directory %s: %v", dirPath, err)
					continue
				}
			}
			realDirInfo, exists := si.GetMetadataInfo(dirPath, true)
			if exists {
				itemInfo.Size = realDirInfo.Size
			}
			totalSize += itemInfo.Size
			itemInfo.Type = "directory"
			dirInfos = append(dirInfos, *itemInfo)
			si.NumDirs++
		} else {
			itemInfo.DetectType(combinedPath+file.Name(), false)
			itemInfo.Size = file.Size()
			fileInfos = append(fileInfos, *itemInfo)
			totalSize += itemInfo.Size
			si.NumFiles++
		}
	}
	// Create FileInfo for the current directory
	dirFileInfo := &FileInfo{
		Path:    adjustedPath,
		Files:   fileInfos,
		Folders: dirInfos,
	}
	dirFileInfo.ItemInfo = ItemInfo{
		Name:    dirInfo.Name(),
		Type:    "directory",
		Size:    totalSize,
		ModTime: dirInfo.ModTime(),
	}

	dirFileInfo.SortItems()

	// Update the current directory metadata in the index
	si.UpdateMetadata(dirFileInfo)

	return nil
}

func (si *Index) makeIndexPath(subPath string) string {
	if strings.HasPrefix(subPath, "./") {
		subPath = strings.TrimPrefix(subPath, ".")
	}
	if si.Root == subPath || subPath == "." {
		return "/"
	}
	// clean path
	subPath = strings.TrimSuffix(subPath, "/")
	// remove index prefix
	adjustedPath := strings.TrimPrefix(subPath, si.Root)
	// remove trailing slash
	adjustedPath = strings.TrimSuffix(adjustedPath, "/")
	if !strings.HasPrefix(adjustedPath, "/") {
		adjustedPath = "/" + adjustedPath
	}
	return adjustedPath
}

func (si *Index) recursiveUpdateDirSizes(childInfo *FileInfo, previousSize int64) {
	parentDir := utils.GetParentDirectoryPath(childInfo.Path)
	parentInfo, exists := si.GetMetadataInfo(parentDir, true)
	if !exists || parentDir == "" {
		return
	}
	newSize := parentInfo.Size - previousSize + childInfo.Size
	parentInfo.Size += newSize
	si.UpdateMetadata(parentInfo)
	si.recursiveUpdateDirSizes(parentInfo, newSize)
}

func (si *Index) RefreshFileInfo(opts FileOptions) error {
	refreshOptions := FileOptions{
		Path:  opts.Path,
		IsDir: opts.IsDir,
	}

	if !refreshOptions.IsDir {
		refreshOptions.Path = si.makeIndexPath(filepath.Dir(refreshOptions.Path))
		refreshOptions.IsDir = true
	} else {
		refreshOptions.Path = si.makeIndexPath(refreshOptions.Path)
	}
	err := si.indexDirectory(refreshOptions.Path, false, false)
	if err != nil {
		return fmt.Errorf("file/folder does not exist to refresh data: %s", refreshOptions.Path)
	}
	file, exists := si.GetMetadataInfo(refreshOptions.Path, true)
	if !exists {
		return fmt.Errorf("file/folder does not exist in metadata: %s", refreshOptions.Path)
	}

	current, firstExisted := si.GetMetadataInfo(refreshOptions.Path, true)
	refreshParentInfo := firstExisted && current.Size != file.Size
	//utils.PrintStructFields(*file)
	result := si.UpdateMetadata(file)
	if !result {
		return fmt.Errorf("file/folder does not exist in metadata: %s", refreshOptions.Path)
	}
	if !exists {
		return nil
	}
	if refreshParentInfo {
		si.recursiveUpdateDirSizes(file, current.Size)
	}
	return nil
}
