package state

import (
	"fmt"
	"net/http"
	"strings"
	"time"

	"github.com/gtsteffaniak/filebrowser/backend/internal/utils"
	"github.com/gtsteffaniak/filebrowser/backend/internal/database/share"
	"github.com/gtsteffaniak/filebrowser/backend/internal/database/users"
	"github.com/gtsteffaniak/go-logger/logger"
)

// Share operations

// GetShare retrieves a share by hash
// Returns a value (not pointer) to prevent modifications to the cache
func GetShare(hash string) (share.Share, error) {
	sharesMux.RLock()
	defer sharesMux.RUnlock()

	link, exists := sharesByHash[hash]
	if !exists {
		return share.Share{}, fmt.Errorf("share not found")
	}

	// Check if expired
	if !link.KeepAfterExpiration && link.Expire > 0 && link.Expire < time.Now().Unix() {
		return share.Share{}, fmt.Errorf("share expired")
	}

	// Return a value copy - automatically immutable
	return copyShareLink(link), nil
}

// GetAllShares retrieves all non-expired shares
// Returns values (not pointers) to prevent modifications to the cache
func GetAllShares() ([]share.Share, error) {
	sharesMux.RLock()
	defer sharesMux.RUnlock()

	now := time.Now().Unix()
	var shares []share.Share

	for _, link := range sharesByHash {
		if link.Expire == 0 || link.Expire > now || link.KeepAfterExpiration {
			shares = append(shares, copyShareLink(link))
		}
	}

	return shares, nil
}

