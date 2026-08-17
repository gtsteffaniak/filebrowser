package web

import (
	"net/http"

	"github.com/google/uuid"
	"github.com/gtsteffaniak/filebrowser/backend/internal/quota"
	"github.com/gtsteffaniak/filebrowser/backend/internal/state"
	"github.com/gtsteffaniak/go-logger/logger"
	"github.com/gtsteffaniak/filebrowser/backend/pkg/settings"
)

func uploadQuotaContext(d *Context, sourceName, fullIndexPath, realPath string, r *http.Request, sessionID string, totalSize int64, hasTotalSize bool) quota.UploadContext {
	sourcePath := ""
	if sourceInfo, ok := settings.Config.Server.NameToSource[sourceName]; ok {
		sourcePath = sourceInfo.Path
	}
	principal := quota.PrincipalForUpload(d.User, d.Share.UserID, d.Share.Hash)
	shareLimit := int64(0)
	if d.Share.Hash != "" {
		shareLimit = d.Share.QuotaLimitBytes
	}
	return quota.UploadContext{
		Principal:       principal,
		SourceName:      sourceName,
		SourcePath:      sourcePath,
		DestIndexPath:   fullIndexPath,
		DestRealPath:    realPath,
		ShareHash:       d.Share.Hash,
		ShareLimitBytes: shareLimit,
		SessionID:       sessionID,
		TotalSize:       totalSize,
		HasKnownSize:    hasTotalSize,
	}
}

func checkUploadQuota(ctx quota.UploadContext) error {
	if err := quota.CheckUploadLength(ctx); err != nil {
		return err
	}
	return quota.ReserveUpload(ctx)
}

func commitUploadQuotaAfterMove(ctx quota.UploadContext) error {
	if err := quota.CommitUpload(ctx); err != nil {
		logger.Warningf("quota commit after upload failed, retrying: %v", err)
		delta := quota.UploadCommitDelta(ctx)
		if retryErr := state.ForceCommitSessionQuota(ctx.SessionID, delta); retryErr != nil {
			return retryErr
		}
	}
	return nil
}

func newQuotaSessionID() string {
	return uuid.New().String()
}

func releaseUploadQuota(sessionID string) {
	quota.ReleaseUpload(sessionID)
}

func copyMoveQuotaContext(d *Context, action, fromSourceName, toSourceName, fromSourcePath, toSourcePath, fromIndexPath, toIndexPath string, isSrcDir bool, overwriteBytes int64, sessionID string) quota.CopyMoveContext {
	principal := quota.PrincipalForUpload(d.User, d.Share.UserID, d.Share.Hash)
	shareLimit := int64(0)
	if d.Share.Hash != "" {
		shareLimit = d.Share.QuotaLimitBytes
	}
	return quota.CopyMoveContext{
		Principal:       principal,
		Action:          action,
		FromSourceName:  fromSourceName,
		ToSourceName:    toSourceName,
		FromSourcePath:  fromSourcePath,
		ToSourcePath:    toSourcePath,
		FromIndexPath:   fromIndexPath,
		ToIndexPath:     toIndexPath,
		IsSrcDir:        isSrcDir,
		OverwriteBytes:  overwriteBytes,
		ShareHash:       d.Share.Hash,
		ShareLimitBytes: shareLimit,
		SessionID:       sessionID,
	}
}

func checkCopyMoveQuota(ctx quota.CopyMoveContext) error {
	return quota.ReserveCopyMove(ctx)
}

func commitCopyMoveQuota(sessionID string) error {
	return quota.CommitCopyMove(sessionID)
}

func releaseCopyMoveQuota(sessionID string) {
	quota.ReleaseCopyMove(sessionID)
}

// renderQuotaError writes a JSON quota error response.
func renderQuotaError(w http.ResponseWriter, r *http.Request, err error) (int, error) {
	qe, ok := quota.AsError(err)
	if !ok {
		return ErrToStatus(err), err
	}
	status := http.StatusInsufficientStorage
	if qe.Code == quota.CodeLengthRequired {
		status = http.StatusBadRequest
	}
	payload := map[string]interface{}{
		"code":          qe.Code,
		"quotaKind":     qe.QuotaKind,
		"limitBytes":    qe.LimitBytes,
		"usedBytes":     qe.UsedBytes,
		"reservedBytes": qe.ReservedBytes,
		"message":       qe.DisplayMessage(),
	}
	return RenderJSON(w, r, payload, status)
}
