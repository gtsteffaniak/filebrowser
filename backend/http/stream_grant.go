package http

import (
	"fmt"
	"path/filepath"
	"strings"
	"time"

	"github.com/gtsteffaniak/filebrowser/backend/common/utils"
	"github.com/gtsteffaniak/filebrowser/backend/indexing/iteminfo"
)

const viewGrantTTL = 15 * time.Minute

func normalizeViewGrantPath(p string) string {
	return filepath.ToSlash(strings.TrimSpace(p))
}

func mintViewGrant(d *requestContext, source, filePath string) (string, error) {
	token, err := utils.RandomHex(16)
	if err != nil {
		return "", err
	}
	grant := utils.ViewGrant{
		UserID:    d.user.ID,
		ShareHash: d.share.Hash,
		Source:    source,
		Path:      normalizeViewGrantPath(filePath),
		ExpiresAt: time.Now().Add(viewGrantTTL).Unix(),
	}
	utils.ViewGrantsCache.Set(token, grant)
	return token, nil
}

func validateViewGrant(token string, d *requestContext, source, filePath string) error {
	grant, ok := utils.ViewGrantsCache.Get(token)
	if !ok {
		return fmt.Errorf("invalid or expired view token")
	}
	if time.Now().Unix() > grant.ExpiresAt {
		utils.ViewGrantsCache.Delete(token)
		return fmt.Errorf("view token expired")
	}
	if grant.UserID != d.user.ID {
		return fmt.Errorf("view token viewer mismatch")
	}
	if grant.ShareHash != d.share.Hash {
		return fmt.Errorf("view token share mismatch")
	}
	if grant.Source != source {
		return fmt.Errorf("view token source mismatch")
	}
	if grant.Path != normalizeViewGrantPath(filePath) {
		return fmt.Errorf("view token path mismatch")
	}
	return nil
}

func attachViewToken(d *requestContext, source, filePath string, file *iteminfo.ExtendedFileInfo) {
	if file == nil || file.Type == "directory" {
		return
	}
	token, err := mintViewGrant(d, source, filePath)
	if err != nil {
		return
	}
	file.ViewToken = token
}

func indexFilePath(dirPath, name string) string {
	dirPath = normalizeViewGrantPath(dirPath)
	if dirPath == "" || dirPath == "/" {
		return "/" + name
	}
	if !strings.HasSuffix(dirPath, "/") {
		dirPath += "/"
	}
	return dirPath + name
}

func attachViewTokensForDirectory(d *requestContext, source, dirPath string, file *iteminfo.ExtendedFileInfo) {
	if file == nil || file.Type != "directory" {
		return
	}
	for i := range file.Files {
		if file.Files[i].Type == "directory" {
			continue
		}
		childPath := indexFilePath(dirPath, file.Files[i].Name)
		token, err := mintViewGrant(d, source, childPath)
		if err != nil {
			continue
		}
		file.Files[i].ViewToken = token
	}
}
