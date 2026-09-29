package web

import (
	"bytes"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"testing"
)

func writeSeedFile(t *testing.T, path string, size int) {
	t.Helper()
	if err := os.WriteFile(path, make([]byte, size), 0o644); err != nil {
		t.Fatalf("writing seed file: %v", err)
	}
}

func TestDirTreeSize(t *testing.T) {
	root := t.TempDir()
	if err := os.MkdirAll(filepath.Join(root, "a", "b"), 0o755); err != nil {
		t.Fatal(err)
	}
	writeSeedFile(t, filepath.Join(root, "f1"), 100)
	writeSeedFile(t, filepath.Join(root, "a", "f2"), 250)
	writeSeedFile(t, filepath.Join(root, "a", "b", "f3"), 75)

	total, err := dirTreeSize(root)
	if err != nil {
		t.Fatalf("dirTreeSize: %v", err)
	}
	if total != 425 {
		t.Fatalf("expected 425 bytes, got %d", total)
	}

	// A missing root reports the not-exist error so callers can treat it as zero usage.
	_, err = dirTreeSize(filepath.Join(root, "does-not-exist"))
	if !os.IsNotExist(err) {
		t.Fatalf("expected IsNotExist error, got %v", err)
	}
}

func TestCheckSourceQuota(t *testing.T) {
	root, user := setupUploadHTTPTest(t)

	// A zero quota means unlimited.
	if status, err := checkSourceQuota(user, "uploads", 1<<40, 0); err != nil {
		t.Fatalf("unlimited quota should pass, got status=%d err=%v", status, err)
	}

	user.BackendScopes[0].MaxStorageBytes = 1024
	if status, err := checkSourceQuota(user, "uploads", 500, 0); err != nil {
		t.Fatalf("write under quota should pass: status=%d err=%v", status, err)
	}
	// Exactly at the limit is allowed; one byte over is rejected.
	if status, err := checkSourceQuota(user, "uploads", 1024, 0); err != nil {
		t.Fatalf("write at the limit should pass: status=%d err=%v", status, err)
	}
	if status, err := checkSourceQuota(user, "uploads", 1025, 0); err == nil {
		t.Fatal("expected quota rejection one byte over the limit")
	} else if status != http.StatusRequestEntityTooLarge {
		t.Fatalf("expected 413, got %d", status)
	}
	// Overwriting a 512-byte file frees space, so a 512-byte write fits exactly.
	if err := os.MkdirAll(filepath.Join(root, "sub"), 0o755); err != nil {
		t.Fatal(err)
	}
	writeSeedFile(t, filepath.Join(root, "sub", "existing"), 512)
	if status, err := checkSourceQuota(user, "uploads", 512, 512); err != nil {
		t.Fatalf("replace at the limit should pass: status=%d err=%v", status, err)
	}
}

func TestResourcePostQuota(t *testing.T) {
	root, user := setupUploadHTTPTest(t)
	user.BackendScopes[0].MaxStorageBytes = 512
	writeSeedFile(t, filepath.Join(root, "seed.bin"), 400)

	post := func(path string, size int) int {
		t.Helper()
		body := bytes.Repeat([]byte("x"), size)
		req := httptest.NewRequest(http.MethodPost, "/api/resources?source=uploads&path="+path, bytes.NewReader(body))
		req.ContentLength = int64(len(body))
		req.Header.Set("X-File-Upload-Session", "quota-test")
		req.Header.Set("X-File-Total-Size", strconv.Itoa(size))
		rec := httptest.NewRecorder()
		status, _ := ResourcePostHandler(rec, req, &requestContext{User: user})
		return status
	}

	// 400 existing + 200 new = 600 > 512 limit.
	if status := post("over.bin", 200); status != http.StatusRequestEntityTooLarge {
		t.Fatalf("expected 413 for over-quota upload, got %d", status)
	}
	if _, err := os.Stat(filepath.Join(root, "over.bin")); !os.IsNotExist(err) {
		t.Fatal("file must not be created when the quota is exceeded")
	}

	// 400 existing + 100 new = 500 <= 512 limit.
	if status := post("ok.bin", 100); status != http.StatusOK {
		t.Fatalf("expected 200 for under-quota upload, got %d", status)
	}

	// Chunked upload declares 500 bytes up front: 500 existing + 500 > 512.
	chunkReq := httptest.NewRequest(http.MethodPost, "/api/resources?source=uploads&path=chunk.bin", bytes.NewReader(bytes.Repeat([]byte("c"), 50)))
	chunkReq.ContentLength = 50
	chunkReq.Header.Set("X-File-Chunk-Offset", "0")
	chunkReq.Header.Set("X-File-Total-Size", "500")
	chunkReq.Header.Set("X-File-Upload-Session", "quota-chunk")
	chunkRec := httptest.NewRecorder()
	if status, _ := ResourcePostHandler(chunkRec, chunkReq, &requestContext{User: user}); status != http.StatusRequestEntityTooLarge {
		t.Fatalf("expected 413 for over-quota chunked upload, got %d", status)
	}
	if _, err := os.Stat(filepath.Join(root, "chunk.bin")); !os.IsNotExist(err) {
		t.Fatal("chunked upload must not finalize when the quota is exceeded")
	}
}

