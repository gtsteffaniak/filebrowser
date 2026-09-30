package web

import (
	"errors"
	"fmt"
	"io"
	"net/http"
	"os"
	"path/filepath"
	"strings"

	"github.com/google/uuid"
	"github.com/gtsteffaniak/filebrowser/backend/internal/adapters/fs/files"
	"github.com/gtsteffaniak/filebrowser/backend/internal/adapters/fs/fileutils"
	"github.com/gtsteffaniak/filebrowser/backend/internal/database/users"
	commonerrors "github.com/gtsteffaniak/filebrowser/backend/internal/errors"
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

// resolvePutRealPath maps an index path to a real filesystem path for PUT.
// Existing files resolve directly; missing final components resolve via the parent directory.
func resolvePutRealPath(idx *indexing.Index, fullIndexPath string) (string, error) {
	realPath, isDir, err := idx.GetRealPath(fullIndexPath)
	if err == nil {
		if isDir {
			return "", fmt.Errorf("path is a directory")
		}
		return realPath, nil
	}
	if !isMissingPutTargetError(err) {
		return "", err
	}

	parentIndexPath := filepath.Dir(fullIndexPath)
	if parentIndexPath == "." || parentIndexPath == "" {
		parentIndexPath = "/"
	}
	parentReal, isDir, parentErr := idx.GetRealPath(parentIndexPath)
	if parentErr != nil {
		return "", parentErr
	}
	if !isDir {
		return "", fmt.Errorf("parent path is not a directory")
	}
	baseName := filepath.Base(strings.TrimSuffix(fullIndexPath, "/"))
	if baseName == "" || baseName == "." {
		return "", fmt.Errorf("invalid file path")
	}
	return filepath.Join(parentReal, baseName), nil
}

var (
	errPutAccessDenied     = fmt.Errorf("access denied")
	errPutDestDirMissing   = fmt.Errorf("destination directory does not exist")
	errPutPathIsDirectory  = fmt.Errorf("path is a directory")
	errPutUpdateFailed     = fmt.Errorf("an error occurred while updating the resource")
	errPutIncompleteUpload = fmt.Errorf("upload incomplete: received bytes do not match declared size")
)

// sanitizePutPathError maps path resolution failures to generic client errors (no filesystem paths).
func sanitizePutPathError(err error) (int, error) {
	if err == nil {
		return http.StatusOK, nil
	}
	logger.Debugf("put path resolution failed: %v", err)
	if errors.Is(err, commonerrors.ErrPathEscapesScope) {
		return http.StatusForbidden, errPutAccessDenied
	}
	if strings.Contains(err.Error(), "path is a directory") {
		return http.StatusMethodNotAllowed, errPutPathIsDirectory
	}
	if os.IsNotExist(err) || errors.Is(err, commonerrors.ErrNotExist) {
		return http.StatusNotFound, errPutDestDirMissing
	}
	if strings.Contains(err.Error(), "parent path is not a directory") ||
		strings.Contains(err.Error(), "invalid file path") ||
		isMissingPutTargetError(err) {
		return http.StatusNotFound, errPutDestDirMissing
	}
	return http.StatusNotFound, errPutDestDirMissing
}

func sanitizePutWriteError(err error) (int, error) {
	if err == nil {
		return http.StatusOK, nil
	}
	logger.Debugf("put write failed: %v", err)
	return http.StatusInternalServerError, errPutUpdateFailed
}

func sanitizePutFinalizeError(err error) (int, error) {
	if err == nil {
		return http.StatusOK, nil
	}
	if strings.Contains(err.Error(), "upload incomplete") {
		return http.StatusBadRequest, errPutIncompleteUpload
	}
	return sanitizePutWriteError(err)
}

// finalizeQuotaPut validates staged bytes, moves the temp file into place, refreshes the index, and commits quota.
// The bool is true when the quota reservation should still be released by the caller.
func finalizeQuotaPut(quotaCtx quota.UploadContext, tempPath, realPath, sourceName string, written, totalSize int64, hasTotalSize bool, contentLength int64) (bool, error) {
	if err := validateReceivedBytes(written, totalSize, hasTotalSize, contentLength); err != nil {
		_ = os.Remove(tempPath)
		return true, err
	}
	if err := fileutils.MoveFile(tempPath, realPath); err != nil {
		_ = os.Remove(tempPath)
		return true, err
	}
	if shouldRefreshIndexAfterPut(sourceName) {
		go files.RefreshIndex(sourceName, filepath.Dir(realPath), true, false) //nolint:errcheck
	}
	if err := commitUploadQuotaAfterMove(quotaCtx); err != nil {
		return false, err
	}
	return false, nil
}

func shouldRefreshIndexAfterPut(sourceName string) bool {
	idx := indexing.GetIndex(sourceName)
	if idx == nil || indexing.GetIndexDB() == nil {
		return false
	}
	return !idx.Config.ResolvedRules.IndexingDisabled
}

func isMissingPutTargetError(err error) bool {
	if err == nil {
		return false
	}
	if os.IsNotExist(err) {
		return true
	}
	msg := err.Error()
	return strings.Contains(msg, "could not stat resolved path") ||
		strings.Contains(msg, "could not resolve symlinks")
}

// putResourceWithQuota writes PUT body bytes with upload-style quota reserve/commit on the destination path.
func putResourceWithQuota(w http.ResponseWriter, r *http.Request, d *Context, sourceName, fullIndexPath string) (int, error) {
	idx := indexing.GetIndex(sourceName)
	if idx == nil {
		return http.StatusNotFound, fmt.Errorf("source not found")
	}
	realPath, err := resolvePutRealPath(idx, fullIndexPath)
	if err != nil {
		return sanitizePutPathError(err)
	}

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
		return sanitizePutWriteError(err)
	}

	tempFilePath := uploadTempPath(realPath, sessionID)
	outFile, err := os.OpenFile(tempFilePath, os.O_CREATE|os.O_WRONLY|os.O_TRUNC, fileutils.PermFile)
	if err != nil {
		return sanitizePutWriteError(err)
	}

	written, copyErr := io.Copy(outFile, r.Body)
	closeErr := outFile.Close()
	if copyErr != nil {
		_ = os.Remove(tempFilePath)
		return sanitizePutWriteError(copyErr)
	}
	if closeErr != nil {
		_ = os.Remove(tempFilePath)
		return sanitizePutWriteError(closeErr)
	}

	stillReserved, finalizeErr := finalizeQuotaPut(quotaCtx, tempFilePath, realPath, sourceName, written, totalSize, hasTotalSize, r.ContentLength)
	if finalizeErr != nil {
		if !stillReserved {
			quotaReserved = false
		}
		if _, ok := quota.AsError(finalizeErr); ok {
			return renderQuotaError(w, r, finalizeErr)
		}
		return sanitizePutFinalizeError(finalizeErr)
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
	status, renderErr := RenderJSON(w, r, payload, status)
	if renderErr != nil {
		return status, renderErr
	}
	return status, nil
}
