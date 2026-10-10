package web

import (
	"fmt"
	"io"
	"net/http"
	"net/url"
	"os"
	"path"
	"path/filepath"
	"sort"
	"strconv"
	"strings"

	"github.com/gtsteffaniak/go-logger/logger"
	"golang.org/x/net/webdav"

	"github.com/gtsteffaniak/filebrowser/backend/internal/adapters/fs/fileutils"
)

// Chunked uploads over WebDAV, in the shape rclone's `--owncloud-chunk-size` sends, which is what
// clients need behind a proxy with a request size limit (`gtsteffaniak/filebrowser#2404`).
//
// Every chunk is an ordinary upload next to the destination, named `<name>-chunk-<n>` (some clients
// hide them with a leading dot), and the transfer ends with a MOVE of the last chunk to `<name>`:
//
//	PUT  /dav/<source>/<dir>/<name>-chunk-0
//	PUT  /dav/<source>/<dir>/<name>-chunk-1
//	MOVE /dav/<source>/<dir>/<name>-chunk-1   Destination: /dav/<source>/<dir>/<name>
//
// The chunks are concatenated on that final MOVE. Nothing is uploaded into a namespace, so a
// transfer in flight is visible in the listing, and an abandoned one leaves its chunks behind (the
// client's retry overwrites them); the alternative, a hidden staging namespace, cannot be told apart
// from a real folder named `uploads`.
//
// The assembled file is placed by the WebDAV handler itself, so locks, `Overwrite`, the
// 201/204/412 choice and the containment of the destination under the scope stay the library's job.
const owncloudChunkSuffix = "-chunk-"

// owncloudChunk is one chunk file of an ownCloud-style upload, with the number in its name.
type owncloudChunk struct {
	number int
	name   string
}

// owncloudChunkParts reports the destination an ownCloud chunk belongs to: `/dir/file.mp4-chunk-3`
// is chunk 3 of `/dir/file.mp4`.
func owncloudChunkParts(requestPath string) (base string, ok bool) {
	idx := strings.LastIndex(requestPath, owncloudChunkSuffix)
	if idx <= 0 || !isUploadChunkNumber(requestPath[idx+len(owncloudChunkSuffix):]) {
		return "", false
	}
	return requestPath[:idx], true
}

func isUploadChunkNumber(name string) bool {
	if name == "" || len(name) > 9 {
		return false
	}
	for _, c := range name {
		if c < '0' || c > '9' {
			return false
		}
	}
	return true
}

// owncloudChunkedMove handles the final MOVE of an ownCloud-style chunked upload, and reports
// handled=false when the request is an ordinary move — including the move of a real file whose name
// merely ends in `-chunk-<n>` and has no sibling chunks on disk.
func owncloudChunkedMove(w http.ResponseWriter, r *http.Request, wd *webdav.Handler, requestPath, scopePath, prefix string) (handled bool, status int, err error) {
	// the handler normalizes request paths with a trailing slash, the chunk number is before it
	base, ok := owncloudChunkParts(strings.TrimSuffix(requestPath, "/"))
	if !ok {
		return false, 0, nil
	}
	base = path.Clean("/" + base)
	destPath, err := uploadDestinationPath(r, prefix)
	if err != nil {
		return true, http.StatusBadRequest, err
	}
	if destPath != base {
		// the client is moving the chunk somewhere else: not a chunked upload
		return false, 0, nil
	}
	realDir := filepath.Dir(filepath.Join(scopePath, destPath))
	chunks, err := collectOwncloudChunks(realDir, path.Base(destPath))
	if err != nil || len(chunks) < 2 {
		return false, 0, nil
	}
	for i, chunk := range chunks {
		if chunk.number != i {
			return true, http.StatusConflict, fmt.Errorf("chunked upload is missing chunk %d of %d", i, chunks[len(chunks)-1].number)
		}
	}

	// The assembled file is a random hidden name in the destination directory: it must not be able to
	// collide with something the user already has (O_EXCL), and staying in that directory is what lets
	// the library below move it with a plain rename.
	tmp, err := os.CreateTemp(realDir, "."+path.Base(destPath)+".assembled-*")
	if err != nil {
		return true, http.StatusInternalServerError, err
	}
	realAssembled := tmp.Name()
	if err := tmp.Chmod(fileutils.EffectiveFilePerm()); err != nil {
		tmp.Close()
		os.Remove(realAssembled)
		return true, http.StatusInternalServerError, err
	}
	written, err := writeOwncloudChunks(realDir, chunks, tmp)
	if err != nil {
		os.Remove(realAssembled)
		return true, http.StatusInternalServerError, err
	}
	if total := r.Header.Get("OC-Total-Length"); total != "" {
		want, convErr := strconv.ParseInt(total, 10, 64)
		if convErr != nil || want != written {
			// keep the chunks: the client can retry the move with the missing bytes
			os.Remove(realAssembled)
			return true, http.StatusBadRequest, fmt.Errorf("assembled %d bytes, but OC-Total-Length is %q", written, total)
		}
	}

	// Move the assembled file the way any other WebDAV move is done: the library cleans the paths,
	// confirms the locks, honours `Overwrite` and answers 201/204/412 itself.
	assembled := path.Join(path.Dir(destPath), filepath.Base(realAssembled))
	original := r.URL.Path
	r.URL.Path = prefix + assembled
	sw := &statusResponseWriter{ResponseWriter: w, status: http.StatusOK}
	wd.ServeHTTP(sw, r)
	r.URL.Path = original
	if sw.status >= 300 {
		os.Remove(realAssembled)
		return true, 0, nil
	}
	for _, chunk := range chunks {
		if err := os.Remove(filepath.Join(realDir, chunk.name)); err != nil {
			logger.Debugf("webdav chunked upload: could not remove chunk %s: %v", chunk.name, err)
		}
	}
	logger.Debugf("webdav chunked upload: assembled %d chunks (%d bytes) into %s", len(chunks), written, destPath)
	return true, 0, nil
}

