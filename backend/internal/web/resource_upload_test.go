package web

import (
	"bytes"
	"errors"
	"fmt"
	"io"
	"math"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strconv"
	"sync"
	"testing"

	"github.com/gtsteffaniak/filebrowser/backend/internal/adapters/fs/fileutils"
	"github.com/gtsteffaniak/filebrowser/backend/internal/database/users"
	"github.com/gtsteffaniak/filebrowser/backend/pkg/indexing"
	"github.com/gtsteffaniak/filebrowser/backend/pkg/settings"
)

var uploadTestResolverMu sync.Mutex

func setupUploadHTTPTest(t *testing.T) (root string, user *users.User) {
	t.Helper()
	uploadTestResolverMu.Lock()

	prevFilePerm := fileutils.PermFile
	prevDirPerm := fileutils.PermDir
	prevSourceMap := settings.Config.Server.SourceMap
	prevNameToSource := settings.Config.Server.NameToSource
	prevSourceResolver := users.GetSourceNameResolver()
	prevSourceConfig := users.GetSourceConfig()

	t.Cleanup(func() {
		indexing.ClearTestIndices()
		fileutils.SetFsPermissions(uint32(prevFilePerm), uint32(prevDirPerm))
		settings.Config.Server.SourceMap = prevSourceMap
		settings.Config.Server.NameToSource = prevNameToSource
		users.SetSourceNameResolver(prevSourceResolver)
		users.SetSourceConfig(prevSourceConfig)
		uploadTestResolverMu.Unlock()
	})

	fileutils.SetFsPermissions(0o644, 0o755)

	root = t.TempDir()
	sourceName := "uploads"
	sourcePath := root

	indexing.SetTestIndex(sourceName, sourcePath)
	idx := indexing.GetIndex(sourceName)
	if idx == nil {
		t.Fatal("expected test index")
	}
	idx.Config.ResolvedRules.IndexingDisabled = true

	settings.Config.Server.SourceMap = map[string]*settings.Source{
		sourcePath: {Path: sourcePath, Name: sourceName},
	}
	settings.Config.Server.NameToSource = map[string]*settings.Source{
		sourceName: settings.Config.Server.SourceMap[sourcePath],
	}

	users.SetSourceNameResolver(func(name string) (string, error) {
		if name == sourceName {
			return sourcePath, nil
		}
		return "", fmt.Errorf("unknown source %q", name)
	})
	users.SetSourceConfig(&users.SourceConfigProvider{
		GetSourceByPath: func(path string) (users.SourceInfo, bool) {
			if path == sourcePath {
				return users.SourceInfo{Path: sourcePath, Name: sourceName}, true
			}
			return users.SourceInfo{}, false
		},
		GetSourceByName: func(name string) (users.SourceInfo, bool) {
			if name == sourceName {
				return users.SourceInfo{Path: sourcePath, Name: sourceName}, true
			}
			return users.SourceInfo{}, false
		},
	})

	perms := users.SourceFilePermissions{
		View: true, Download: true, Modify: true, Create: true, Delete: true,
	}
	user = &users.User{
		ID: 1,
		FrontendUser: users.FrontendUser{
			Username: "uploader",
		},
		BackendScopes: []users.BackendScope{
			{Path: sourcePath, Scope: "/", Permissions: perms},
		},
		BackendSourcePermissions: map[string]users.SourceFilePermissions{
			sourcePath: perms,
		},
		Version: users.SourcePermissionsMigrationVersion,
	}
	return root, user
}

func postUpload(
	t *testing.T,
	user *users.User,
	path string,
	body io.Reader,
	contentLength int64,
	headers map[string]string,
) (int, error) {
	t.Helper()
	url := "/api/resources?source=uploads&path=" + path
	req := httptest.NewRequest(http.MethodPost, url, body)
	if contentLength >= 0 {
		req.ContentLength = contentLength
	}
	if headers == nil {
		headers = map[string]string{}
	}
	if _, ok := headers["X-File-Upload-Session"]; !ok {
		headers["X-File-Upload-Session"] = "test-session"
	}
	for k, v := range headers {
		req.Header.Set(k, v)
	}
	rec := httptest.NewRecorder()
	return ResourcePostHandler(rec, req, &requestContext{User: user})
}