// GetSharesByUserID retrieves all non-expired shares owned by userID.
// Returns values (not pointers) to prevent modifications to the cache
func GetSharesByUserID(userID uint64) ([]share.Share, error) {
	sharesMux.RLock()
	defer sharesMux.RUnlock()

	now := time.Now().Unix()
	var shares []share.Share

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
func GetSharesByPath(source, path string) ([]share.Share, error) {
	sharesMux.RLock()
	defer sharesMux.RUnlock()

	pathKey := makePathKey(source, path)
	hashes, exists := sharesByPath[pathKey]

	if !exists {
		return []share.Share{}, nil
	}

	var shares []share.Share
	for _, hash := range hashes {
		if link, exists := sharesByHash[hash]; exists {
			shares = append(shares, copyShareLink(link))
		}
	}

	return shares, nil
}

// CreateShare creates a new share with write-through to SQL
func CreateShare(link *share.Share) error {
	sharesMux.Lock()
	defer sharesMux.Unlock()

	// 1. Check if share already exists in cache (state)
	if _, exists := sharesByHash[link.Hash]; exists {
		return fmt.Errorf("share with hash %s already exists", link.Hash)
	}

	// 2. Write to database
	err := sqlDb.SaveShare(link)
	if err != nil {
		return err
	}

	// 3. Update cache to match database
	sharesByHash[link.Hash] = link
	pathKey := makePathKey(link.SourcePath, link.Path)

	// Add to path index
	sharesByPath[pathKey] = append(sharesByPath[pathKey], link.Hash)

	return nil
}

// UpdateShare updates an existing share with write-through to SQL
// Takes a function that modifies the share to ensure thread-safe updates
func UpdateShare(hash string, updateFn func(*share.Share) error) error {
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
	err := sqlDb.SaveShare(link)
	if err != nil {
		return err
	}

	// 3. Cache is already updated since we modified the pointer directly

	return nil
}

// RecordShareDownload increments global download count and, when per-user limits are enabled,
// increments the count for viewerUsername. It persists to the database. Callers must not
// mutate download counters on share snapshots directly.
func RecordShareDownload(hash, viewerUsername string) error {
	sharesMux.Lock()
	defer sharesMux.Unlock()
	link, exists := sharesByHash[hash]
	if !exists {
		return fmt.Errorf("share not found in cache")
	}
	link.Downloads++
	if link.PerUserDownloadLimit {
		link.IncrementUserDownload(viewerUsername)
	}
	return sqlDb.SaveShare(link)
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
	err := sqlDb.DeleteShare(hash)
	if err != nil {
		return err
	}

	// 3. Remove from cache to match database
	delete(sharesByHash, hash)

	// Remove from path index
	pathKey := makePathKey(link.SourcePath, link.Path)
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

	oldPathKey := makePathKey(link.SourcePath, link.Path)

	// 2. Update database
	err := sqlDb.UpdateSharePath(hash, newPath)
	if err != nil {
		return err
	}

	// 3. Update cache to match database
	link.Path = newPath
	newPathKey := makePathKey(link.SourcePath, newPath)

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
	err := sqlDb.UpdateSharesPaths(oldSource, oldPath, newSource, newPath)
	if err != nil {
		return err
	}

	// Update cache
	oldPathKey := makePathKey(oldSource, oldPath)
	if hashes, exists := sharesByPath[oldPathKey]; exists {
		for _, hash := range hashes {
			if link, exists := sharesByHash[hash]; exists {
				link.SourcePath = newSource
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

// removeShareFromPathIndexLocked removes hash from sharesByPath for the given source/path key. Caller must hold sharesMux.
func removeShareFromPathIndexLocked(source, path, hash string) {
	adjustedPath := utils.AddTrailingSlashIfNotExists(path)
	adjustedSource := utils.AddTrailingSlashIfNotExists(source)
	key := makePathKey(adjustedSource, adjustedPath)
	if inner, ok := sharesByPath[key]; ok {
		out := make([]string, 0, len(inner))
		for _, h := range inner {
			if h != hash {
				out = append(out, h)
			}
		}
		if len(out) == 0 {
			delete(sharesByPath, key)
		} else {
			sharesByPath[key] = out
		}
	}
}

// appendShareToPathIndexLocked appends hash to sharesByPath for source/path. Caller must hold sharesMux.
func appendShareToPathIndexLocked(source, path, hash string) {
	adjustedPath := utils.AddTrailingSlashIfNotExists(path)
	adjustedSource := utils.AddTrailingSlashIfNotExists(source)
	key := makePathKey(adjustedSource, adjustedPath)
	sharesByPath[key] = append(sharesByPath[key], hash)
}

// UpdateSharesForMovedResource updates share rows whose stored path is under a moved index path (same logic as
// share.Storage.UpdateShares). It is the only supported way to reconcile shares after a filesystem move.
// Returns hashes of updated shares (e.g. for logging).
func UpdateSharesForMovedResource(oldSource, oldPath, newSource, newPath string) ([]string, error) {
	sharesMux.Lock()
	defer sharesMux.Unlock()

	oldPath = utils.AddTrailingSlashIfNotExists(oldPath)
	newPath = utils.AddTrailingSlashIfNotExists(newPath)

	var updatedHashes []string
	for _, l := range sharesByHash {
		if l == nil || l.SourcePath != oldSource {
			continue
		}
		norm := utils.AddTrailingSlashIfNotExists(l.Path)
		if !strings.Contains(norm, oldPath) {
			continue
		}

		removeShareFromPathIndexLocked(l.SourcePath, l.Path, l.Hash)

		l.SourcePath = newSource
		l.Path = newPath

		if err := sqlDb.SaveShare(l); err != nil {
			logger.Error("failed to save updated share after resource move", "hash", l.Hash, "error", err)
			return updatedHashes, err
		}

		appendShareToPathIndexLocked(l.SourcePath, l.Path, l.Hash)
		updatedHashes = append(updatedHashes, l.Hash)
	}
	return updatedHashes, nil
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

// PrepSharesForFrontend builds API-safe ShareFrontend copies for share pointers.
func PrepSharesForFrontend(viewer *users.User, r *http.Request, publicHost, publicScheme string, links ...*share.Share) []*share.ShareFrontend {
	ownerLookup := func(userID uint64) string {
		u, err := GetUserByID(userID)
		if err != nil {
			return ""
		}
		return u.Username
	}
	return share.PrepForFrontend(viewer, r, publicHost, publicScheme, ownerLookup, links...)
}

// PrepShareValuesForFrontend builds API-safe ShareFrontend copies from immutable share values.
func PrepShareValuesForFrontend(viewer *users.User, r *http.Request, publicHost, publicScheme string, shares []share.Share) []*share.ShareFrontend {
	if len(shares) == 0 {
		return nil
	}
	ptrs := make([]*share.Share, len(shares))
	for i := range shares {
		s := shares[i]
		ptrs[i] = new(share.Share)
		*ptrs[i] = s
	}
	return PrepSharesForFrontend(viewer, r, publicHost, publicScheme, ptrs...)
}

// PrepShareForFrontend builds a single API-safe ShareFrontend copy from an immutable share value.
func PrepShareForFrontend(viewer *users.User, r *http.Request, publicHost, publicScheme string, sh share.Share) *share.ShareFrontend {
	prepped := PrepShareValuesForFrontend(viewer, r, publicHost, publicScheme, []share.Share{sh})
	if len(prepped) == 0 {
		return nil
	}
	return prepped[0]
}

// GetSharesInScope returns non-expired shares for scopePath on source owned by userID.
func GetSharesInScope(scopePath, source string, userID uint64) ([]*share.Share, error) {
	links, err := GetSharesByPath(source, scopePath)
	if err != nil {
		return nil, err
	}
	now := time.Now().Unix()
	filtered := make([]*share.Share, 0, len(links))
	for i := range links {
		l := links[i]
		if l.UserID == userID && (l.Expire == 0 || l.Expire > now || l.KeepAfterExpiration) {
			copy := l
			filtered = append(filtered, &copy)
		}
	}
	return filtered, nil
}

// copyShareLink creates a deep copy of a share link
func copyShareLink(link *share.Share) share.Share {
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
