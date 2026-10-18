package web

import (
	"fmt"
	"os"
	"path/filepath"
	"github.com/gtsteffaniak/filebrowser/backend/internal/database/users"
	"github.com/gtsteffaniak/filebrowser/backend/internal/state"
	"github.com/gtsteffaniak/filebrowser/backend/internal/utils"
	"github.com/gtsteffaniak/filebrowser/backend/pkg/indexing"
)

type extractAccessPolicy struct {
	idx             *indexing.Index
	destRealRoot    string
	destIndexPrefix string
	username        string
	perms           users.SourceFilePermissions
}

func (p *extractAccessPolicy) allowWrite(destPath string) error {
	if p == nil || p.idx == nil {
		return nil
	}
	rel, err := filepath.Rel(filepath.Clean(p.destRealRoot), filepath.Clean(destPath))
	if err != nil {
		return err
	}
	rel = filepath.ToSlash(rel)
	indexPath := utils.JoinPathAsUnix(p.destIndexPrefix, rel)
	if !state.AccessPermitted(p.idx.Path, utils.IndexPathFromNormalized(indexPath, true), p.username) {
		return fmt.Errorf("access denied to %s", rel)
	}
	if _, statErr := os.Stat(destPath); os.IsNotExist(statErr) {
		if !p.perms.Create {
			return fmt.Errorf("create permission required")
		}
		return nil
	}
	if !p.perms.Modify {
		return fmt.Errorf("modify permission required")
	}
	return nil
}