func TestParseUploadTotalSize(t *testing.T) {
	t.Parallel()

	req := httptest.NewRequest(http.MethodPost, "/", nil)
	total, ok, err := parseUploadTotalSize(req)
	if err != nil || ok || total != 0 {
		t.Fatalf("empty header: total=%d ok=%v err=%v", total, ok, err)
	}

	req.Header.Set("X-File-Total-Size", "12345")
	total, ok, err = parseUploadTotalSize(req)
	if err != nil || !ok || total != 12345 {
		t.Fatalf("valid header: total=%d ok=%v err=%v", total, ok, err)
	}

	req.Header.Set("X-File-Total-Size", "nope")
	_, _, err = parseUploadTotalSize(req)
	if err == nil {
		t.Fatal("expected parse error")
	}
}

func TestValidateReceivedBytes(t *testing.T) {
	t.Parallel()

	if err := validateReceivedBytes(10, 10, true, 10); err != nil {
		t.Fatalf("matching sizes: %v", err)
	}
	if err := validateReceivedBytes(9, 10, true, -1); err == nil {
		t.Fatal("expected total size mismatch")
	}
	if err := validateReceivedBytes(9, 0, false, 10); err == nil {
		t.Fatal("expected content-length mismatch")
	}
	if err := validateReceivedBytes(10, 0, false, -1); err != nil {
		t.Fatalf("no expected sizes: %v", err)
	}
}

func TestValidateChunkBounds(t *testing.T) {
	t.Parallel()

	if err := validateChunkBounds(0, 10, 100); err != nil {
		t.Fatalf("valid bounds: %v", err)
	}
	if err := validateChunkBounds(-1, 10, 100); err == nil {
		t.Fatal("expected negative offset error")
	}
	if err := validateChunkBounds(101, 1, 100); err == nil {
		t.Fatal("expected offset exceeds total error")
	}
	if err := validateChunkBounds(90, 20, 100); err == nil {
		t.Fatal("expected chunk exceeds remaining error")
	}
	if err := validateChunkBounds(math.MaxInt64, 1, math.MaxInt64); err == nil {
		t.Fatal("expected offset overflow error")
	}
	if err := validateAssembledSize(90, 20, 100); err == nil {
		t.Fatal("expected assembled size error")
	}
}

func TestUploadTempPathStable(t *testing.T) {
	t.Parallel()
	a := uploadTempPath("/data/photo.jpg", "session-a")
	b := uploadTempPath("/data/photo.jpg", "session-a")
	if a != b {
		t.Fatalf("expected stable temp path, got %q and %q", a, b)
	}
	if a == uploadTempPath("/data/photo.jpg", "session-b") {
		t.Fatal("expected different sessions to use different temp paths")
	}
	if filepath.Ext(a) != ".tmp" {
		t.Fatalf("expected .tmp suffix, got %q", a)
	}
}

func TestResourcePostHandler_NonChunkedMatchesTotalSize(t *testing.T) {
	root, user := setupUploadHTTPTest(t)
	body := []byte("hello-image-bytes")
	status, err := postUpload(t, user, "/photo.jpg", bytes.NewReader(body), int64(len(body)), map[string]string{
		"X-File-Total-Size": strconv.Itoa(len(body)),
	})
	if status != http.StatusOK || err != nil {
		t.Fatalf("status=%d err=%v", status, err)
	}
	got, err := os.ReadFile(filepath.Join(root, "photo.jpg"))
	if err != nil {
		t.Fatal(err)
	}
	if !bytes.Equal(got, body) {
		t.Fatalf("file contents mismatch: got %q", got)
	}
	if matches, _ := filepath.Glob(filepath.Join(root, "*.uploading.tmp")); len(matches) != 0 {
		t.Fatalf("expected no leftover temp files, got %v", matches)
	}
}

func TestResourcePostHandler_NonChunkedTruncatedBody(t *testing.T) {
	root, user := setupUploadHTTPTest(t)
	body := []byte("partial")
	status, err := postUpload(t, user, "/photo.jpg", bytes.NewReader(body), int64(len(body)), map[string]string{
		"X-File-Total-Size": "100",
	})
	if status != http.StatusBadRequest || err == nil {
		t.Fatalf("status=%d err=%v, want 400", status, err)
	}
	if _, err := os.Stat(filepath.Join(root, "photo.jpg")); !os.IsNotExist(err) {
		t.Fatalf("expected no final file, stat err=%v", err)
	}
	if matches, _ := filepath.Glob(filepath.Join(root, "*.uploading.tmp")); len(matches) != 0 {
		t.Fatalf("expected temp removed, got %v", matches)
	}
}

