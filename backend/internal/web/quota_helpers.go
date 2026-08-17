package web

import (
	"fmt"
	"io"
	"net/http"
	"os"
	"path/filepath"

	"github.com/google/uuid"
	"github.com/gtsteffaniak/filebrowser/backend/internal/adapters/fs/files"
	"github.com/gtsteffaniak/filebrowser/backend/internal/adapters/fs/fileutils"
	"github.com/gtsteffaniak/filebrowser/backend/internal/database/users"
	"github.com/gtsteffaniak/filebrowser/backend/internal/quota"
	"github.com/gtsteffaniak/filebrowser/backend/internal/state"
	"github.com/gtsteffaniak/go-logger/logger"
	"github.com/gtsteffaniak/filebrowser/backend/pkg/indexing"
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
	return buildUploadQuotaContext(principal, sourceName, sourcePath, fullIndexPath, realPath, d.Share.Hash, shareLimit, sessionID, totalSize, hasTotalSize)
}

func buildUploadQuotaContext(principal *users.User, sourceName, sourcePath, fullIndexPath, realPath, shareHash string, shareLimit int64, sessionID string, totalSize int64, hasTotalSize bool) quota.UploadContext {
	return quota.UploadContext{
		Principal:       principal,
		SourceName:      sourceName,
		SourcePath:      sourcePath,
		DestIndexPath:   fullIndexPath,
		DestRealPath:    realPath,
		ShareHash:       shareHash,
		ShareLimitBytes: shareLimit,
		SessionID:       sessionID,
		TotalSize:       totalSize,
		HasKnownSize:    hasTotalSize,
	}
}

// putResourceWithQuota writes PUT body bytes with upload-style quota reserve/commit on the destination path.
func putResourceWithQuota(w http.ResponseWriter, r *http.Request, d *Context, sourceName, fullIndexPath, realPath string) (int, error) {
	totalSize, hasTotalSize, err := parsePutTotalSize(r)
	if err != nil {
		return http.StatusBadRequest, err
	}

	sessionID := newQuotaSessionID()
	quotaCtx := uploadQuotaContext(d, sourceName, fullIndexPath, realPath, r, sessionID, totalSize, hasTotalSize)
	quotaReserved := false
	defer func() {
		if quotaReserved {
			releaseUploadQuota(sessionID)
		}
	}()

	if quotaErr := checkUploadQuota(quotaCtx); quotaErr != nil {
		return renderQuotaError(w, r, quotaErr)
	}
	quotaReserved = true

	if err = os.MkdirAll(filepath.Dir(realPath), fileutils.EffectiveDirPerm()); err != nil {
		logger.Debugf("could not create parent directory: %v", err)
		return http.StatusInternalServerError, fmt.Errorf("could not create parent directory: %v", err)
	}

	tempFilePath := uploadTempPath(realPath, sessionID)
	outFile, err := os.OpenFile(tempFilePath, os.O_CREATE|os.O_WRONLY|os.O_TRUNC, fileutils.PermFile)
	if err != nil {
		logger.Debugf("could not open temp file: %v", err)
		return http.StatusInternalServerError, fmt.Errorf("could not open temp file: %v", err)
	}

	written, copyErr := io.Copy(outFile, r.Body)
	closeErr := outFile.Close()
	if copyErr != nil {
		_ = os.Remove(tempFilePath)
		logger.Debugf("error writing file: %v", copyErr)
		return ErrToStatus(copyErr), copyErr
	}
	if closeErr != nil {
		_ = os.Remove(tempFilePath)
		logger.Debugf("error closing temp file: %v", closeErr)
		return http.StatusInternalServerError, closeErr
	}

	if err = validateReceivedBytes(written, totalSize, hasTotalSize, r.ContentLength); err != nil {
		_ = os.Remove(tempFilePath)
		logger.Debugf("%v", err)
		return http.StatusBadRequest, err
	}

	if err = fileutils.MoveFile(tempFilePath, realPath); err != nil {
		_ = os.Remove(tempFilePath)
		logger.Debugf("error writing file: %v", err)
		return ErrToStatus(err), err
	}
	if idx := indexing.GetIndex(sourceName); idx != nil && !idx.Config.ResolvedRules.IndexingDisabled {
		go files.RefreshIndex(sourceName, filepath.Dir(realPath), true, false) //nolint:errcheck
	}

	if commitErr := commitUploadQuotaAfterMove(quotaCtx); commitErr != nil {
		quotaReserved = false
		return renderQuotaError(w, r, commitErr)
	}
	quotaReserved = false
	return http.StatusOK, nil
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
