package web

import (
	"encoding/xml"
	"net/http"
	"net/http/httptest"
	"path/filepath"
	"strings"
	"testing"

	"github.com/gtsteffaniak/filebrowser/backend/internal/database/users"
	"github.com/gtsteffaniak/filebrowser/backend/pkg/indexing"
	"github.com/gtsteffaniak/filebrowser/backend/pkg/settings"
)

const windowsDAVPatch = `<D:propertyupdate xmlns:D="DAV:" xmlns:Z="urn:schemas-microsoft-com:"><D:set><D:prop><Z:Win32CreationTime>Thu, 22 Oct 2026 10:00:00 GMT</Z:Win32CreationTime><Z:Win32LastModifiedTime>Thu, 22 Oct 2026 11:00:00 GMT</Z:Win32LastModifiedTime><Z:Win32FileAttributes>00000020</Z:Win32FileAttributes></D:prop></D:set></D:propertyupdate>`

func TestWebDAVWindowsExplorerUploadLifecycle(t *testing.T) {
	source, _ := setupWebDAVTestEnv(t)
	initTestIndex(t, "source1", source)
	oldDB := settings.Config.Server.DatabaseV2.Path
	settings.Config.Server.DatabaseV2.Path = filepath.Join(t.TempDir(), "database.sqlite")
	t.Cleanup(func() { settings.Config.Server.DatabaseV2.Path = oldDB })
	user := &users.User{ID: 1, FrontendUser: users.FrontendUser{Username: "windows-test"}, BackendScopes: []users.BackendScope{{Path: source, Scope: "/"}}, BackendSourcePermissions: webDAVPermsForPaths(true, true, true, true, source), Version: users.SourcePermissionsMigrationVersion}
	applyBackendSourcePerms(user, user.BackendSourcePermissions)
	do := func(method, name, body string, headers map[string]string, want int) *httptest.ResponseRecorder {
		t.Helper()
		r := httptest.NewRequest(method, "/dav/source1"+name, strings.NewReader(body))
		r.SetPathValue("source", "source1")
		r.SetPathValue("path", name)
		for key, value := range headers {
			r.Header.Set(key, value)
		}
		w := httptest.NewRecorder()
		status, err := webDAVHandler(w, r, &requestContext{User: user})
		if status != 0 && w.Code == http.StatusOK {
			w.Code = status
		}
		if w.Code != want {
			t.Fatalf("%s %s: got %d, want %d; err=%v body=%s", method, name, w.Code, want, err, w.Body.String())
		}
		return w
	}
	photo := "\xff\xd8test-photo-original\xff\xd9"
	do("MKCOL", "/public/windows-upload", "", nil, 201)
	name := "/public/windows-upload/photo.jpg"
	do("PUT", name, photo, nil, 201)
	lock := do("LOCK", name, `<D:lockinfo xmlns:D="DAV:"><D:lockscope><D:exclusive/></D:lockscope><D:locktype><D:write/></D:locktype></D:lockinfo>`, map[string]string{"Depth": "0", "Timeout": "Second-60"}, 200)
	token := lock.Header().Get("Lock-Token")
	if token == "" {
		t.Fatal("LOCK did not return a token")
	}
	do("PROPPATCH", name, windowsDAVPatch, nil, 423)
	do("PROPPATCH", name, windowsDAVPatch, map[string]string{"If": "(<urn:uuid:incorrect-token>)"}, 412)
	patched := do("PROPPATCH", name, windowsDAVPatch, map[string]string{"If": "(" + token + ")"}, 207)
	windowsDAVAssertProperties(t, patched.Body.String(), 200, false)
	findBody := `<D:propfind xmlns:D="DAV:" xmlns:Z="urn:schemas-microsoft-com:"><D:prop><Z:Win32CreationTime/><Z:Win32LastModifiedTime/><Z:Win32FileAttributes/></D:prop></D:propfind>`
	found := do("PROPFIND", name, findBody, map[string]string{"Depth": "0"}, 207)
	windowsDAVAssertProperties(t, found.Body.String(), 200, true)
	do("UNLOCK", name, "", map[string]string{"Lock-Token": token}, 204)
	// Collection properties must not require opening the directory O_RDWR.
	directoryPatch := do("PROPPATCH", "/public/windows-upload", windowsDAVPatch, nil, 207)
	windowsDAVAssertProperties(t, directoryPatch.Body.String(), 200, false)
	directoryFound := do("PROPFIND", "/public/windows-upload", findBody, map[string]string{"Depth": "0"}, 207)
	windowsDAVAssertProperties(t, directoryFound.Body.String(), 200, true)
	// A narrower user scope names the same physical object differently.
	user.BackendScopes[0].Scope = "/public"
	scoped := do("PROPFIND", "/windows-upload/photo.jpg", findBody, map[string]string{"Depth": "0"}, 207)
	windowsDAVAssertProperties(t, scoped.Body.String(), 200, true)
	do("PROPPATCH", "/windows-upload/photo.jpg", windowsDAVPatch, nil, 207)
	user.BackendScopes[0].Scope = "/"
	found = do("PROPFIND", name, findBody, map[string]string{"Depth": "0"}, 207)
	windowsDAVAssertProperties(t, found.Body.String(), 200, true)
	do("COPY", name, "", map[string]string{"Destination": "http://example.com/dav/source1/public/windows-upload/copy.jpg", "Overwrite": "F"}, 201)
	copied := do("PROPFIND", "/public/windows-upload/copy.jpg", findBody, map[string]string{"Depth": "0"}, 207)
	windowsDAVAssertProperties(t, copied.Body.String(), 200, true)
	if copyBody := do("GET", "/public/windows-upload/copy.jpg", "", nil, 200); copyBody.Body.String() != photo {
		t.Fatal("COPY changed photo bytes")
	}
	do("DELETE", "/public/windows-upload/copy.jpg", "", nil, 204)
	do("PUT", "/public/windows-upload/copy.jpg", photo, nil, 201)
	recreated := do("PROPFIND", "/public/windows-upload/copy.jpg", findBody, map[string]string{"Depth": "0"}, 207)
	windowsDAVAssertProperties(t, recreated.Body.String(), 404, false)
	protected := `<D:propertyupdate xmlns:D="DAV:" xmlns:Z="urn:schemas-microsoft-com:"><D:set><D:prop><D:getcontentlength>999</D:getcontentlength><Z:Win32FileAttributes>00000001</Z:Win32FileAttributes></D:prop></D:set></D:propertyupdate>`
	rejected := do("PROPPATCH", name, protected, nil, 207)
	if !strings.Contains(rejected.Body.String(), "403 Forbidden") || !strings.Contains(rejected.Body.String(), "424 Failed Dependency") {
		t.Fatalf("protected property patch was not atomic: %s", rejected.Body.String())
	}
	found = do("PROPFIND", name, findBody, map[string]string{"Depth": "0"}, 207)
	windowsDAVAssertProperties(t, found.Body.String(), 200, true)
	do("MOVE", "/public/windows-upload", "", map[string]string{"Destination": "http://example.com/dav/source1/public/windows-renamed", "Overwrite": "F"}, 201)
	got := do("GET", "/public/windows-renamed/photo.jpg", "", nil, 200)
	if got.Body.String() != photo {
		t.Fatal("photo bytes changed during metadata update or folder move")
	}
	found = do("PROPFIND", "/public/windows-renamed/photo.jpg", findBody, map[string]string{"Depth": "0"}, 207)
	windowsDAVAssertProperties(t, found.Body.String(), 200, true)
	directoryFound = do("PROPFIND", "/public/windows-renamed", findBody, map[string]string{"Depth": "0"}, 207)
	windowsDAVAssertProperties(t, directoryFound.Body.String(), 200, true)
	do("COPY", "/public/windows-renamed", "", map[string]string{"Destination": "http://example.com/dav/source1/public/windows-folder-copy", "Overwrite": "F"}, 201)
	folderCopy := do("PROPFIND", "/public/windows-folder-copy", findBody, map[string]string{"Depth": "0"}, 207)
	windowsDAVAssertProperties(t, folderCopy.Body.String(), 200, true)
	childCopy := do("PROPFIND", "/public/windows-folder-copy/photo.jpg", findBody, map[string]string{"Depth": "0"}, 207)
	windowsDAVAssertProperties(t, childCopy.Body.String(), 200, true)
	indexing.GetIndex("source1").Config.ReadOnly = true
	do("PROPPATCH", "/public/windows-renamed/photo.jpg", windowsDAVPatch, nil, 403)
	indexing.GetIndex("source1").Config.ReadOnly = false
	user.BackendSourcePermissions = webDAVPermsForPaths(true, false, false, false, source)
	applyBackendSourcePerms(user, user.BackendSourcePermissions)
	do("PROPPATCH", "/public/windows-renamed/photo.jpg", windowsDAVPatch, nil, 403)
}

