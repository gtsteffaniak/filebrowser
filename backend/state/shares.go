package state

import (
	"fmt"
	"strings"
	"time"

	"github.com/gtsteffaniak/filebrowser/backend/database/share"
	"github.com/gtsteffaniak/filebrowser/backend/database/users"
	"github.com/gtsteffaniak/go-logger/logger"
)

// Share operations

// GetShare retrieves a share by hash
// Returns a value (not pointer) to prevent modifications to the cache
func GetShare(hash string) (share.Link, error) {
	sharesMux.RLock()
	defer sharesMux.RUnlock()

	link, exists := sharesByHash[hash]
	if !exists {
		return share.Link{}, fmt.Errorf("share not found")
	}

	// Check if expired
	if !link.KeepAfterExpiration && link.Expire > 0 && link.Expire < time.Now().Unix() {
		return share.Link{}, fmt.Errorf("share expired")
	}

	// Return a value copy - automatically immutable
	return copyShareLink(link), nil
}

// GetAllShares retrieves all non-expired shares
// Returns values (not pointers) to prevent modifications to the cache
func GetAllShares() ([]share.Link, error) {
	sharesMux.RLock()
	defer sharesMux.RUnlock()

	now := time.Now().Unix()
	var shares []share.Link

	for _, link := range sharesByHash {
		if link.Expire == 0 || link.Expire > now || link.KeepAfterExpiration {
			shares = append(shares, copyShareLink(link))
		}
	}

	return shares, nil
}

// GetSharesByUserID retrieves all non-expired shares owned by userID.
// Returns values (not pointers) to prevent modifications to the cache
func GetSharesByUserID(userID uint64) ([]share.Link, error) {
	sharesMux.RLock()
	defer sharesMux.RUnlock()

	now := time.Now().Unix()
	var shares []share.Link

	for _, link := range sharesByHash {
		if link.UserID == userID {
			if link.Expire == 0 || link.Expire > now || link.KeepAfterExpiration {
				shares = append(shares, copyShareLink(link))
			}
		}
	}

	return shares, nil
}

// GetSharesByPath retrieves shares for a specific source and path
// Returns values (not pointers) to prevent modifications to the cache
func GetSharesByPath(source, path string) ([]share.Link, error) {
	logger.Debug("GetSharesByPath ENTRY", "source", source, "path", path)
	sharesMux.RLock()
	defer sharesMux.RUnlock()

	pathKey := makePathKey(source, path)
	hashes, exists := sharesByPath[pathKey]
	
	logger.Debug("GetSharesByPath lookup", "pathKey", pathKey, "found", exists, "count", len(hashes), "totalKeys", len(sharesByPath))
	
	if !exists {
		return []share.Link{}, nil
	}

	var shares []share.Link
	for _, hash := range hashes {
		if link, exists := sharesByHash[hash]; exists {
			shares = append(shares, copyShareLink(link))
		}
	}

	return shares, nil
}

// CreateShare creates a new share with write-through to SQL
func CreateShare(link *share.Link) error {
	sharesMux.Lock()
	defer sharesMux.Unlock()

	// 1. Check if share already exists in cache (state)
	if _, exists := sharesByHash[link.Hash]; exists {
		return fmt.Errorf("share with hash %s already exists", link.Hash)
	}

	// 2. Write to database
	err := sqlStore.SaveShare(link)
	if err != nil {
		return err
	}

	// 3. Update cache to match database
	sharesByHash[link.Hash] = link
	pathKey := makePathKey(link.Source, link.Path)

	// Add to path index
	sharesByPath[pathKey] = append(sharesByPath[pathKey], link.Hash)
	
	logger.Debug("CreateShare complete", "hash", link.Hash, "pathKey", pathKey, "totalSharesInPath", len(sharesByPath[pathKey]))

	return nil
}

// UpdateShare updates an existing share with write-through to SQL
// Takes a function that modifies the share to ensure thread-safe updates
func UpdateShare(hash string, updateFn func(*share.Link) error) error {
	sharesMux.Lock()
	defer sharesMux.Unlock()

	// 1. Check if share exists in cache (state)
	link, exists := sharesByHash[hash]
	if !exists {
		return fmt.Errorf("share not found in cache")
	}

	// Apply updates to get the new state
	if err := updateFn(link); err != nil {
		return err
	}

	// 2. Write to database
	err := sqlStore.SaveShare(link)
	if err != nil {
		return err
	}

	// 3. Cache is already updated since we modified the pointer directly

	return nil
}

