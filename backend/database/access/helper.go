package access

import (
	"github.com/gtsteffaniak/filebrowser/backend/common/errors"
	"github.com/gtsteffaniak/filebrowser/backend/common/utils"
	"github.com/gtsteffaniak/filebrowser/backend/indexing"
	"github.com/gtsteffaniak/filebrowser/backend/indexing/iteminfo"
)

func (s *Storage) CheckChildItemAccess(response *iteminfo.ExtendedFileInfo, index *indexing.Index, username string) error {

	// Collect all item names to check
	allItemNames := make([]string, 0, len(response.Folders)+len(response.Files))
	for _, folder := range response.Folders {
		allItemNames = append(allItemNames, folder.Name)
	}
	for _, file := range response.Files {
		allItemNames = append(allItemNames, file.Name)
	}

	// Use standardized path with trailing slash for proper path construction
	parentPath := index.MakeIndexPath(response.Path)

	// Check if user has access to any items
	if !s.HasAnyVisibleItems(index.Path, parentPath, allItemNames, username) {
		return errors.ErrAccessDenied
	}

	// Filter and return only the items the user has access to
	response.Folders = make([]iteminfo.ItemInfo, 0)
	response.Files = make([]iteminfo.ItemInfo, 0)

	// Check each subfolder for access permissions
	for _, folder := range response.Folders {
		indexPath := parentPath + folder.Name
		if s.Permitted(index.Path, indexPath, username) {
			response.Folders = append(response.Folders, folder)
		}
	}

	// Check each subfile for access permissions
	for _, file := range response.Files {
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
