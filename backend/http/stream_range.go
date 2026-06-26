package http

import (
	"errors"
	"fmt"
	"io"
	"mime"
	"net/http"
	"strconv"
	"strings"
)

// maxStreamRangeBytes caps each partial response on the stream endpoint so clients
// cannot retrieve a full file in one request (e.g. browser "Save as" on a stream URL).
const maxStreamRangeBytes = 4 << 20 // 4 MiB

var errStreamRangeInvalid = errors.New("invalid byte range")

// streamUseRangeOnly reports whether the stream endpoint must serve capped partial
// content only (never a full-file 200 response).
func streamUseRangeOnly(d *requestContext, displayFileName string) bool {
	if isMediaStreamFile(displayFileName) {
		return true
	}
	if d.share.Hash != "" && d.share.DisableDownload {
		return true
	}
	if d.share.Hash == "" && d.user != nil && !d.user.Permissions.Download {
		return true
	}
	return false
}

func isMediaStreamFile(displayFileName string) bool {
	contentType := mime.TypeByExtension(strings.ToLower(filepathExt(displayFileName)))
	return strings.HasPrefix(contentType, "video/") || strings.HasPrefix(contentType, "audio/")
}

func filepathExt(name string) string {
	if i := strings.LastIndex(name, "."); i >= 0 {
		return name[i:]
	}
	return ""
}

func parseStreamByteRange(rangeHeader string, size int64) (start, end int64, err error) {
	if size <= 0 {
		return 0, 0, errStreamRangeInvalid
	}
	if !strings.HasPrefix(rangeHeader, "bytes=") {
		return 0, 0, errStreamRangeInvalid
	}
	spec := strings.TrimPrefix(rangeHeader, "bytes=")
	if spec == "" || strings.Contains(spec, ",") {
		return 0, 0, errStreamRangeInvalid
	}

	dash := strings.Index(spec, "-")
	if dash < 0 {
		return 0, 0, errStreamRangeInvalid
	}
	startStr := strings.TrimSpace(spec[:dash])
	endStr := strings.TrimSpace(spec[dash+1:])

	if startStr == "" {
		// suffix range: bytes=-500
		suffix, parseErr := strconv.ParseInt(endStr, 10, 64)
		if parseErr != nil || suffix <= 0 {
			return 0, 0, errStreamRangeInvalid
		}
		if suffix > size {
			suffix = size
		}
		start = size - suffix
		end = size - 1
		return start, end, nil
	}

	start, err = strconv.ParseInt(startStr, 10, 64)
	if err != nil || start < 0 || start >= size {
		return 0, 0, errStreamRangeInvalid
	}

	if endStr == "" {
		end = size - 1
	} else {
		end, err = strconv.ParseInt(endStr, 10, 64)
		if err != nil || end < start {
			return 0, 0, errStreamRangeInvalid
		}
		if end >= size {
			end = size - 1
		}
	}
	return start, end, nil
}

func capStreamByteRange(start, end int64) (int64, int64) {
	if end-start+1 <= maxStreamRangeBytes {
		return start, end
	}
	return start, start + maxStreamRangeBytes - 1
}

func setStreamResponseHeaders(w http.ResponseWriter, r *http.Request, displayFileName string, size int64) {
	setContentDisposition(w, r, displayFileName, true)
	w.Header().Set("Accept-Ranges", "bytes")
	w.Header().Set("Cache-Control", "private")
	w.Header().Set("X-Content-Type-Options", "nosniff")
	if contentType := mime.TypeByExtension(strings.ToLower(filepathExt(displayFileName))); contentType != "" {
		w.Header().Set("Content-Type", contentType)
	}
	w.Header().Set("Content-Length", strconv.FormatInt(size, 10))
}

func serveStreamByteRange(w http.ResponseWriter, r *http.Request, reader io.ReadSeeker, displayFileName string, size int64) (int, error) {
	if r.Method == http.MethodHead {
		setStreamResponseHeaders(w, r, displayFileName, size)
		return http.StatusOK, nil
	}

	rangeHeader := r.Header.Get("Range")
	if rangeHeader == "" {
		setStreamResponseHeaders(w, r, displayFileName, size)
		w.Header().Set("Content-Range", fmt.Sprintf("bytes */%d", size))
		return http.StatusRequestedRangeNotSatisfiable, fmt.Errorf("stream requires byte range requests")
	}

	start, end, err := parseStreamByteRange(rangeHeader, size)
	if err != nil {
		setStreamResponseHeaders(w, r, displayFileName, size)
		w.Header().Set("Content-Range", fmt.Sprintf("bytes */%d", size))
		return http.StatusRequestedRangeNotSatisfiable, fmt.Errorf("invalid byte range")
	}

	start, end = capStreamByteRange(start, end)

	chunkSize := end - start + 1
	if _, err := reader.Seek(start, io.SeekStart); err != nil {
		return http.StatusInternalServerError, err
	}

	setContentDisposition(w, r, displayFileName, true)
	w.Header().Set("Accept-Ranges", "bytes")
	w.Header().Set("Cache-Control", "private")
	w.Header().Set("X-Content-Type-Options", "nosniff")
	if contentType := mime.TypeByExtension(strings.ToLower(filepathExt(displayFileName))); contentType != "" {
		w.Header().Set("Content-Type", contentType)
	}
	w.Header().Set("Content-Range", fmt.Sprintf("bytes %d-%d/%d", start, end, size))
	w.Header().Set("Content-Length", strconv.FormatInt(chunkSize, 10))
	w.WriteHeader(http.StatusPartialContent)

	if _, err := io.CopyN(w, reader, chunkSize); err != nil && !errors.Is(err, io.EOF) {
		return http.StatusPartialContent, err
	}
	return http.StatusPartialContent, nil
}