func TestResourcePostHandler_ChunkedTwoChunksComplete(t *testing.T) {
	root, user := setupUploadHTTPTest(t)
	part1 := []byte("aaaa")
	part2 := []byte("bbbb")
	total := len(part1) + len(part2)

	status, err := postUpload(t, user, "/big.bin", bytes.NewReader(part1), int64(len(part1)), map[string]string{
		"X-File-Chunk-Offset": "0",
		"X-File-Total-Size":   strconv.Itoa(total),
	})
	if status != http.StatusOK || err != nil {
		t.Fatalf("chunk1 status=%d err=%v", status, err)
	}
	if _, err = os.Stat(filepath.Join(root, "big.bin")); !os.IsNotExist(err) {
		t.Fatal("final file should not exist after first chunk")
	}

	status, err = postUpload(t, user, "/big.bin", bytes.NewReader(part2), int64(len(part2)), map[string]string{
		"X-File-Chunk-Offset": strconv.Itoa(len(part1)),
		"X-File-Total-Size":   strconv.Itoa(total),
	})
	if status != http.StatusOK || err != nil {
		t.Fatalf("chunk2 status=%d err=%v", status, err)
	}
	got, err := os.ReadFile(filepath.Join(root, "big.bin"))
	if err != nil {
		t.Fatal(err)
	}
	want := append(append([]byte{}, part1...), part2...)
	if !bytes.Equal(got, want) {
		t.Fatalf("assembled mismatch: got %q want %q", got, want)
	}
}

func TestResourcePostHandler_ChunkedShortChunkContentLength(t *testing.T) {
	root, user := setupUploadHTTPTest(t)
	part1 := []byte("12345678")
	total := 16

	status, err := postUpload(t, user, "/short.bin", bytes.NewReader(part1), int64(len(part1)), map[string]string{
		"X-File-Chunk-Offset": "0",
		"X-File-Total-Size":   strconv.Itoa(total),
	})
	if status != http.StatusOK || err != nil {
		t.Fatalf("chunk1 status=%d err=%v", status, err)
	}

	short := []byte("xx")
	status, err = postUpload(t, user, "/short.bin", bytes.NewReader(short), 8, map[string]string{
		"X-File-Chunk-Offset": strconv.Itoa(len(part1)),
		"X-File-Total-Size":   strconv.Itoa(total),
	})
	if status != http.StatusBadRequest || err == nil {
		t.Fatalf("status=%d err=%v, want 400", status, err)
	}

	realPath := filepath.Join(root, "short.bin")
	tempPath := uploadTempPath(realPath, "test-session")
	info, err := os.Stat(tempPath)
	if err != nil {
		t.Fatalf("expected temp preserved: %v", err)
	}
	if info.Size() != int64(len(part1)) {
		t.Fatalf("temp size=%d, want %d", info.Size(), len(part1))
	}
	if _, err := os.Stat(realPath); !os.IsNotExist(err) {
		t.Fatal("final file should not exist after short chunk")
	}
}

type errAfterReader struct {
	data []byte
	err  error
	n    int
}

func (r *errAfterReader) Read(p []byte) (int, error) {
	if r.n >= len(r.data) {
		return 0, r.err
	}
	n := copy(p, r.data[r.n:])
	r.n += n
	if r.n >= len(r.data) {
		return n, r.err
	}
	return n, nil
}

