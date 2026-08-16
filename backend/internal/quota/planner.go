package quota

import (
	"os"

	quotadb "github.com/gtsteffaniak/filebrowser/backend/internal/database/quota"
	"github.com/gtsteffaniak/filebrowser/backend/internal/database/users"
	"github.com/gtsteffaniak/filebrowser/backend/internal/state"
	"github.com/gtsteffaniak/filebrowser/backend/pkg/settings"
)

// UploadContext describes an upload admission check.
type UploadContext struct {
	Principal       *users.User
	SourceName      string
	SourcePath      string
	DestIndexPath   string
	DestRealPath    string
	ShareHash       string
	ShareLimitBytes int64
	SessionID       string
	TotalSize       int64
	HasKnownSize    bool
}

// CheckUploadLength verifies size is known when quotas apply.
func CheckUploadLength(ctx UploadContext) error {
	shareLimit := ctx.ShareLimitBytes
	if !state.HasApplicableQuotas(ctx.Principal, ctx.SourcePath, ctx.DestIndexPath, ctx.ShareHash, shareLimit) {
		return nil
	}
	if !ctx.HasKnownSize || ctx.TotalSize < 0 {
		return newError(CodeLengthRequired, "", "", 0, 0, 0, "upload size required when storage quota applies")
	}
	return nil
}

// ReserveUpload reserves quota for an upload session.
func ReserveUpload(ctx UploadContext) error {
	delta := uploadDelta(ctx)
	if delta <= 0 {
		return nil
	}
	checks := state.BuildReserveChecks(ctx.Principal, ctx.SourceName, ctx.SourcePath, ctx.DestIndexPath, ctx.ShareHash, ctx.ShareLimitBytes, delta)
	if len(checks) == 0 {
		return nil
	}
	if err := state.ReserveQuota(ctx.SessionID, checks); err != nil {
		return mapReserveError(err, checks)
	}
	return nil
}

// CommitUpload commits quota after successful upload.
func CommitUpload(ctx UploadContext) error {
	delta := uploadDelta(ctx)
	if delta <= 0 {
		state.ReleaseQuota(ctx.SessionID)
		return nil
	}
	return state.CommitQuota(ctx.SessionID, delta)
}

// ReleaseUpload releases quota reservation on failure.
func ReleaseUpload(sessionID string) {
	state.ReleaseQuota(sessionID)
}

// PrincipalForUpload returns the user whose quota is charged.
func PrincipalForUpload(actor *users.User, shareOwnerID uint64, shareHash string) *users.User {
	if shareHash != "" && shareOwnerID != 0 {
		owner, err := state.GetUserByID(shareOwnerID)
		if err == nil {
			return &owner
		}
	}
	return actor
}

func uploadDelta(ctx UploadContext) int64 {
	if !ctx.HasKnownSize {
		return 0
	}
	oldSize := int64(0)
	if ctx.DestRealPath != "" {
		if stat, err := os.Stat(ctx.DestRealPath); err == nil && !stat.IsDir() {
			oldSize = stat.Size()
		}
	}
	delta := ctx.TotalSize - oldSize
	if delta < 0 {
		return 0
	}
	return delta
}

// ScopeQuotaForSource returns scope quota snapshot for sidebar.
func ScopeQuotaForSource(user *users.User, sourceName string) (quotadb.Snapshot, bool) {
	sourceInfo, ok := settings.Config.Server.NameToSource[sourceName]
	if !ok {
		return quotadb.Snapshot{}, false
	}
	for _, bs := range user.BackendScopes {
		if bs.Path != sourceInfo.Path || bs.Quota == nil || bs.Quota.LimitBytes <= 0 {
			continue
		}
		return state.ScopeQuotaSnapshot(user, sourceName, bs.Quota), true
	}
	return quotadb.Snapshot{}, false
}
