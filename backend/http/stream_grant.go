package http

import (
	"fmt"
	"path/filepath"
	"strings"
	"time"

	"github.com/gtsteffaniak/filebrowser/backend/common/utils"
	"github.com/gtsteffaniak/filebrowser/backend/indexing/iteminfo"
)

const streamGrantTTL = 15 * time.Minute

func normalizeStreamGrantPath(p string) string {
	return filepath.ToSlash(strings.TrimSpace(p))
}

func mintStreamGrant(d *requestContext, source, filePath string, size int64, metadata *iteminfo.MediaMetadata) (string, error) {
	token, err := utils.RandomHex(16)
	if err != nil {
		return "", err
	}
	grant := utils.StreamGrant{
		UserID:    d.user.ID,
		ShareHash: d.share.Hash,
		Source:    source,
		Path:      normalizeStreamGrantPath(filePath),
		ExpiresAt: time.Now().Add(streamGrantTTL).Unix(),
		FileSize:  size,
	}
	if metadata != nil && metadata.Duration > 0 {
		grant.DurationSec = metadata.Duration
	}
	utils.StreamGrantsCache.Set(token, grant)
	return token, nil
}

func validateStreamGrant(token string, d *requestContext, source, filePath string) error {
	grant, ok := utils.StreamGrantsCache.Get(token)
	if !ok {
		return fmt.Errorf("invalid or expired stream token")
	}
	if time.Now().Unix() > grant.ExpiresAt {
		utils.StreamGrantsCache.Delete(token)
		return fmt.Errorf("stream token expired")
	}
	if grant.UserID != d.user.ID {
		return fmt.Errorf("stream token viewer mismatch")
	}
	if grant.ShareHash != d.share.Hash {
		return fmt.Errorf("stream token share mismatch")
	}
	if grant.Source != source {
		return fmt.Errorf("stream token source mismatch")
	}
	if grant.Path != normalizeStreamGrantPath(filePath) {
		return fmt.Errorf("stream token path mismatch")
	}
	return nil
}

func attachStreamToken(d *requestContext, source, filePath string, file *iteminfo.ExtendedFileInfo) {
	if file == nil || file.Type == "directory" {
		return
	}
	if !isStreamableMediaType(file.Type) {
		return
	}
	token, err := mintStreamGrant(d, source, filePath, file.Size, file.Metadata)
	if err != nil {
		return
	}
	file.StreamToken = token
}

func indexFilePath(dirPath, name string) string {
	dirPath = normalizeStreamGrantPath(dirPath)
	if dirPath == "" || dirPath == "/" {
		return "/" + name
	}
	if !strings.HasSuffix(dirPath, "/") {
		dirPath += "/"
	}
	return dirPath + name
}

func isStreamableMediaType(mimeType string) bool {
	return strings.HasPrefix(mimeType, "video/") || strings.HasPrefix(mimeType, "audio/")
}

func attachStreamTokensForDirectory(d *requestContext, source, dirPath string, file *iteminfo.ExtendedFileInfo) {
	if file == nil || file.Type != "directory" {
		return
	}
	for i := range file.Files {
		if file.Files[i].Type == "directory" {
			continue
		}
		if !isStreamableMediaType(file.Files[i].Type) {
			continue
		}
		childPath := indexFilePath(dirPath, file.Files[i].Name)
		child := &file.Files[i]
		token, err := mintStreamGrant(d, source, childPath, child.Size, child.Metadata)
		if err != nil {
			continue
		}
		file.Files[i].StreamToken = token
	}
}
