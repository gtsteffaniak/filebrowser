package web

import (
	"fmt"
	"io"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/gtsteffaniak/filebrowser/backend/internal/database/users"
)

// chunkedUploadUser builds a user with the given permissions on source1.
func chunkedUploadUser(source1Path, scope string, perms users.SourceFilePermissions) *users.User {
	return &users.User{
		ID: 1,
		FrontendUser: users.FrontendUser{
			Username: "chunked-uploader",
		},
		BackendScopes: []users.BackendScope{
			{Path: source1Path, Scope: scope},
		},
		BackendSourcePermissions: map[string]users.SourceFilePermissions{source1Path: perms},
		Version:                  users.SourcePermissionsMigrationVersion,
	}
}

func chunkedUploadFullPerms() users.SourceFilePermissions {
	return users.SourceFilePermissions{View: true, Download: true, Modify: true, Create: true, Delete: true}
}

// doSource1WebDAV calls the handler the way the router does, and reports the status the client
// would see (the handler writes success codes itself and returns error codes).
func doSource1WebDAV(t *testing.T, user *users.User, method, path string, body io.Reader, headers map[string]string) int {
	t.Helper()
	req := httptest.NewRequest(method, "/dav/source1"+path, body)
	req.SetPathValue("source", "source1")
	req.SetPathValue("path", path)
	for k, v := range headers {
		req.Header.Set(k, v)
	}
	w := httptest.NewRecorder()
	status, err := webDAVHandler(w, req, &requestContext{User: user})
	if status != 0 && status != http.StatusOK && (w.Code == 0 || w.Code == http.StatusOK) {
		w.Code = status
	}
	if status >= 400 && err == nil {
		t.Fatalf("%s %s: status %d without an error", method, path, status)
	}
	return w.Code
}

func mustPutChunk(t *testing.T, user *users.User, dir, name string, number int, content string) {
	t.Helper()
	path := fmt.Sprintf("%s/%s-chunk-%d", dir, name, number)
	if got := doSource1WebDAV(t, user, http.MethodPut, path, strings.NewReader(content), nil); got != http.StatusCreated {
		t.Fatalf("PUT %s: got %d, want 201", path, got)
	}
}

func TestWebDAV_OwncloudChunkedUpload_AssemblesChunks(t *testing.T) {
	source1Path, _ := setupWebDAVTestEnv(t)
	initTestIndex(t, "source1", source1Path)
	user := chunkedUploadUser(source1Path, "/", chunkedUploadFullPerms())

	// rclone's shape: the chunks are ordinary uploads next to the destination
	parts := []string{"hello ", "big ", "world"}
	for i, part := range parts {
		mustPutChunk(t, user, "/public", "video.mp4", i, part)
	}

	got := doSource1WebDAV(t, user, "MOVE", "/public/video.mp4-chunk-2", nil, map[string]string{
		"Destination":     "/dav/source1/public/video.mp4",
		"OC-Total-Length": "15",
	})
	if got != http.StatusCreated {
		t.Fatalf("MOVE of the last chunk: got %d, want 201", got)
	}

	content, err := os.ReadFile(filepath.Join(source1Path, "public", "video.mp4"))
	if err != nil {
		t.Fatalf("assembled file not written: %v", err)
	}
	if string(content) != "hello big world" {
		t.Errorf("assembled content = %q, want %q", content, "hello big world")
	}
	entries, err := os.ReadDir(filepath.Join(source1Path, "public"))
	if err != nil {
		t.Fatal(err)
	}
	for _, entry := range entries {
		if strings.Contains(entry.Name(), "-chunk-") || strings.HasSuffix(entry.Name(), ".assembled") {
			t.Errorf("transfer artifact left behind: %s", entry.Name())
		}
	}
}

func TestWebDAV_OwncloudChunkedUpload_RefusesGapsAndWrongTotal(t *testing.T) {
	source1Path, _ := setupWebDAVTestEnv(t)
	initTestIndex(t, "source1", source1Path)
	user := chunkedUploadUser(source1Path, "/", chunkedUploadFullPerms())

	// a missing chunk must not be assembled into a shorter file
	mustPutChunk(t, user, "/public", "gap.bin", 0, "abc")
	mustPutChunk(t, user, "/public", "gap.bin", 2, "def")
	got := doSource1WebDAV(t, user, "MOVE", "/public/gap.bin-chunk-2", nil, map[string]string{
		"Destination":     "/dav/source1/public/gap.bin",
		"OC-Total-Length": "6",
	})
	if got != http.StatusConflict {
		t.Errorf("MOVE with a missing chunk: got %d, want 409", got)
	}
	if _, err := os.Stat(filepath.Join(source1Path, "public", "gap.bin")); !os.IsNotExist(err) {
		t.Errorf("a truncated file was written anyway: %v", err)
	}

	// ... and neither must a total length that disagrees with the chunks
	mustPutChunk(t, user, "/public", "short.bin", 0, "abc")
	mustPutChunk(t, user, "/public", "short.bin", 1, "def")
	got = doSource1WebDAV(t, user, "MOVE", "/public/short.bin-chunk-1", nil, map[string]string{
		"Destination":     "/dav/source1/public/short.bin",
		"OC-Total-Length": "99",
	})
	if got != http.StatusBadRequest {
		t.Errorf("MOVE with a wrong total length: got %d, want 400", got)
	}
}