func windowsDAVAssertProperties(t *testing.T, body string, wantStatus int, values bool) {
	t.Helper()
	var result struct {
		Responses []struct {
			Propstats []struct {
				Status string `xml:"status"`
				Prop   struct {
					Properties []struct {
						XMLName xml.Name
						Value   string `xml:",chardata"`
					} `xml:",any"`
				} `xml:"prop"`
			} `xml:"propstat"`
		} `xml:"response"`
	}
	if err := xml.Unmarshal([]byte(body), &result); err != nil {
		t.Fatal(err)
	}
	wanted := map[string]string{"Win32CreationTime": "Thu, 22 Oct 2026 10:00:00 GMT", "Win32LastModifiedTime": "Thu, 22 Oct 2026 11:00:00 GMT", "Win32FileAttributes": "00000020"}
	for _, response := range result.Responses {
		for _, stat := range response.Propstats {
			for _, prop := range stat.Prop.Properties {
				expected, ok := wanted[prop.XMLName.Local]
				if !ok || prop.XMLName.Space != "urn:schemas-microsoft-com:" {
					continue
				}
				if !strings.Contains(stat.Status, http.StatusText(wantStatus)) {
					t.Fatalf("%s returned %s: %s", prop.XMLName.Local, stat.Status, body)
				}
				if values && strings.TrimSpace(prop.Value) != expected {
					t.Fatalf("%s value %q, want %q", prop.XMLName.Local, prop.Value, expected)
				}
				delete(wanted, prop.XMLName.Local)
			}
		}
	}
	if len(wanted) != 0 {
		t.Fatalf("missing Microsoft properties %v: %s", wanted, body)
	}
}
