package http

import (
	"net/url"
	"testing"
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
