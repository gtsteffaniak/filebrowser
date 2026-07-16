package web

import (
	"net/http"
	"net/http/httptest"
	"net/url"
	"testing"

	"github.com/gtsteffaniak/filebrowser/backend/internal/adapters/fs/files"
	"github.com/gtsteffaniak/filebrowser/backend/internal/database/users"
	"github.com/gtsteffaniak/filebrowser/backend/internal/utils"
	"github.com/gtsteffaniak/filebrowser/backend/pkg/indexing/iteminfo"
	"github.com/gtsteffaniak/filebrowser/backend/pkg/settings"
)

func TestOnlyOfficeClientConfigDeniedWithoutView(t *testing.T) {
	initStreamGrantTestSources(t)

	origOnlyOffice := settings.Config.Integrations.OnlyOffice
	origNameToSource := settings.Config.Server.NameToSource
	t.Cleanup(func() {
		settings.Config.Integrations.OnlyOffice = origOnlyOffice
		settings.Config.Server.NameToSource = origNameToSource
	})

	settings.Config.Integrations.OnlyOffice.Url = "http://onlyoffice.example"
	settings.Config.Server.NameToSource = map[string]*settings.Source{
		"default": {Name: "default", Path: "/default"},
	}

	originalFileInfoFaster := files.FileInfoFasterFunc
	t.Cleanup(func() { files.FileInfoFasterFunc = originalFileInfoFaster })
	files.FileInfoFasterFunc = func(opts utils.FileOptions, user *users.User) (*iteminfo.ExtendedFileInfo, error) {
		return &iteminfo.ExtendedFileInfo{
			FileInfo: iteminfo.FileInfo{
				ItemInfo: iteminfo.ItemInfo{Name: "doc.docx", Type: "application/vnd.openxmlformats-officedocument.wordprocessingml.document"},
				Path:     opts.Path,
			},
			RealPath: "/tmp/doc.docx",
		}, nil
	}

	user := testUserWithSourcePerms("/default", users.SourceFilePermissions{
		View: false, Download: true, Modify: true,
	})
	d := &requestContext{User: user}

	req := httptest.NewRequest(http.MethodGet, "/api/office/config?source=default&path=/doc.docx", nil)
	status, err := onlyofficeClientConfigGetHandler(httptest.NewRecorder(), req, d)
	if status != http.StatusForbidden {
		t.Fatalf("status = %d, want %d (err: %v)", status, http.StatusForbidden, err)
	}
}

func TestWebDAVMethodPermissionMatrix(t *testing.T) {
	viewOnly := users.SourceFilePermissions{View: true}
	if status, err := webDAVMethodPermission("PROPFIND", viewOnly); err != nil || status != 0 {
		t.Fatalf("PROPFIND with view: status=%d err=%v", status, err)
	}
	if status, err := webDAVMethodPermission(http.MethodGet, viewOnly); err == nil || status != http.StatusForbidden {
		t.Fatalf("GET without download: status=%d err=%v", status, err)
	}

	downloadOnly := users.SourceFilePermissions{Download: true}
	if status, err := webDAVMethodPermission("PROPFIND", downloadOnly); err == nil || status != http.StatusForbidden {
		t.Fatalf("PROPFIND without view: status=%d err=%v", status, err)
	}
	if status, err := webDAVMethodPermission(http.MethodGet, downloadOnly); err != nil || status != 0 {
		t.Fatalf("GET with download: status=%d err=%v", status, err)
	}
}

func TestResolveOnlyOfficeDownloadURL(t *testing.T) {
	orig := settings.Config.Integrations.OnlyOffice
	t.Cleanup(func() {
		settings.Config.Integrations.OnlyOffice = orig
	})

	settings.Config.Integrations.OnlyOffice.Url = "http://192.168.88.100:8282"
	settings.Config.Integrations.OnlyOffice.InternalUrl = "http://onlyoffice"

	cachePath := "/cache/files/data/doc_1/output.ods/output.ods?md5=abc&expires=1"
	publicURL := "http://192.168.88.100:8282" + cachePath
	internalURL := "http://onlyoffice" + cachePath

	tests := []struct {
		name  string
		input string
		want  string
	}{
		{
			name:  "rewrites public cache URL to internal host",
			input: publicURL,
			want:  internalURL,
		},
		{
			name:  "empty URL",
			input: "",
			want:  "",
		},
		{
			name:  "untrusted host rejected",
			input: "http://other-host:8282" + cachePath,
			want:  "",
		},
		{
			name:  "non-http scheme rejected",
			input: "ftp://192.168.88.100:8282" + cachePath,
			want:  "",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := resolveOnlyOfficeDownloadURL(tt.input)
			if got != tt.want {
				t.Errorf("resolveOnlyOfficeDownloadURL() = %q, want %q", got, tt.want)
			}
		})
	}

	t.Run("pass through public URL when internalUrl unset", func(t *testing.T) {
		settings.Config.Integrations.OnlyOffice.InternalUrl = ""
		got := resolveOnlyOfficeDownloadURL(publicURL)
		if got != publicURL {
			t.Errorf("resolveOnlyOfficeDownloadURL() = %q, want %q", got, publicURL)
		}
	})

	t.Run("hostname match with default http port", func(t *testing.T) {
		settings.Config.Integrations.OnlyOffice.Url = "http://office.local"
		settings.Config.Integrations.OnlyOffice.InternalUrl = "http://onlyoffice"
		input := "http://office.local:80" + cachePath
		want := "http://onlyoffice" + cachePath
		got := resolveOnlyOfficeDownloadURL(input)
		if got != want {
			t.Errorf("resolveOnlyOfficeDownloadURL() = %q, want %q", got, want)
		}
	})

	t.Run("reject when public url not configured", func(t *testing.T) {
		settings.Config.Integrations.OnlyOffice.Url = ""
		settings.Config.Integrations.OnlyOffice.InternalUrl = "http://onlyoffice"
		got := resolveOnlyOfficeDownloadURL(publicURL)
		if got != "" {
			t.Errorf("resolveOnlyOfficeDownloadURL() = %q, want empty", got)
		}
	})
}

func TestOnlyOfficeURLHostsMatch(t *testing.T) {
	mustParse := func(raw string) *url.URL {
		t.Helper()
		u, err := url.Parse(raw)
		if err != nil {
			t.Fatalf("url.Parse(%q): %v", raw, err)
		}
		return u
	}

	if !onlyOfficeURLHostsMatch(mustParse("http://office.local:80/x"), mustParse("http://office.local/x")) {
		t.Fatal("expected office.local:80 to match office.local with default port")
	}
	if onlyOfficeURLHostsMatch(mustParse("http://evil.example/x"), mustParse("http://office.local/x")) {
		t.Fatal("expected different hostnames not to match")
	}
}