func TestResourcePutQuota(t *testing.T) {
	root, user := setupUploadHTTPTest(t)
	user.BackendScopes[0].MaxStorageBytes = 512
	writeSeedFile(t, filepath.Join(root, "edit.bin"), 300)

	// Overwriting the 300-byte file with 300 bytes keeps usage at 300.
	body := bytes.Repeat([]byte("z"), 300)
	req := httptest.NewRequest(http.MethodPut, "/api/resources?source=uploads&path=edit.bin", bytes.NewReader(body))
	req.ContentLength = int64(len(body))
	rec := httptest.NewRecorder()
	if status, _ := resourcePutHandler(rec, req, &requestContext{User: user}); status != http.StatusOK {
		t.Fatalf("expected 200 for in-place overwrite, got %d", status)
	}

	// Creating a new 300-byte file: 300 existing + 300 = 600 > 512.
	req = httptest.NewRequest(http.MethodPut, "/api/resources?source=uploads&path=new.bin", bytes.NewReader(body))
	req.ContentLength = int64(len(body))
	rec = httptest.NewRecorder()
	if status, _ := resourcePutHandler(rec, req, &requestContext{User: user}); status != http.StatusRequestEntityTooLarge {
		t.Fatalf("expected 413 for over-quota PUT, got %d", status)
	}
	if _, err := os.Stat(filepath.Join(root, "new.bin")); !os.IsNotExist(err) {
		t.Fatal("file must not be created when the quota is exceeded")
	}
}

func TestResourcePatchCopyQuota(t *testing.T) {
	root, user := setupUploadHTTPTest(t)
	user.BackendScopes[0].MaxStorageBytes = 512
	writeSeedFile(t, filepath.Join(root, "copy-src.bin"), 300)

	reqBody := `{"action":"copy","items":[{"fromSource":"uploads","fromPath":"copy-src.bin","toSource":"uploads","toPath":"copy-dst.bin"}]}`
	req := httptest.NewRequest(http.MethodPatch, "/api/resources", strings.NewReader(reqBody))
	rec := httptest.NewRecorder()
	status, _ := ResourcePatchHandler(rec, req, &requestContext{User: user})
	if status != http.StatusInternalServerError {
		t.Fatalf("expected 500 for all-failed patch, got %d (body=%s)", status, rec.Body.String())
	}
	var resp MoveCopyResponse
	if err := json.NewDecoder(rec.Body).Decode(&resp); err != nil {
		t.Fatalf("decoding patch response: %v", err)
	}
	if len(resp.Failed) != 1 || !strings.Contains(resp.Failed[0].Message, "quota") {
		t.Fatalf("expected one quota failure, got %+v", resp.Failed)
	}
	if _, err := os.Stat(filepath.Join(root, "copy-dst.bin")); !os.IsNotExist(err) {
		t.Fatal("copy destination must not exist when the quota is exceeded")
	}
}
