package quota

import (
	"github.com/gtsteffaniak/filebrowser/backend/internal/database/users"
	"github.com/gtsteffaniak/filebrowser/backend/internal/state"
)

// CopyMoveContext describes a copy/move/rename admission check.
type CopyMoveContext struct {
	Principal       *users.User
	Action          string
	FromSourceName  string
	ToSourceName    string
	FromSourcePath  string
	ToSourcePath    string
	FromIndexPath   string
	ToIndexPath     string
	IsSrcDir        bool
	OverwriteBytes  int64
	ShareHash       string
	ShareLimitBytes int64
	SessionID       string
}

// ReserveCopyMove reserves quota before a copy/move/rename.
func ReserveCopyMove(ctx CopyMoveContext) error {
	itemSize, ok := state.IndexedPathBytes(ctx.FromSourceName, ctx.FromIndexPath, ctx.IsSrcDir)
	if !ok {
		if state.HasApplicableQuotas(ctx.Principal, ctx.ToSourcePath, ctx.ToIndexPath, ctx.ShareHash, ctx.ShareLimitBytes) {
			return newError(CodeUsageUnknown, "", "", 0, 0, 0, "indexed size unavailable for quota check")
		}
		return nil
	}
	checks := state.BuildCopyMoveReserveChecks(
		ctx.Principal,
		ctx.ToSourceName,
		ctx.ToSourcePath,
		ctx.FromIndexPath,
		ctx.ToIndexPath,
		ctx.Action,
		itemSize,
		ctx.OverwriteBytes,
		ctx.ShareHash,
		ctx.ShareLimitBytes,
	)
	if len(checks) == 0 {
		return nil
	}
	if err := state.ReserveQuota(ctx.SessionID, checks); err != nil {
		return mapReserveError(err, checks)
	}
	return nil
}

// CommitCopyMove commits reserved quota after a successful copy/move/rename.
func CommitCopyMove(sessionID string) error {
	return state.CommitQuota(sessionID, 0)
}

// ReleaseCopyMove releases quota reservation on failure.
func ReleaseCopyMove(sessionID string) {
	state.ReleaseQuota(sessionID)
}

// OverwriteBytesAtPath returns indexed bytes at dest when replacing an existing path.
func OverwriteBytesAtPath(sourceName, indexPath string, isDir bool) int64 {
	size, ok := state.IndexedPathBytes(sourceName, indexPath, isDir)
	if !ok {
		return 0
	}
	return size
}
