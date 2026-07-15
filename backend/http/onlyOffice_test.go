package http

import (
	"fmt"
	"net/url"
	"testing"

	"github.com/gtsteffaniak/filebrowser/backend/adapters/fs/files"
	"github.com/gtsteffaniak/filebrowser/backend/common/utils"
	"github.com/gtsteffaniak/filebrowser/backend/database/access"
	"github.com/gtsteffaniak/filebrowser/backend/database/share"
	"github.com/gtsteffaniak/filebrowser/backend/database/storage/bolt"
	"github.com/gtsteffaniak/filebrowser/backend/database/users"
	"github.com/gtsteffaniak/filebrowser/backend/indexing/iteminfo"
)

func TestResolveOnlyOfficeDownloadURL(t *testing.T) {
	orig := config.Integrations.OnlyOffice
	t.Cleanup(func() {
		config.Integrations.OnlyOffice = orig
	})

	config.Integrations.OnlyOffice.Url = "http://192.168.88.100:8282"
	config.Integrations.OnlyOffice.InternalUrl = "http://onlyoffice"

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
		config.Integrations.OnlyOffice.InternalUrl = ""
		got := resolveOnlyOfficeDownloadURL(publicURL)
		if got != publicURL {
			t.Errorf("resolveOnlyOfficeDownloadURL() = %q, want %q", got, publicURL)
		}
	})

	t.Run("hostname match with default http port", func(t *testing.T) {
		config.Integrations.OnlyOffice.Url = "http://office.local"
		config.Integrations.OnlyOffice.InternalUrl = "http://onlyoffice"
		input := "http://office.local:80" + cachePath
		want := "http://onlyoffice" + cachePath
		got := resolveOnlyOfficeDownloadURL(input)
		if got != want {
			t.Errorf("resolveOnlyOfficeDownloadURL() = %q, want %q", got, want)
		}
	})

	t.Run("reject when public url not configured", func(t *testing.T) {
		config.Integrations.OnlyOffice.Url = ""
		config.Integrations.OnlyOffice.InternalUrl = "http://onlyoffice"
		got := resolveOnlyOfficeDownloadURL(publicURL)
		if got != "" {
			t.Errorf("resolveOnlyOfficeDownloadURL() = %q, want empty", got)
		}
	})
}

func TestDeleteOfficeId(t *testing.T) {
	const rawPath = "/docs/document.docx"

	tests := []struct {
		name      string
		resolve   func(utils.FileOptions) (*iteminfo.ExtendedFileInfo, error)
		deleteKey string
		remainKey string
	}{
		{
			name: "deletes resolved realpath",
			resolve: func(utils.FileOptions) (*iteminfo.ExtendedFileInfo, error) {
				return &iteminfo.ExtendedFileInfo{RealPath: "/some/path/document.docx"}, nil
			},
			deleteKey: "/some/path/document.docx",
			remainKey: rawPath,
		},
		{
			name: "fallback to raw path on error",
			resolve: func(utils.FileOptions) (*iteminfo.ExtendedFileInfo, error) {
				return nil, fmt.Errorf("could not resolve path")
			},
			deleteKey: rawPath,
			remainKey: "/some/path/document.docx",
		},
		{
			name: "also deletes realpath when partially resolved before erroring",
			resolve: func(utils.FileOptions) (*iteminfo.ExtendedFileInfo, error) {
				return &iteminfo.ExtendedFileInfo{RealPath: "/some/path/document.docx"}, fmt.Errorf("access check failed")
			},
			deleteKey: "/some/path/document.docx",
			remainKey: rawPath,
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			origStore := store
			store = &bolt.BoltStore{Access: &access.Storage{}, Share: &share.Storage{}}
			t.Cleanup(func() { store = origStore })

			origFunc := files.FileInfoFasterFunc
			t.Cleanup(func() { files.FileInfoFasterFunc = origFunc })
			files.FileInfoFasterFunc = func(opts utils.FileOptions, accessStore *access.Storage, user *users.User, shareStore *share.Storage) (*iteminfo.ExtendedFileInfo, error) {
				if opts.Path != rawPath {
					t.Errorf("expected opts.Path=%q, got %q", rawPath, opts.Path)
				}
				if opts.Source != "source" {
					t.Errorf("expected opts.Source=%q, got %q", "source", opts.Source)
				}
				if !opts.FollowSymlinks {
					t.Error("expected FollowSymlinks=true")
				}
				return tt.resolve(opts)
			}
			utils.OnlyOfficeCache.Set(tt.deleteKey, "document-key")
			utils.OnlyOfficeCache.Set(tt.remainKey, "other-key")
			t.Cleanup(func() {
				utils.OnlyOfficeCache.Delete(tt.deleteKey)
				utils.OnlyOfficeCache.Delete(tt.remainKey)
			})
			deleteOfficeId("source", rawPath, &users.User{})
			if _, err := getOnlyOfficeId(tt.deleteKey); err == nil {
				t.Errorf("expected cache entry %q to be deleted", tt.deleteKey)
			}
			if _, err := getOnlyOfficeId(tt.remainKey); err != nil {
				t.Errorf("expected cache entry %q to remain, but it was deleted", tt.remainKey)
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