// DeleteShare deletes a share by hash
func DeleteShare(hash string) error {
	sharesMux.Lock()
	defer sharesMux.Unlock()

	// 1. Check if share exists in cache (state)
	link, exists := sharesByHash[hash]
	if !exists {
		return fmt.Errorf("share not found in cache")
	}

	// 2. Delete from database
	err := sqlStore.DeleteShare(hash)
	if err != nil {
		return err
	}

	// 3. Remove from cache to match database
	delete(sharesByHash, hash)

	// Remove from path index
	pathKey := makePathKey(link.Source, link.Path)
	hashes := sharesByPath[pathKey]
	for i, h := range hashes {
		if h == hash {
			sharesByPath[pathKey] = append(hashes[:i], hashes[i+1:]...)
			break
		}
	}

	// Clean up empty path entries
	if len(sharesByPath[pathKey]) == 0 {
		delete(sharesByPath, pathKey)
	}

	return nil
}

// UpdateSharePath updates the path for a specific share
func UpdateSharePath(hash, newPath string) error {
	sharesMux.Lock()
	defer sharesMux.Unlock()

	// 1. Check if share exists in cache (state)
	link, exists := sharesByHash[hash]
	if !exists {
		return fmt.Errorf("share not found in cache")
	}

	oldPathKey := makePathKey(link.Source, link.Path)

	// 2. Update database
	err := sqlStore.UpdateSharePath(hash, newPath)
	if err != nil {
		return err
	}

	// 3. Update cache to match database
	link.Path = newPath
	newPathKey := makePathKey(link.Source, newPath)

	// Remove from old path index
	if oldHashes, exists := sharesByPath[oldPathKey]; exists {
		for i, h := range oldHashes {
			if h == hash {
				sharesByPath[oldPathKey] = append(oldHashes[:i], oldHashes[i+1:]...)
				break
			}
		}
		if len(sharesByPath[oldPathKey]) == 0 {
			delete(sharesByPath, oldPathKey)
		}
	}

	// Add to new path index
	sharesByPath[newPathKey] = append(sharesByPath[newPathKey], hash)

	return nil
}

// UpdateSharesPaths updates paths for shares when a resource is moved
func UpdateSharesPaths(oldSource, oldPath, newSource, newPath string) error {
	sharesMux.Lock()
	defer sharesMux.Unlock()

	// Update SQL
	err := sqlStore.UpdateSharesPaths(oldSource, oldPath, newSource, newPath)
	if err != nil {
		return err
	}

	// Update cache
	oldPathKey := makePathKey(oldSource, oldPath)
	if hashes, exists := sharesByPath[oldPathKey]; exists {
		for _, hash := range hashes {
			if link, exists := sharesByHash[hash]; exists {
				link.Source = newSource
				link.Path = newPath
			}
		}

		// Move hashes to new path key
		newPathKey := makePathKey(newSource, newPath)
		sharesByPath[newPathKey] = hashes
		delete(sharesByPath, oldPathKey)
	}

	return nil
}

// IsShared checks if a path is shared by the given owner user id.
func IsShared(source, path string, userID uint64) bool {
	sharesMux.RLock()
	defer sharesMux.RUnlock()

	// Normalize path
	normalizedPath := strings.TrimSuffix(path, "/")
	if normalizedPath == "" {
		normalizedPath = "/"
	}
	pathKey := makePathKey(source, normalizedPath)

	hashes, exists := sharesByPath[pathKey]
	if !exists {
		return false
	}

	now := time.Now().Unix()
	for _, hash := range hashes {
		link, exists := sharesByHash[hash]
		if !exists {
			continue
		}
		if link.UserID == userID {
			if link.Expire == 0 || link.Expire > now {
				return true
			}
		}
	}

	return false
}

// copyShareLink creates a deep copy of a share link
func copyShareLink(link *share.Link) share.Link {
	// Shallow copy (now safe since there's no mutex)
	linkCopy := *link

	// Deep copy slices
	if link.AllowedUsernames != nil {
		linkCopy.AllowedUsernames = make([]string, len(link.AllowedUsernames))
		copy(linkCopy.AllowedUsernames, link.AllowedUsernames)
	}

	if link.SidebarLinks != nil {
		linkCopy.SidebarLinks = make([]users.SidebarLink, len(link.SidebarLinks))
		copy(linkCopy.SidebarLinks, link.SidebarLinks)
	}

	// Deep copy map
	if link.UserDownloads != nil {
		linkCopy.UserDownloads = make(map[string]int, len(link.UserDownloads))
		for k, v := range link.UserDownloads {
			linkCopy.UserDownloads[k] = v
		}
	}

	//nolint:govet // We intentionally don't copy the Mu field to avoid mutex copy
	return linkCopy
}