// collectOwncloudChunks lists the chunk files of one destination, lowest number first. Clients name
// them `<name>-chunk-<n>`; a leading dot is accepted as well, as some clients hide them.
//
// shortcut: two real files `x-chunk-0` and `x-chunk-1` do get concatenated when the last of them is
// moved to `x`; the protocol has no way to tell that intent apart, so the client's naming wins.
func collectOwncloudChunks(realDir, baseName string) ([]owncloudChunk, error) {
	entries, err := os.ReadDir(realDir)
	if err != nil {
		return nil, err
	}
	prefixes := []string{baseName + owncloudChunkSuffix, "." + baseName + owncloudChunkSuffix}
	var chunks []owncloudChunk
	for _, entry := range entries {
		if entry.IsDir() {
			continue
		}
		for _, p := range prefixes {
			number, found := strings.CutPrefix(entry.Name(), p)
			if !found || !isUploadChunkNumber(number) {
				continue
			}
			n, _ := strconv.Atoi(number)
			chunks = append(chunks, owncloudChunk{number: n, name: entry.Name()})
			break
		}
	}
	sort.Slice(chunks, func(i, j int) bool {
		return chunks[i].number < chunks[j].number
	})
	return chunks, nil
}

func writeOwncloudChunks(realDir string, chunks []owncloudChunk, out *os.File) (int64, error) {
	var written int64
	for _, chunk := range chunks {
		in, err := os.Open(filepath.Join(realDir, chunk.name))
		if err != nil {
			out.Close()
			return written, err
		}
		n, copyErr := io.Copy(out, in)
		in.Close()
		written += n
		if copyErr != nil {
			out.Close()
			return written, copyErr
		}
	}
	return written, out.Close()
}

// uploadDestinationPath reads the Destination header of the final MOVE and returns the path inside
// the source, the way the WebDAV client sees it (relative to the user's scope). The path is cleaned
// the same way golang.org/x/net/webdav cleans it, i.e. anchored at the scope: a client cannot walk
// out of its scope with `..`, and the caller gets exactly the path the library would have used.
func uploadDestinationPath(r *http.Request, prefix string) (string, error) {
	raw := r.Header.Get("Destination")
	if raw == "" {
		return "", fmt.Errorf("missing Destination header")
	}
	u, err := url.Parse(raw)
	if err != nil {
		return "", fmt.Errorf("invalid Destination header")
	}
	rest, ok := strings.CutPrefix(u.Path, prefix)
	if !ok || (rest != "" && rest[0] != '/') {
		return "", fmt.Errorf("Destination is outside this source")
	}
	rest = path.Clean("/" + rest)
	if rest == "/" || rest == "." {
		return "", fmt.Errorf("Destination is empty")
	}
	return rest, nil
}
