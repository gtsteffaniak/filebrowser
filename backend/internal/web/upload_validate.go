package web

import (
	"crypto/md5"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"math"
	"net/http"
	"os"
	"strconv"
	"time"

	"github.com/gtsteffaniak/go-logger/logger"
)

// uploadTempPath returns the temporary path used while assembling an upload for realPath.
// sessionID isolates concurrent uploads targeting the same destination.
func uploadTempPath(realPath, sessionID string) string {
	hasher := md5.New()
	hasher.Write([]byte(realPath))
	hasher.Write([]byte{0})
	hasher.Write([]byte(sessionID))
	return fmt.Sprintf("%s.%s.uploading.tmp", realPath, hex.EncodeToString(hasher.Sum(nil)))
}

// parsePutTotalSize returns the declared body size for PUT / in-place writes.
// Uses X-File-Total-Size when set, otherwise Content-Length when positive.
func parsePutTotalSize(r *http.Request) (total int64, ok bool, err error) {
	total, ok, err = parseUploadTotalSize(r)
	if err != nil || ok {
		return total, ok, err
	}
	if r.ContentLength > 0 {
		return r.ContentLength, true, nil
	}
	return 0, false, nil
}

// parseUploadTotalSize reads optional X-File-Total-Size.
// ok is false when the header is absent.
func parseUploadTotalSize(r *http.Request) (total int64, ok bool, err error) {
	s := r.Header.Get("X-File-Total-Size")
	if s == "" {
		return 0, false, nil
	}
	total, err = strconv.ParseInt(s, 10, 64)
	if err != nil {
		return 0, false, fmt.Errorf("invalid total size: %v", err)
	}
	if total < 0 {
		return 0, false, fmt.Errorf("invalid total size: %d", total)
	}
	return total, true, nil
}

// maxUploadMtimeSeconds is 9999-12-31T23:59:59Z, the last second time.Time formats safely.
const maxUploadMtimeSeconds = 253402300799

// parseUploadMtime reads the optional X-OC-Mtime header: the file's original
// modification time in Unix seconds, the same header WebDAV clients send.
// ok is false when the header is absent or does not hold a usable time.
func parseUploadMtime(r *http.Request) (mtime time.Time, ok bool) {
	raw := r.Header.Get("X-OC-Mtime")
	if raw == "" {
		return time.Time{}, false
	}
	seconds, err := strconv.ParseFloat(raw, 64)
	if err != nil || math.IsNaN(seconds) || seconds <= 0 || seconds > maxUploadMtimeSeconds {
		logger.Debugf("ignoring invalid X-OC-Mtime header %q", raw)
		return time.Time{}, false
	}
	return time.Unix(int64(seconds), 0), true
}

// applyUploadMtime sets the modification time requested by the client on a
// fully received upload. It is called on the temporary file before it is moved
// into place, so the destination never shows the upload time. A failure is
// logged and otherwise ignored: the upload itself succeeded.
func applyUploadMtime(r *http.Request, filePath string) {
	mtime, ok := parseUploadMtime(r)
	if !ok {
		return
	}
	if err := os.Chtimes(filePath, time.Now(), mtime); err != nil {
		logger.Debugf("could not set mtime on %s: %v", filePath, err)
	}
}

// validateChunkBounds rejects invalid offsets and sizes before writing a chunk.
// contentLength is the request Content-Length when known (>0).
func validateChunkBounds(offset, contentLength, totalSize int64) error {
	if offset < 0 {
		return fmt.Errorf("invalid chunk offset: %d", offset)
	}
	if totalSize < 0 {
		return fmt.Errorf("invalid total size: %d", totalSize)
	}
	if offset > totalSize {
		return fmt.Errorf("chunk offset exceeds total size: offset %d > %d", offset, totalSize)
	}
	remaining := totalSize - offset
	if contentLength > 0 {
		if contentLength > remaining {
			return fmt.Errorf("chunk exceeds total size: offset %d + %d > %d", offset, contentLength, totalSize)
		}
		if offset > math.MaxInt64-contentLength {
			return fmt.Errorf("chunk offset overflow")
		}
	}
	return nil
}

// validateAssembledSize checks written chunk bytes against the declared total without overflow.
func validateAssembledSize(offset, chunkSize, totalSize int64) error {
	if chunkSize < 0 {
		return fmt.Errorf("invalid chunk size: %d", chunkSize)
	}
	if err := validateChunkBounds(offset, chunkSize, totalSize); err != nil {
		return err
	}
	return nil
}

// validateReceivedBytes compares bytes written to optional declared totals.
// expectedTotal is checked only when hasExpected is true.
// contentLength is checked when greater than 0 (known Content-Length).
func validateReceivedBytes(written, expectedTotal int64, hasExpected bool, contentLength int64) error {
	if hasExpected && written != expectedTotal {
		return fmt.Errorf("upload incomplete: expected %d bytes, received %d", expectedTotal, written)
	}
	if contentLength > 0 && written != contentLength {
		return fmt.Errorf("upload incomplete: expected %d bytes, received %d", contentLength, written)
	}
	return nil
}

// isContentUpload reports whether the request is uploading file bytes (as opposed to
// creating an empty file via POST with no upload metadata).
func isContentUpload(r *http.Request) bool {
	if r.Header.Get("X-File-Chunk-Offset") != "" {
		return true
	}
	if r.Header.Get(uploadSessionHeader) != "" {
		return true
	}
	_, ok, err := parseUploadTotalSize(r)
	return err == nil && ok
}

// rollbackChunkForResume truncates a partial chunk write back to offset and syncs.
// It returns false when rollback cannot be confirmed, in which case tempFilePath is removed.
func rollbackChunkForResume(outFile *os.File, tempFilePath string, offset int64) bool {
	if truncErr := outFile.Truncate(offset); truncErr != nil {
		_ = os.Remove(tempFilePath)
		return false
	}
	if syncErr := outFile.Sync(); syncErr != nil {
		_ = os.Remove(tempFilePath)
		return false
	}
	return true
}

// abortConflictingUpload closes an upload body without draining it and marks the
// response connection for closure so the server can return immediately.
func abortConflictingUpload(w http.ResponseWriter, r *http.Request) {
	if r.Body != nil {
		_ = r.Body.Close()
	}
	w.Header().Set("Connection", "close")
}

type chunkUploadResponse struct {
	Offset   int64 `json:"offset"`
	Total    int64 `json:"total"`
	Complete bool  `json:"complete"`
}

func writeChunkUploadResponse(w http.ResponseWriter, offset, total int64, complete bool) error {
	w.Header().Set("Connection", "close")
	w.Header().Set("Content-Type", "application/json; charset=utf-8")
	return json.NewEncoder(w).Encode(chunkUploadResponse{
		Offset:   offset,
		Total:    total,
		Complete: complete,
	})
}
