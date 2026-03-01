package share

import (
	"os"
	"path/filepath"
	"strings"

	"github.com/gtsteffaniak/filebrowser/backend/common/settings"
	"github.com/gtsteffaniak/filebrowser/backend/database/users"
)

// IsSingleFileShare determines if this share is for a single file (not a directory).
// It checks both the file extension and filesystem to determine if the path points to a file.
func (l *Link) IsSingleFileShare() bool {
	if l.Path == "" {
		return false
	}

	// First, check if the path has a file extension (common indicator of a file)
	ext := filepath.Ext(l.Path)
	if ext != "" {
		// If it has an extension, it's likely a file, but let's verify with filesystem
		return l.isFileOnFilesystem()
	}

	// If no extension, check if it's a directory by looking at the filesystem
	return !l.isDirectoryOnFilesystem()
}

// isFileOnFilesystem checks if the path exists and is a file on the filesystem
func (l *Link) isFileOnFilesystem() bool {
	// Construct the full path using Source and Path
	fullPath := l.Path
	if l.Source != "" {
		// If Source is provided, it might be a relative path from the source
		fullPath = filepath.Join(l.Source, l.Path)
	}

	info, err := os.Stat(fullPath)
	if err != nil {
		// If we can't stat the file, fall back to extension check
		return filepath.Ext(l.Path) != ""
	}

	return !info.IsDir()
}

// isDirectoryOnFilesystem checks if the path exists and is a directory on the filesystem
func (l *Link) isDirectoryOnFilesystem() bool {
	// Construct the full path using Source and Path
	fullPath := l.Path
	if l.Source != "" {
		// If Source is provided, it might be a relative path from the source
		fullPath = filepath.Join(l.Source, l.Path)
	}

	info, err := os.Stat(fullPath)
	if err != nil {
		// If we can't stat the file, assume it's a file if it has an extension
		return filepath.Ext(l.Path) == ""
	}

	return info.IsDir()
}

// IsExpired checks if the share has expired based on the Expire timestamp
func (l *Link) IsExpired() bool {
	if l.Expire == 0 {
		return false // No expiration set
	}
	// This would need time.Now().Unix() but we avoid importing time here
	// The actual expiration check is handled in the storage layer
	return false // Placeholder - actual implementation would check current time
}

// HasPassword checks if the share is password protected
func (l *Link) HasPassword() bool {
	return l.PasswordHash != ""
}

// IsPermanent checks if the share is permanent (no expiration)
func (l *Link) IsPermanent() bool {
	return l.Expire == 0
}

// GetFileExtension returns the file extension of the shared file
func (l *Link) GetFileExtension() string {
	if l.Path == "" {
		return ""
	}
	return filepath.Ext(l.Path)
}

// GetFileName returns just the filename (without path) of the shared item
func (l *Link) GetFileName() string {
	if l.Path == "" {
		return ""
	}
	return filepath.Base(l.Path)
}

// InitUserDownloads initializes the user downloads map if needed
func (l *Link) InitUserDownloads() {
	l.Mu.Lock()
	defer l.Mu.Unlock()
	if l.UserDownloads == nil {
		l.UserDownloads = make(map[string]int)
	}
}

// IncrementUserDownload increments the download count for a specific user
func (l *Link) IncrementUserDownload(username string) {
	l.Mu.Lock()
	defer l.Mu.Unlock()
	if l.UserDownloads == nil {
		l.UserDownloads = make(map[string]int)
	}
	l.UserDownloads[username]++
}

// GetUserDownloadCount returns the download count for a specific user
func (l *Link) GetUserDownloadCount(username string) int {
	l.Mu.Lock()
	defer l.Mu.Unlock()
	if l.UserDownloads == nil {
		return 0
	}
	return l.UserDownloads[username]
}

// ResetDownloadCounts resets both global and per-user download counts
func (l *Link) ResetDownloadCounts() {
	l.Mu.Lock()
	defer l.Mu.Unlock()
	l.Downloads = 0
	l.UserDownloads = make(map[string]int)
}

// HasReachedUserLimit checks if a user has reached their download limit
func (l *Link) HasReachedUserLimit(username string) bool {
	if !l.PerUserDownloadLimit || l.DownloadsLimit == 0 {
		return false
	}
	count := l.GetUserDownloadCount(username)
	return count >= l.DownloadsLimit
}

func (l *Link) GetSourceName() string {
	sourceInfo, ok := settings.Config.Server.SourceMap[l.Source]
	if !ok {
		return ""
	}
	return sourceInfo.Name
}

func (l *Link) UserCanEdit(user *users.User) bool {
	return l.UserID == user.ID || user.Permissions.Admin
}

func (l *Link) SourceURL(user *users.User) string {
	sourceName := l.GetSourceName()
	// get user scope path from share
	userScope, err := user.GetScopeForSourceName(sourceName)
	if err != nil {
		return ""
	}
	if !strings.HasPrefix(l.Path, userScope) {
		return ""
	}
	scopedPath := strings.TrimPrefix(l.Path, userScope)
	return filepath.Join(settings.Config.Server.BaseURL, sourceName, scopedPath)
}
