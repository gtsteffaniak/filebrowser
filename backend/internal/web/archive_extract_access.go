package web

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"github.com/gtsteffaniak/filebrowser/backend/internal/database/users"
	"github.com/gtsteffaniak/filebrowser/backend/internal/state"
	"github.com/gtsteffaniak/filebrowser/backend/internal/utils"
	"github.com/gtsteffaniak/filebrowser/backend/pkg/indexing"
)

// extractAccessPolicy enforces per-path access rules and source file permissions during unarchive.
type extractAccessPolicy struct {
	idx             *indexing.Index
	destRealRoot    string
	destIndexPrefix string
	username        string
	perms           users.SourceFilePermissions
}

// allowWrite checks index access and source Create/Modify permissions for one destination path.
func (p *extractAccessPolicy) allowWrite(destPath string) error {
	if p == nil || p.idx == nil {
		return nil
	}
	effectivePath, err := resolvedWritePathUnderDest(p.destRealRoot, destPath)
	if err != nil {
		return err
	}
	destResolved, err := filepath.EvalSymlinks(p.destRealRoot)
	if err != nil {
		return err
	}
	rel, err := filepath.Rel(filepath.Clean(destResolved), filepath.Clean(effectivePath))
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

// allowWriteIncludingParents authorizes destPath and every missing ancestor directory that extraction would create.
func (p *extractAccessPolicy) allowWriteIncludingParents(destPath string) error {
	if p == nil || p.idx == nil {
		return nil
	}
	destRoot := filepath.Clean(p.destRealRoot)
	var missingParents []string
	for dir := filepath.Dir(destPath); ; dir = filepath.Dir(dir) {
		cleanDir := filepath.Clean(dir)
		if cleanDir == destRoot {
			break
		}
		rel, relErr := filepath.Rel(destRoot, cleanDir)
		if relErr != nil || rel == ".." || strings.HasPrefix(rel, ".."+string(filepath.Separator)) {
			break
		}
		if _, statErr := os.Stat(cleanDir); os.IsNotExist(statErr) {
			missingParents = append(missingParents, cleanDir)
		}
	}
	for i := len(missingParents) - 1; i >= 0; i-- {
		if err := p.allowWrite(missingParents[i]); err != nil {
			return err
		}
	}
	return p.allowWrite(destPath)
}