func TestWebDAV_OwncloudChunkedUpload_HonoursOverwrite(t *testing.T) {
	source1Path, _ := setupWebDAVTestEnv(t)
	initTestIndex(t, "source1", source1Path)
	user := chunkedUploadUser(source1Path, "/", chunkedUploadFullPerms())
	existing := filepath.Join(source1Path, "public", "existing.bin")
	if err := os.WriteFile(existing, []byte("do not lose me"), 0644); err != nil {
		t.Fatal(err)
	}

	mustPutChunk(t, user, "/public", "existing.bin", 0, "new ")
	mustPutChunk(t, user, "/public", "existing.bin", 1, "content")
	// without `Overwrite: T` the destination must not be replaced (the WebDAV library's rule)
	got := doSource1WebDAV(t, user, "MOVE", "/public/existing.bin-chunk-1", nil, map[string]string{
		"Destination": "/dav/source1/public/existing.bin",
	})
	if got != http.StatusPreconditionFailed {
		t.Errorf("MOVE onto an existing file without Overwrite: got %d, want 412", got)
	}
	if content, _ := os.ReadFile(existing); string(content) != "do not lose me" {
		t.Errorf("existing file was replaced anyway: %q", content)
	}

	got = doSource1WebDAV(t, user, "MOVE", "/public/existing.bin-chunk-1", nil, map[string]string{
		"Destination": "/dav/source1/public/existing.bin",
		"Overwrite":   "T",
	})
	if got != http.StatusNoContent {
		t.Errorf("MOVE with Overwrite: T: got %d, want 204", got)
	}
	if content, _ := os.ReadFile(existing); string(content) != "new content" {
		t.Errorf("overwrite did not land: %q", content)
	}
}

func TestWebDAV_OwncloudChunkedUpload_CannotWriteOutsideTheScope(t *testing.T) {
	source1Path, _ := setupWebDAVTestEnv(t)
	initTestIndex(t, "source1", source1Path)
	// the user is confined to /public
	user := chunkedUploadUser(source1Path, "/public", chunkedUploadFullPerms())

	// both the request path and the Destination climb out with `..`: this is the shape that enters the
	// chunked move itself (they have to agree, or it is an ordinary move)
	mustPutChunk(t, user, "/public/..", "escape.bin", 0, "ESCAPED")
	mustPutChunk(t, user, "/public/..", "escape.bin", 1, "!")
	got := doSource1WebDAV(t, user, "MOVE", "/public/../escape.bin-chunk-1", nil, map[string]string{
		"Destination": "/dav/source1/public/../escape.bin",
	})
	if got != http.StatusCreated {
		t.Fatalf("chunked move with a climbing path: got %d, want 201", got)
	}
	// the cleaned paths are anchored at the scope, so the file lands inside it and not above it,
	// and it is the concatenation of both chunks (which only the chunked move produces)
	content, err := os.ReadFile(filepath.Join(source1Path, "public", "escape.bin"))
	if err != nil {
		t.Errorf("assembled file is not inside the user's scope: %v", err)
	} else if string(content) != "ESCAPED!" {
		t.Errorf("assembled content = %q, want %q", content, "ESCAPED!")
	}
	if _, err := os.Stat(filepath.Join(source1Path, "public", "escape.bin-chunk-0")); !os.IsNotExist(err) {
		t.Errorf("a chunk was left behind: %v", err)
	}
	if _, err := os.Stat(filepath.Join(source1Path, "escape.bin")); !os.IsNotExist(err) {
		t.Errorf("the climbing path escaped the scope: %v", err)
	}

	// a Destination that resolves somewhere else entirely is not a chunked upload, and the library
	// refuses to move the chunk there
	mustPutChunk(t, user, "", "other.bin", 0, "a")
	mustPutChunk(t, user, "", "other.bin", 1, "b")
	got = doSource1WebDAV(t, user, "MOVE", "/other.bin-chunk-1", nil, map[string]string{
		"Destination": "/dav/source1/public/../../" + strings.Repeat("../", 20) + "tmp/fb_escape_probe.txt",
	})
	if got < 300 {
		t.Errorf("MOVE to a destination outside the source: got %d, want an error", got)
	}
	if _, err := os.Stat("/tmp/fb_escape_probe.txt"); !os.IsNotExist(err) {
		t.Fatalf("path traversal escaped the scope (status %d): %v", got, err)
	}

	// and no half-assembled file is left behind anywhere the user can see
	entries, err := os.ReadDir(filepath.Join(source1Path, "public"))
	if err != nil {
		t.Fatal(err)
	}
	for _, entry := range entries {
		if strings.Contains(entry.Name(), ".assembled") {
			t.Errorf("assembled temp file left behind: %s", entry.Name())
		}
	}
}

