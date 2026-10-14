package web

import (
	"fmt"
	"io"
	"net/http"
	"net/url"
	"os"
	"path"
	"path/filepath"
	"slices"
	"sort"
	"strconv"
	"strings"
	"sync"

	"github.com/gtsteffaniak/go-logger/logger"
	"golang.org/x/net/webdav"

	"github.com/gtsteffaniak/filebrowser/backend/internal/activity"
	"github.com/gtsteffaniak/filebrowser/backend/internal/adapters/fs/fileutils"
	activitydb "github.com/gtsteffaniak/filebrowser/backend/internal/database/activity"
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

// owncloudChunkParts reports the destination an ownCloud chunk belongs to and which chunk it is:
// `/dir/file.mp4-chunk-3` is chunk 3 of `/dir/file.mp4`.
func owncloudChunkParts(requestPath string) (base string, number int, ok bool) {
	idx := strings.LastIndex(requestPath, owncloudChunkSuffix)
	if idx <= 0 || !isUploadChunkNumber(requestPath[idx+len(owncloudChunkSuffix):]) {
		return "", 0, false
	}
	number, convErr := strconv.Atoi(requestPath[idx+len(owncloudChunkSuffix):])
	if convErr != nil {
		return "", 0, false
	}
	return requestPath[:idx], number, true
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

// owncloudAssemblies tracks the real destination paths with an assembly in flight, so two
// concurrent final MOVEs for the same file cannot interleave their chunks.
var (
	owncloudAssembliesMu sync.Mutex
	owncloudAssemblies   = map[string]struct{}{}
)

func acquireOwncloudAssembly(realDest string) bool {
	owncloudAssembliesMu.Lock()
	defer owncloudAssembliesMu.Unlock()
	if _, ok := owncloudAssemblies[realDest]; ok {
		return false
	}
	owncloudAssemblies[realDest] = struct{}{}
	return true
}

func releaseOwncloudAssembly(realDest string) {
	owncloudAssembliesMu.Lock()
	defer owncloudAssembliesMu.Unlock()
	delete(owncloudAssemblies, realDest)
}

// owncloudChunkedMove handles the final MOVE of an ownCloud-style chunked upload, and reports
// handled=false when the request is an ordinary move — including the move of a real file whose name
// merely ends in `-chunk-<n>` and has no sibling chunks on disk.
func owncloudChunkedMove(w http.ResponseWriter, r *http.Request, wd *webdav.Handler, ffs *filteredFileSystem, requestPath, scopePath, prefix string) (handled bool, status int, err error) {
	// the handler normalizes request paths with a trailing slash, the chunk number is before it
	sourceName := path.Base(strings.TrimSuffix(requestPath, "/"))
	base, moved, ok := owncloudChunkParts(strings.TrimSuffix(requestPath, "/"))
	if !ok {
		return false, 0, nil
	}
	base = path.Clean("/" + base)
	destPath, err := uploadDestinationPath(r, prefix)
	if err != nil {
		return true, http.StatusBadRequest, err
	}
	if destPath != base {
		// a dot-prefixed chunk name is the hidden variant of the same transfer
		if !strings.HasPrefix(path.Base(base), ".") || destPath != path.Join(path.Dir(base), strings.TrimPrefix(path.Base(base), ".")) {
			// the client is moving the chunk somewhere else: not a chunked upload
			return false, 0, nil
		}
	}
	realDest := filepath.Join(scopePath, destPath)
	realDir := filepath.Dir(realDest)
	chunks, err := collectOwncloudChunks(realDir, path.Base(destPath))
	if err != nil {
		return false, 0, nil
	}
	// the moved chunk closes the transfer: a higher number is a leftover of an earlier, longer upload
	// of the same name, and assembling it would mix two files together
	chunks = slices.DeleteFunc(chunks, func(chunk owncloudChunk) bool { return chunk.number > moved })
	// the chunk named in the request must be on disk, exactly under that name
	if !slices.ContainsFunc(chunks, func(chunk owncloudChunk) bool { return chunk.name == sourceName }) {
		return true, http.StatusNotFound, fmt.Errorf("chunk %s does not exist", sourceName)
	}
	if len(chunks) < 2 {
		return false, 0, nil
	}
	for i, chunk := range chunks {
		if i > 0 && chunk.number == chunks[i-1].number {
			return true, http.StatusConflict, fmt.Errorf("chunked upload has two files for chunk %d", chunk.number)
		}
		if chunk.number != i {
			return true, http.StatusConflict, fmt.Errorf("chunked upload is missing chunk %d of %d", i, chunks[len(chunks)-1].number)
		}
	}

	// assembling deletes the chunks afterwards, and every chunk must be one the user may touch;
	// check both up front so a denied transfer touches nothing
	if !ffs.sourceFilePerms().Delete {
		return true, http.StatusForbidden, fmt.Errorf("delete permission required")
	}
	destDir := path.Dir(destPath)
	for _, chunk := range chunks {
		if accessErr := ffs.checkAccess(path.Join(destDir, chunk.name)); accessErr != nil {
			return true, http.StatusForbidden, fmt.Errorf("chunked upload cannot access chunk %s: %w", chunk.name, accessErr)
		}
	}

	if !acquireOwncloudAssembly(realDest) {
		return true, http.StatusConflict, fmt.Errorf("another chunked upload is already assembling %s", destPath)
	}
	defer releaseOwncloudAssembly(realDest)

	// The assembled file is a random hidden name in the destination directory: it must not be able to
	// collide with something the user already has (O_EXCL), and staying in that directory is what lets
	// the library below move it with a plain rename.
	tmp, err := os.CreateTemp(realDir, "."+path.Base(destPath)+".assembled-*")
	if err != nil {
		return true, http.StatusInternalServerError, err
	}
	realAssembled := tmp.Name()
	if chmodErr := tmp.Chmod(fileutils.EffectiveFilePerm()); chmodErr != nil {
		tmp.Close()
		os.Remove(realAssembled)
		return true, http.StatusInternalServerError, chmodErr
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
	assembled := path.Join(destDir, filepath.Base(realAssembled))
	// the library move of the hidden staging file is not a user-visible move: skip its activity
	// record and record one upload for the real destination instead
	ffs.skipRenameActivityFrom = assembled
	original := r.URL.Path
	r.URL.Path = prefix + assembled
	sw := &statusResponseWriter{ResponseWriter: w, status: http.StatusOK}
	wd.ServeHTTP(sw, r)
	r.URL.Path = original
	if sw.status >= 300 {
		os.Remove(realAssembled)
		return true, 0, nil
	}
	activity.RecordWebDAVUser(r, ffs.user, activitydb.Entry{
		EventType: activitydb.EventUpload,
		Source:    ffs.source,
		Path:      destPath,
		Details: activitydb.Details{
			Source: ffs.source,
			Path:   destPath,
		},
	})
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
		// only regular files are chunks: a symlink or device named `x-chunk-0` must never
		// be read into the assembled file (Type()==0 means "unknown", openOwncloudChunks
		// re-checks those against Lstat)
		if t := entry.Type(); t != 0 && !t.IsRegular() {
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
		in, err := openOwncloudChunk(filepath.Join(realDir, chunk.name))
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
	if err := out.Sync(); err != nil {
		out.Close()
		return written, err
	}
	return written, out.Close()
}

// openOwncloudChunk opens a chunk for reading without ever following a symlink: the entry is
// Lstat'ed first (a symlink swap between ReadDir and open is caught by the SameFile check),
// and on platforms that support it the open itself uses O_NOFOLLOW.
func openOwncloudChunk(name string) (*os.File, error) {
	lstat, err := os.Lstat(name)
	if err != nil {
		return nil, err
	}
	if !lstat.Mode().IsRegular() {
		return nil, fmt.Errorf("chunk %s is not a regular file", filepath.Base(name))
	}
	in, err := os.OpenFile(name, os.O_RDONLY|owncloudNoFollowFlag, 0)
	if err != nil {
		return nil, err
	}
	stat, err := in.Stat()
	if err == nil && (!stat.Mode().IsRegular() || !os.SameFile(lstat, stat)) {
		err = fmt.Errorf("chunk %s changed while opening", filepath.Base(name))
	}
	if err != nil {
		in.Close()
		return nil, err
	}
	return in, nil
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
		return "", fmt.Errorf("destination is outside this source")
	}
	rest = path.Clean("/" + rest)
	if rest == "/" || rest == "." {
		return "", fmt.Errorf("destination is empty")
	}
	return rest, nil
}
