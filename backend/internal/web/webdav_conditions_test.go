package web

import (
	"fmt"
	"golang.org/x/net/webdav"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"testing"
)

func guardedDAV(dir string, locks webdav.LockSystem) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		fs := &conditionalDAVFS{FileSystem: webdav.Dir(dir), request: r, name: r.URL.Path}
		lockView := newScopedLockSystem(locks, filepath.ToSlash(dir))
		lockView.recursiveMutation = r.Method == "DELETE" || r.Method == "MOVE" || r.Method == "COPY"
		h := &webdav.Handler{FileSystem: fs, LockSystem: lockView}
		h.ServeHTTP(&conditionalDAVWriter{ResponseWriter: w, fs: fs}, r)
	})
}
func davRequest(h http.Handler, method, path, body string, headers map[string]string) *httptest.ResponseRecorder {
	r := httptest.NewRequest(method, path, strings.NewReader(body))
	for k, v := range headers {
		r.Header.Set(k, v)
	}
	w := httptest.NewRecorder()
	h.ServeHTTP(w, r)
	return w
}
func TestDAVHTTPPreconditionsBeforeTruncation(t *testing.T) {
	dir := t.TempDir()
	name := filepath.Join(dir, "file")
	if err := os.WriteFile(name, []byte("original"), 0600); err != nil {
		t.Fatal(err)
	}
	h := guardedDAV(dir, webdav.NewMemLS())
	tag := davRequest(h, "GET", "/file", "", nil).Header().Get("ETag")
	for _, headers := range []map[string]string{{"If-Match": `"wrong"`}, {"If-Match": "W/" + tag}, {"If-None-Match": "*"}, {"If-None-Match": "W/" + tag}, {"If-Unmodified-Since": "Mon, 02 Jan 2006 15:04:05 GMT"}} {
		w := davRequest(h, "PUT", "/file", "destroy", headers)
		if w.Code != 412 {
			t.Errorf("headers %v: status %d", headers, w.Code)
		}
		data, _ := os.ReadFile(name)
		if string(data) != "original" {
			t.Fatal("failed condition changed file")
		}
	}
	if w := davRequest(h, "PUT", "/missing", "new", map[string]string{"If-Match": "*"}); w.Code != 412 {
		t.Fatal(w.Code)
	}
	if _, err := os.Stat(filepath.Join(dir, "missing")); !os.IsNotExist(err) {
		t.Fatal("failed condition created file")
	}
	if w := davRequest(h, "PUT", "/file", "updated", map[string]string{"If-Match": tag}); w.Code != 201 {
		t.Fatal(w.Code, w.Body.String())
	}
}
func TestDAVRealHTTPLockLifecycle(t *testing.T) {
	dir := t.TempDir()
	if err := os.WriteFile(filepath.Join(dir, "file"), []byte("original"), 0600); err != nil {
		t.Fatal(err)
	}
	h := guardedDAV(dir, webdav.NewMemLS())
	body := `<D:lockinfo xmlns:D="DAV:"><D:lockscope><D:exclusive/></D:lockscope><D:locktype><D:write/></D:locktype><D:owner>test</D:owner></D:lockinfo>`
	w := davRequest(h, "LOCK", "/file", body, map[string]string{"Depth": "0", "Timeout": "Second-60"})
	if w.Code != 200 {
		t.Fatal(w.Code, w.Body.String())
	}
	token := w.Header().Get("Lock-Token")
	if token == "" || strings.Contains(token, "nginxfake") {
		t.Fatal("missing real lock")
	}
	for _, method := range []string{"PUT", "DELETE"} {
		w = davRequest(h, method, "/file", "blocked", nil)
		if w.Code != 423 {
			t.Errorf("%s without token: %d", method, w.Code)
		}
	}
	w = davRequest(h, "PUT", "/file", "authorized", map[string]string{"If": "(" + token + ")"})
	if w.Code != 201 {
		t.Fatal(w.Code, w.Body.String())
	}
	w = davRequest(h, "UNLOCK", "/file", "", map[string]string{"Lock-Token": token})
	if w.Code != 204 {
		t.Fatal(w.Code)
	}
	if w = davRequest(h, "PUT", "/file", "after unlock", nil); w.Code != 201 {
		t.Fatal(w.Code)
	}
}
func TestDAVConcurrentIfMatchHasOneWinner(t *testing.T) {
	dir := t.TempDir()
	if err := os.WriteFile(filepath.Join(dir, "file"), []byte("original"), 0600); err != nil {
		t.Fatal(err)
	}
	h := guardedDAV(dir, webdav.NewMemLS())
	tag := davRequest(h, "GET", "/file", "", nil).Header().Get("ETag")
	var wg sync.WaitGroup
	results := make(chan int, 20)
	start := make(chan struct{})
	for i := 0; i < 20; i++ {
		wg.Add(1)
		go func(i int) {
			defer wg.Done()
			<-start
			w := davRequest(h, "PUT", "/file", fmt.Sprintf("writer-%d", i), map[string]string{"If-Match": tag})
			results <- w.Code
		}(i)
	}
	close(start)
	wg.Wait()
	close(results)
	wins := 0
	for status := range results {
		if status == 201 {
			wins++
		} else if status != 412 && status != 423 {
			t.Errorf("unexpected status %d", status)
		}
	}
	if wins != 1 {
		t.Fatalf("got %d successful writers with same old ETag", wins)
	}
}
func TestDAVConditionalDeleteAndMovePreserveFiles(t *testing.T) {
	dir := t.TempDir()
	if err := os.WriteFile(filepath.Join(dir, "file"), []byte("original"), 0600); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(dir, "dest"), []byte("destination"), 0600); err != nil {
		t.Fatal(err)
	}
	h := guardedDAV(dir, webdav.NewMemLS())
	for _, method := range []string{"DELETE", "MOVE"} {
		headers := map[string]string{"If-Match": `"wrong"`}
		if method == "MOVE" {
			headers["Destination"] = "http://example.com/dest"
			headers["Overwrite"] = "T"
		}
		w := davRequest(h, method, "/file", "", headers)
		if w.Code != 412 {
			t.Errorf("%s: %d %s", method, w.Code, w.Body.String())
		}
		data, _ := os.ReadFile(filepath.Join(dir, "file"))
		if string(data) != "original" {
			t.Fatal("source changed")
		}
		data, _ = os.ReadFile(filepath.Join(dir, "dest"))
		if string(data) != "destination" {
			t.Fatal("destination changed")
		}
	}
}