func TestWebDAV_OwncloudChunkedUpload_PlainMoveStillWorks(t *testing.T) {
	source1Path, _ := setupWebDAVTestEnv(t)
	initTestIndex(t, "source1", source1Path)
	user := chunkedUploadUser(source1Path, "/", chunkedUploadFullPerms())

	// a real file whose name ends in -chunk-N, with no siblings, is an ordinary file
	doSource1WebDAV(t, user, http.MethodPut, "/public/notes-chunk-1", strings.NewReader("just notes"), nil)
	got := doSource1WebDAV(t, user, "MOVE", "/public/notes-chunk-1", nil, map[string]string{
		"Destination": "/dav/source1/public/notes.txt",
	})
	if got != http.StatusCreated {
		t.Fatalf("MOVE of a plain file: got %d, want 201", got)
	}
	content, err := os.ReadFile(filepath.Join(source1Path, "public", "notes.txt"))
	if err != nil || string(content) != "just notes" {
		t.Errorf("plain move lost the file: %v %q", err, content)
	}
}

func TestWebDAV_OwncloudChunkedUpload_DoesNotDestroyCollidingTemp(t *testing.T) {
	source1Path, _ := setupWebDAVTestEnv(t)
	initTestIndex(t, "source1", source1Path)
	user := chunkedUploadUser(source1Path, "/", chunkedUploadFullPerms())

	// a user file that looks like our staging name must survive the upload
	colliding := filepath.Join(source1Path, "public", ".video.mp4.assembled")
	if err := os.WriteFile(colliding, []byte("user data"), 0644); err != nil {
		t.Fatal(err)
	}
	mustPutChunk(t, user, "/public", "video.mp4", 0, "hello ")
	mustPutChunk(t, user, "/public", "video.mp4", 1, "world")
	got := doSource1WebDAV(t, user, "MOVE", "/public/video.mp4-chunk-1", nil, map[string]string{
		"Destination": "/dav/source1/public/video.mp4",
	})
	if got != http.StatusCreated {
		t.Fatalf("MOVE: got %d, want 201", got)
	}
	if content, err := os.ReadFile(colliding); err != nil || string(content) != "user data" {
		t.Errorf("the colliding file was destroyed: %v %q", err, content)
	}
	if content, err := os.ReadFile(filepath.Join(source1Path, "public", "video.mp4")); err != nil || string(content) != "hello world" {
		t.Errorf("assembled content = %q, %v", content, err)
	}
}

func TestWebDAV_OwncloudChunkedUploadPathParsing(t *testing.T) {
	cases := []struct {
		path          string
		base          string
		ok            bool
		chunkNumber   string
		validChunkNum bool
	}{
		{"/public/video.mp4-chunk-0", "/public/video.mp4", true, "0", true},
		{"/video.mp4-chunk-12", "/video.mp4", true, "12", true},
		{"/video.mp4-chunk-", "", false, "", false},
		{"/chunk-1", "", false, "1", true},
		{"/public/readme.txt", "", false, "", false},
		{"/public/video.mp4-chunk-00x", "", false, "00x", false},
		{"/public/video.mp4-chunk-1234567890", "", false, "1234567890", false},
	}
	for _, tc := range cases {
		base, ok := owncloudChunkParts(tc.path)
		if base != tc.base || ok != tc.ok {
			t.Errorf("owncloudChunkParts(%q) = %q, %v; want %q, %v", tc.path, base, ok, tc.base, tc.ok)
		}
		if got := isUploadChunkNumber(tc.chunkNumber); got != tc.validChunkNum {
			t.Errorf("isUploadChunkNumber(%q) = %v, want %v", tc.chunkNumber, got, tc.validChunkNum)
		}
	}
}

func TestWebDAV_UploadDestinationPathIsCleaned(t *testing.T) {
	cases := []struct {
		destination string
		want        string
	}{
		{"/dav/source1/public/file.txt", "/public/file.txt"},
		{"/dav/source1/public/../../tmp/x.txt", "/tmp/x.txt"},
		{"/dav/source1/public/" + strings.Repeat("../", 25) + "tmp/x.txt", "/tmp/x.txt"},
		{"/dav/source1/", ""},
		{"/dav/source1", ""},
	}
	for _, tc := range cases {
		req := httptest.NewRequest("MOVE", "/dav/source1/public/x-chunk-0", nil)
		req.Header.Set("Destination", tc.destination)
		got, err := uploadDestinationPath(req, "/dav/source1")
		if tc.want == "" {
			if err == nil {
				t.Errorf("uploadDestinationPath(%q) = %q, want an error", tc.destination, got)
			}
			continue
		}
		if err != nil || got != tc.want {
			t.Errorf("uploadDestinationPath(%q) = %q, %v; want %q", tc.destination, got, err, tc.want)
		}
	}
}
