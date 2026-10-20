package web

import (
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"github.com/gtsteffaniak/filebrowser/backend/internal/database/users"
	"github.com/gtsteffaniak/filebrowser/backend/internal/state"
	"github.com/gtsteffaniak/filebrowser/backend/internal/utils"
	"github.com/gtsteffaniak/filebrowser/backend/pkg/indexing"
	"github.com/gtsteffaniak/go-logger/logger"
)

// extractAccessPolicy enforces per-path access rules and source file permissions during unarchive.
type extractAccessPolicy struct {
	idx             *indexing.Index
	destRealRoot    string
	destIndexPrefix string
	username        string
	perms           users.SourceFilePermissions
}

// allowWriteSymlinkEntry authorizes creating or replacing a symlink at destPath without following the final path component.
func (p *extractAccessPolicy) allowWriteSymlinkEntry(archiveEntryName, destPath string) error {
	if p == nil || p.idx == nil {
		return nil
	}
	parent := filepath.Dir(destPath)
	if err := p.allowWriteIncludingParents(archiveEntryName, parent); err != nil {
		return err
	}
	destResolved, err := filepath.EvalSymlinks(p.destRealRoot)
	if err != nil {
		logger.Debugf("unarchive: eval destination symlinks: %v", err)
		return errEntryPathInvalid
	}
	// Resolve the parent chain only: the final component is the link being
	// created or replaced and must be checked at the link path, not its target.
	linkPath, err := resolvedLinkPath(destPath)
	if err != nil {
		logger.Debugf("unarchive: resolve symlink entry parent: %v", err)
		return errEntryPathInvalid
	}
	rel, err := filepath.Rel(filepath.Clean(destResolved), linkPath)
	if err != nil {
		logger.Debugf("unarchive: symlink entry relative path: %v", err)
		return errEntryPathInvalid
	}
	if rel == ".." || strings.HasPrefix(rel, ".."+string(filepath.Separator)) {
		return errEntryPathEscape
	}
	rel = filepath.ToSlash(rel)
	indexPath := utils.JoinPathAsUnix(p.destIndexPrefix, rel)
	if !state.AccessPermitted(p.idx.Path, utils.IndexPathFromNormalized(indexPath, true), p.username) {
		logger.Debugf("unarchive: symlink entry %q denied at index path %s for user %s", archiveEntryName, indexPath, p.username)
		return newExtractAccessDenied(archiveEntryName)
	}
	if _, statErr := os.Lstat(destPath); os.IsNotExist(statErr) {
		if !p.perms.Create {
			return fmt.Errorf("%w: create permission required", newExtractAccessDenied(archiveEntryName))
		}
		return nil
	}
	if !p.perms.Modify {
		return fmt.Errorf("%w: modify permission required", newExtractAccessDenied(archiveEntryName))
	}
	return nil
}

// resolvedLinkPath resolves symlink components in the parents of p without
// following the final path component, so checks apply to the link path itself.
func resolvedLinkPath(p string) (string, error) {
	parent := filepath.Dir(p)
	leaf := filepath.Base(p)
	for {
		resolved, err := filepath.EvalSymlinks(parent)
		if err == nil {
			return filepath.Join(resolved, leaf), nil
		}
		if !errors.Is(err, os.ErrNotExist) {
			return "", err
		}
		next := filepath.Dir(parent)
		if next == parent {
			return "", err
		}
		leaf = filepath.Join(filepath.Base(parent), leaf)
		parent = next
	}
}

// allowWrite checks index access and source Create/Modify permissions for one destination path.
func (p *extractAccessPolicy) allowWrite(archiveEntryName, destPath string) error {
	if p == nil || p.idx == nil {
		return nil
	}
	effectivePath, err := resolvedWritePathUnderDest(p.destRealRoot, destPath)
	if err != nil {
		return err
	}
	destResolved, err := filepath.EvalSymlinks(p.destRealRoot)
	if err != nil {
		logger.Debugf("unarchive: eval destination symlinks: %v", err)
		return errEntryPathInvalid
	}
	rel, err := filepath.Rel(filepath.Clean(destResolved), filepath.Clean(effectivePath))
	if err != nil {
		logger.Debugf("unarchive: entry relative path: %v", err)
		return errEntryPathInvalid
	}
	rel = filepath.ToSlash(rel)
	indexPath := utils.JoinPathAsUnix(p.destIndexPrefix, rel)
	if !state.AccessPermitted(p.idx.Path, utils.IndexPathFromNormalized(indexPath, true), p.username) {
		logger.Debugf("unarchive: entry %q denied at index path %s for user %s", archiveEntryName, indexPath, p.username)
		return newExtractAccessDenied(archiveEntryName)
	}
	if _, statErr := os.Stat(destPath); os.IsNotExist(statErr) {
		if !p.perms.Create {
			return fmt.Errorf("%w: create permission required", newExtractAccessDenied(archiveEntryName))
		}
		return nil
	}
	if !p.perms.Modify {
		return fmt.Errorf("%w: modify permission required", newExtractAccessDenied(archiveEntryName))
	}
	return nil
}

// allowWriteIncludingParents authorizes destPath and every missing ancestor directory that extraction would create.
func (p *extractAccessPolicy) allowWriteIncludingParents(archiveEntryName, destPath string) error {
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
		if err := p.allowWrite(archiveEntryName, missingParents[i]); err != nil {
			return err
		}
	}
	return p.allowWrite(archiveEntryName, destPath)
}

func preflightExtractEntry(policy *extractAccessPolicy, archiveEntryName, destPath string, isDir, isSymlink bool) error {
	if policy == nil {
		return nil
	}
	if isSymlink {
		return policy.allowWriteSymlinkEntry(archiveEntryName, destPath)
	}
	return policy.allowWriteIncludingParents(archiveEntryName, destPath)
}
