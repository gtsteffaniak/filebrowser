package state

import (
	"fmt"
	"strings"
	"time"

	"github.com/gtsteffaniak/filebrowser/backend/database/share"
)

// Share operations

// GetShare retrieves a share by hash
func GetShare(hash string) (*share.Link, error) {
	mux.RLock()
	defer mux.RUnlock()
	
	link, exists := sharesByHash[hash]
	if !exists {
		return nil, fmt.Errorf("share not found")
	}
	
	// Check if expired
	if !link.KeepAfterExpiration && link.Expire > 0 && link.Expire < time.Now().Unix() {
		return nil, fmt.Errorf("share expired")
	}
	
	return link, nil
}

// GetAllShares retrieves all non-expired shares
func GetAllShares() ([]*share.Link, error) {
	mux.RLock()
	defer mux.RUnlock()
	
	now := time.Now().Unix()
	var shares []*share.Link
	
	for _, link := range sharesByHash {
		if link.Expire == 0 || link.Expire > now || link.KeepAfterExpiration {
			shares = append(shares, link)
		}
	}
	
	return shares, nil
}

// GetSharesByUserID retrieves all non-expired shares for a user
func GetSharesByUserID(userID uint) ([]*share.Link, error) {
	mux.RLock()
	defer mux.RUnlock()
	
	now := time.Now().Unix()
	var shares []*share.Link
	
	for _, link := range sharesByHash {
		if link.UserID == userID {
			if link.Expire == 0 || link.Expire > now || link.KeepAfterExpiration {
				shares = append(shares, link)
			}
		}
	}
	
	return shares, nil
}

// GetSharesByPath retrieves shares for a specific source and path
func GetSharesByPath(source, path string) ([]*share.Link, error) {
	mux.RLock()
	defer mux.RUnlock()
	
	pathKey := makePathKey(source, path)
	hashes, exists := sharesByPath[pathKey]
	if !exists {
		return []*share.Link{}, nil
	}
	
	var shares []*share.Link
	for _, hash := range hashes {
		if link, exists := sharesByHash[hash]; exists {
			shares = append(shares, link)
		}
	}
	
	return shares, nil
}

// SaveShare saves a share with write-through to SQL
func SaveShare(link *share.Link) error {
	mux.Lock()
	defer mux.Unlock()
	
	// Write through to SQL
	err := sqlStore.SaveShare(link)
	if err != nil {
		return err
	}
	
	// Update cache
	sharesByHash[link.Hash] = link
	pathKey := makePathKey(link.Source, link.Path)
	
	// Add to path index if not already there
	found := false
	for _, hash := range sharesByPath[pathKey] {
		if hash == link.Hash {
			found = true
			break
		}
	}
	if !found {
		sharesByPath[pathKey] = append(sharesByPath[pathKey], link.Hash)
	}
	
	return nil
}

// DeleteShare deletes a share by hash
func DeleteShare(hash string) error {
	mux.Lock()
	defer mux.Unlock()
	
	// Get share before deleting
	link, exists := sharesByHash[hash]
	if !exists {
		return fmt.Errorf("share not found")
	}
	
	// Delete from SQL
	err := sqlStore.DeleteShare(hash)
	if err != nil {
		return err
	}
	
	// Remove from cache
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
	mux.Lock()
	defer mux.Unlock()
	
	link, exists := sharesByHash[hash]
	if !exists {
		return fmt.Errorf("share not found")
	}
	
	oldPathKey := makePathKey(link.Source, link.Path)
	
	// Update SQL
	err := sqlStore.UpdateSharePath(hash, newPath)
	if err != nil {
		return err
	}
	
	// Update cache
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
	mux.Lock()
	defer mux.Unlock()
	
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

// IsShared checks if a path is shared by a user
func IsShared(source, path string, userID uint) bool {
	mux.RLock()
	defer mux.RUnlock()
	
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
