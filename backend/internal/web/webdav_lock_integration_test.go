package web

import (
	"github.com/gtsteffaniak/filebrowser/backend/internal/database/users"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestWebDAVLockingAcrossScopesAndAPI(t *testing.T) {
	root, _ := setupWebDAVTestEnv(t)
	user := func(id uint64, scope string) *users.User {
		return &users.User{ID: id, FrontendUser: users.FrontendUser{Username: "lock-test"}, BackendScopes: []users.BackendScope{{Path: root, Scope: scope}}, BackendSourcePermissions: webDAVPermsForPaths(true, true, true, true, root), Version: users.SourcePermissionsMigrationVersion}
	}
	broad, narrow := user(1, "/"), user(2, "/public")
	target := filepath.Join(root, "public", "readme.txt")
	request := func(u *users.User, method, path, body string, headers map[string]string) *httptest.ResponseRecorder {
		r := httptest.NewRequest(method, "/dav/source1"+path, strings.NewReader(body))
		r.SetPathValue("source", "source1")
		r.SetPathValue("path", path)
		for k, v := range headers {
			r.Header.Set(k, v)
		}
		w := httptest.NewRecorder()
		status, err := webDAVHandler(w, r, &Context{User: u})
		if err != nil && status != 200 && w.Code == 200 {
			w.Code = status
		}
		return w
	}
	lockXML := `<D:lockinfo xmlns:D="DAV:"><D:lockscope><D:exclusive/></D:lockscope><D:locktype><D:write/></D:locktype><D:owner>test</D:owner></D:lockinfo>`
	w := request(broad, "LOCK", "/public/readme.txt", lockXML, map[string]string{"Depth": "0", "Timeout": "Second-60"})
	if w.Code != 200 {
		t.Fatalf("LOCK: %d %s", w.Code, w.Body.String())
	}
	token := w.Header().Get("Lock-Token")
	if w = request(narrow, "PUT", "/readme.txt", "blocked", nil); w.Code != 423 {
		t.Fatalf("cross-scope PUT: %d", w.Code)
	}
	r := httptest.NewRequest("PUT", "/api/resources?source=source1&path=/public/readme.txt", strings.NewReader("API bypass"))
	status, err := resourcePutHandler(httptest.NewRecorder(), r, &Context{User: broad})
	if status != 423 {
		t.Fatalf("API write: %d %v", status, err)
	}

	other := request(broad, "LOCK", "/public/other.txt", lockXML, map[string]string{"Depth": "0", "Timeout": "Second-60"})
	if other.Code != 201 {
		t.Fatalf("other lock %d", other.Code)
	}
	otherToken := other.Header().Get("Lock-Token")
	tagged := "<http://example.com/dav/source1/public/other.txt> (" + otherToken + ")"
	if bad := request(broad, "PUT", "/public/readme.txt", "tagged bypass", map[string]string{"If": tagged}); bad.Code != 412 {
		t.Fatalf("tagged If bypass: %d", bad.Code)
	}
	request(broad, "UNLOCK", "/public/other.txt", "", map[string]string{"Lock-Token": otherToken})
	data, _ := os.ReadFile(target)
	if string(data) != "public content" {
		t.Fatal("blocked write changed file")
	}
	if w = request(narrow, "PUT", "/readme.txt", "authorized", map[string]string{"If": "(" + token + ")"}); w.Code != 201 {
		t.Fatalf("token write: %d %s", w.Code, w.Body.String())
	}
	if w = request(narrow, "UNLOCK", "/readme.txt", "", map[string]string{"Lock-Token": token}); w.Code != 204 {
		t.Fatalf("UNLOCK: %d", w.Code)
	}
	if w = request(broad, "PUT", "/public/readme.txt", "stale", map[string]string{"If-Match": `"wrong"`}); w.Code != 412 {
		t.Fatalf("If-Match: %d", w.Code)
	}
	data, _ = os.ReadFile(target)
	if string(data) != "authorized" {
		t.Fatal("precondition changed file")
	}
}
