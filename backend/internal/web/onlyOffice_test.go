package web

import (
	"fmt"
	"net/url"
	"testing"

	"github.com/gtsteffaniak/filebrowser/backend/internal/adapters/fs/files"
	"github.com/gtsteffaniak/filebrowser/backend/internal/database/users"
	"github.com/gtsteffaniak/filebrowser/backend/internal/utils"
	"github.com/gtsteffaniak/filebrowser/backend/pkg/indexing/iteminfo"
	"github.com/gtsteffaniak/filebrowser/backend/pkg/settings"
)

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
func TestDeleteOfficeId(t *testing.T) {
	const rawPath = "/docs/document.docx"

	tests := []struct {
		name     string
		resolve  func(utils.FileOptions) (*iteminfo.ExtendedFileInfo, error)
		cacheKey string
	}{
		{
			name: "deletes resolved realpath from cache",
			resolve: func(utils.FileOptions) (*iteminfo.ExtendedFileInfo, error) {
				return &iteminfo.ExtendedFileInfo{RealPath: "/some/path/document.docx"}, nil
			},
			cacheKey: "/some/path/document.docx",
		},
		{
			name: "fallback to raw path on error",
			resolve: func(utils.FileOptions) (*iteminfo.ExtendedFileInfo, error) {
				return nil, fmt.Errorf("could not resolve path")
			},
			cacheKey: rawPath,
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			origFunc := files.FileInfoFasterFunc
			t.Cleanup(func() { files.FileInfoFasterFunc = origFunc })
			files.FileInfoFasterFunc = func(opts utils.FileOptions, user *users.User) (*iteminfo.ExtendedFileInfo, error) {
				return tt.resolve(opts)
			}
			utils.OnlyOfficeCache.Set(tt.cacheKey, "document-key")
			deleteOfficeId("source", rawPath, &users.User{})
			if _, err := GetOnlyOfficeId(tt.cacheKey); err == nil {
				t.Errorf("expected cache entry %q to be deleted", tt.cacheKey)
			}
		})
	}
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