func TestResourcePostHandler_ChunkedKeepsPartialOnBodyError(t *testing.T) {
	root, user := setupUploadHTTPTest(t)
	part1 := []byte("aaaaaaaa")
	total := 16

	status, err := postUpload(t, user, "/resume.bin", bytes.NewReader(part1), int64(len(part1)), map[string]string{
		"X-File-Chunk-Offset": "0",
		"X-File-Total-Size":   strconv.Itoa(total),
	})
	if status != http.StatusOK || err != nil {
		t.Fatalf("chunk1 status=%d err=%v", status, err)
	}

	failing := &errAfterReader{
		data: []byte("bbb"),
		err:  errors.New("simulated disconnect"),
	}
	status, err = postUpload(t, user, "/resume.bin", failing, 8, map[string]string{
		"X-File-Chunk-Offset": strconv.Itoa(len(part1)),
		"X-File-Total-Size":   strconv.Itoa(total),
	})
	if status != http.StatusInternalServerError || err == nil {
		t.Fatalf("status=%d err=%v, want 500", status, err)
	}

	realPath := filepath.Join(root, "resume.bin")
	tempPath := uploadTempPath(realPath, "test-session")
	info, err := os.Stat(tempPath)
	if err != nil {
		t.Fatalf("expected temp preserved: %v", err)
	}
	if info.Size() != int64(len(part1)) {
		t.Fatalf("temp size=%d, want truncated to %d", info.Size(), len(part1))
	}

	// Resume with a full second chunk.
	part2 := []byte("bbbbbbbb")
	status, err = postUpload(t, user, "/resume.bin", bytes.NewReader(part2), int64(len(part2)), map[string]string{
		"X-File-Chunk-Offset": strconv.Itoa(len(part1)),
		"X-File-Total-Size":   strconv.Itoa(total),
	})
	if status != http.StatusOK || err != nil {
		t.Fatalf("resume status=%d err=%v", status, err)
	}
	got, err := os.ReadFile(realPath)
	if err != nil {
		t.Fatal(err)
	}
	want := append(append([]byte{}, part1...), part2...)
	if !bytes.Equal(got, want) {
		t.Fatalf("resumed contents mismatch: got %q want %q", got, want)
	}
}

func TestResourcePostHandler_ChunkOffsetExceedsTotal(t *testing.T) {
	_, user := setupUploadHTTPTest(t)
	body := []byte("data")
	status, err := postUpload(t, user, "/big.bin", bytes.NewReader(body), int64(len(body)), map[string]string{
		"X-File-Chunk-Offset": "200",
		"X-File-Total-Size":   "100",
	})
	if status != http.StatusBadRequest || err == nil {
		t.Fatalf("status=%d err=%v, want 400", status, err)
	}
}

func TestResourcePostHandler_ConflictingUploadSessions(t *testing.T) {
	root, user := setupUploadHTTPTest(t)
	part1 := []byte("aaaa")
	total := 8

	status, err := postUpload(t, user, "/race.bin", bytes.NewReader(part1), int64(len(part1)), map[string]string{
		"X-File-Upload-Session": "session-a",
		"X-File-Chunk-Offset":   "0",
		"X-File-Total-Size":     strconv.Itoa(total),
	})
	if status != http.StatusOK || err != nil {
		t.Fatalf("chunk1 status=%d err=%v", status, err)
	}

	status, err = postUpload(t, user, "/race.bin", bytes.NewReader(part1), int64(len(part1)), map[string]string{
		"X-File-Upload-Session": "session-b",
		"X-File-Chunk-Offset":   "0",
		"X-File-Total-Size":     strconv.Itoa(total),
	})
	if status != http.StatusConflict || err == nil {
		t.Fatalf("status=%d err=%v, want 409", status, err)
	}

	// Non-chunked upload should also conflict with an active chunked session.
	status, err = postUpload(t, user, "/race.bin", bytes.NewReader([]byte("tiny")), 4, map[string]string{
		"X-File-Upload-Session": "session-c",
		"X-File-Total-Size":     "4",
	})
	if status != http.StatusConflict || err == nil {
		t.Fatalf("non-chunked status=%d err=%v, want 409", status, err)
	}

	// Original session can still resume.
	part2 := []byte("bbbb")
	status, err = postUpload(t, user, "/race.bin", bytes.NewReader(part2), int64(len(part2)), map[string]string{
		"X-File-Upload-Session": "session-a",
		"X-File-Chunk-Offset":   strconv.Itoa(len(part1)),
		"X-File-Total-Size":     strconv.Itoa(total),
	})
	if status != http.StatusOK || err != nil {
		t.Fatalf("resume status=%d err=%v", status, err)
	}
	got, err := os.ReadFile(filepath.Join(root, "race.bin"))
	if err != nil {
		t.Fatal(err)
	}
	want := append(append([]byte{}, part1...), part2...)
	if !bytes.Equal(got, want) {
		t.Fatalf("assembled mismatch: got %q want %q", got, want)
	}
}
