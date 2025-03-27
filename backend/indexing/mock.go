package indexing

import (
	"math/rand"
	"time"

	"github.com/gtsteffaniak/filebrowser/backend/common/utils"
	"github.com/gtsteffaniak/filebrowser/backend/indexing/iteminfo"
)

func (idx *Index) CreateMockData(numDirs, numFilesPerDir int) {
	for i := 0; i < numDirs; i++ {
		dirPath := utils.GenerateRandomPath(rand.Intn(3) + 1)
		files := []iteminfo.ItemInfo{} // Slice of FileInfo

		// Simulating files and directories with FileInfo
		for j := 0; j < numFilesPerDir; j++ {
			newFile := iteminfo.ItemInfo{
				Name:    "file-" + utils.GetRandomTerm() + utils.GetRandomExtension(),
				Size:    rand.Int63n(1000),                                          // Random size
				ModTime: time.Now().Add(-time.Duration(rand.Intn(100)) * time.Hour), // Random mod time
				Type:    "blob",
			}
			files = append(files, newFile)
		}
		dirInfo := &iteminfo.FileInfo{
			Path:  dirPath,
			Files: files,
		}

		idx.UpdateMetadata(dirInfo)
	}
}
