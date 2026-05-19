package http

import (
	"testing"

	"github.com/gtsteffaniak/filebrowser/backend/common/settings"
)

func TestRewriteOnlyOfficeIntegrationURL(t *testing.T) {
	orig := settings.Config.Integrations.OnlyOffice
	t.Cleanup(func() {
		settings.Config.Integrations.OnlyOffice = orig
	})

	settings.Config.Integrations.OnlyOffice.Url = "http://192.168.88.100:8282"
	settings.Config.Integrations.OnlyOffice.InternalUrl = "http://onlyoffice"

	cachePath := "/cache/files/data/doc_1/output.ods/output.ods?md5=abc&expires=1"
	publicURL := "http://192.168.88.100:8282" + cachePath
	want := "http://onlyoffice" + cachePath

	tests := []struct {
		name  string
		input string
		want  string
	}{
		{
			name:  "rewrites public cache URL to internal host",
			input: publicURL,
			want:  want,
		},
		{
			name:  "empty URL unchanged",
			input: "",
			want:  "",
		},
		{
			name:  "unrelated host unchanged",
			input: "http://other-host:8282" + cachePath,
			want:  "http://other-host:8282" + cachePath,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := rewriteOnlyOfficeIntegrationURL(tt.input)
			if got != tt.want {
				t.Errorf("rewriteOnlyOfficeIntegrationURL() = %q, want %q", got, tt.want)
			}
		})
	}

	t.Run("no rewrite when internalUrl unset", func(t *testing.T) {
		settings.Config.Integrations.OnlyOffice.InternalUrl = ""
		got := rewriteOnlyOfficeIntegrationURL(publicURL)
		if got != publicURL {
			t.Errorf("rewriteOnlyOfficeIntegrationURL() = %q, want %q", got, publicURL)
		}
	})
}
