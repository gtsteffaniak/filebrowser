package files

import (
	"github.com/gtsteffaniak/filebrowser/backend/internal/database/share"
	"github.com/gtsteffaniak/filebrowser/backend/internal/database/users"
	"github.com/gtsteffaniak/filebrowser/backend/internal/ports"
	"github.com/gtsteffaniak/filebrowser/backend/internal/utils"
	"github.com/gtsteffaniak/filebrowser/backend/pkg/indexing"
	"github.com/gtsteffaniak/filebrowser/backend/pkg/indexing/iteminfo"
)

// ChildAccessChecker filters directory listings by access rules.
type ChildAccessChecker interface {
	CheckChildItemAccess(response *iteminfo.FileInfo, idx *indexing.Index, username string) error
}

// Service performs filesystem operations with injected access and share dependencies.
type Service struct {
	access ports.AccessGate
	child  ChildAccessChecker
	shares ports.ShareReader
}

var defaultService *Service

// New constructs a file service with the given port implementations.
func New(access ports.AccessGate, child ChildAccessChecker, shares ports.ShareReader) *Service {
	return &Service{access: access, child: child, shares: shares}
}

// SetDefault registers the process-wide file service (called from app.WireServices).
func SetDefault(s *Service) {
	defaultService = s
}

func svc() *Service {
	if defaultService != nil {
		return defaultService
	}
	return &Service{}
}

func (s *Service) accessPermitted(sourcePath string, indexPath utils.IndexPath, username string) bool {
	if s.access == nil {
		return true
	}
	return s.access.AccessPermitted(sourcePath, indexPath, username)
}

func (s *Service) checkChildItemAccess(response *iteminfo.FileInfo, idx *indexing.Index, username string) error {
	if s.child == nil {
		return nil
	}
	return s.child.CheckChildItemAccess(response, idx, username)
}

func (s *Service) pathIsShared(path, source string, userID uint64) bool {
	if s.shares == nil {
		return false
	}
	return s.shares.IsShared(source, path, userID)
}

func (s *Service) shareByHash(hash string) (share.Share, bool) {
	if s.shares == nil || hash == "" {
		return share.Share{}, false
	}
	sh, err := s.shares.GetShare(hash)
	if err != nil {
		return share.Share{}, false
	}
	return sh, true
}

func (s *Service) updateAccessRulesOnMove(sourcePath string, oldPath, newPath utils.IndexPath) {
	if s.access == nil {
		return
	}
	_, _ = s.access.UpdateAccessRulesOnMove(sourcePath, oldPath, newPath)
}

// CheckPermissions validates user access and returns the resolved index path and user scope.
func (s *Service) CheckPermissions(opts utils.FileOptions, user *users.User) (string, string, error) {
	return checkPermissionsImpl(opts, user, s)
}

// FileInfoFaster returns extended file info with access filtering applied.
func (s *Service) FileInfoFaster(opts utils.FileOptions, user *users.User) (*iteminfo.ExtendedFileInfo, error) {
	return fileInfoFasterImpl(opts, user, s)
}

// GetDirItems lists directory entries visible to the user.
func (s *Service) GetDirItems(opts utils.FileOptions, user *users.User) (Items, error) {
	return getDirItemsImpl(opts, user, s)
}

// MoveResource moves a file or directory and updates access rules when needed.
func (s *Service) MoveResource(isSrcDir bool, sourceIndex, destIndex, realsrc, realdst string) error {
	return moveResourceImpl(isSrcDir, sourceIndex, destIndex, realsrc, realdst, s)
}
