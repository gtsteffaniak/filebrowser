package web

import (
	"crypto/md5"
	"encoding/hex"
	"fmt"
	"net/http"
	"strconv"
)

// uploadTempPath returns the temporary path used while assembling an upload for realPath.
func uploadTempPath(realPath string) string {
	hasher := md5.New()
	hasher.Write([]byte(realPath))
	return fmt.Sprintf("%s.%s.uploading.tmp", realPath, hex.EncodeToString(hasher.Sum(nil)))
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
	return total, true, nil
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
