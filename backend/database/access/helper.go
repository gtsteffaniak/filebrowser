package access

import (
	"github.com/gtsteffaniak/filebrowser/backend/common/errors"
	"github.com/gtsteffaniak/filebrowser/backend/common/utils"
	"github.com/gtsteffaniak/filebrowser/backend/indexing"
	"github.com/gtsteffaniak/filebrowser/backend/indexing/iteminfo"
)

func (s *Storage) CheckChildItemAccess(response *iteminfo.FileInfo, index *indexing.Index, username string) error {

	// Collect all item names to check
	allItemNames := make([]string, 0, len(response.Folders)+len(response.Files))
	for _, folder := range response.Folders {
		allItemNames = append(allItemNames, folder.Name)
	}
	for _, file := range response.Files {
		allItemNames = append(allItemNames, file.Name)
	}

	// Use standardized path with trailing slash for proper path construction
	// response is an ExtendedFileInfo which represents a directory (has Folders and Files)
	parentPath := index.MakeIndexPath(response.Path, true)

	// Check if user has access to any items
	if !s.HasAnyVisibleItems(index.Path, parentPath, allItemNames, username) && len(allItemNames) > 0 {
		return errors.ErrAccessDenied
	}

	// Save original folders and files before filtering
	originalFolders := response.Folders
	originalFiles := response.Files

	// Filter and return only the items the user has access to
	response.Folders = make([]iteminfo.ItemInfo, 0)
	response.Files = make([]iteminfo.ExtendedItemInfo, 0)

	// Check each subfolder for access permissions
	for _, folder := range originalFolders {
		indexPath := parentPath + folder.Name
		if s.Permitted(index.Path, indexPath, username) {
			response.Folders = append(response.Folders, folder)
		}
	}

	// Check each subfile for access permissions
	for _, file := range originalFiles {
		indexPath := parentPath + file.Name
		if s.Permitted(index.Path, indexPath, username) {
			response.Files = append(response.Files, file)
		}
	}

	return nil
}

type FileOptionsExtended struct {
	utils.FileOptions
	Access *Storage
}
