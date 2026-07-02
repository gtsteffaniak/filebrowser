package settings

import (
	"regexp"
	"testing"
)

func TestDefaultApiLogExcludePattern(t *testing.T) {
	oldBaseURL := Config.Server.BaseURL
	defer func() { Config.Server.BaseURL = oldBaseURL }()

	tests := []struct {
		name          string
		baseURL       string
		path          string
		shouldExclude bool
	}{
		{
			name:          "base url static icons",
			baseURL:       "/testing",
			path:          "/testing/public/static/icons/pwa-icon-192.png",
			shouldExclude: true,
		},
		{
			name:          "root static assets",
			baseURL:       "/",
			path:          "/public/static/assets/index.js",
			shouldExclude: true,
		},
		{
			name:          "api path not excluded",
			baseURL:       "/testing",
			path:          "/testing/api/users?username=self",
			shouldExclude: false,
		},
		{
			name:          "base url health",
			baseURL:       "/testing/",
			path:          "/testing/health",
			shouldExclude: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			Config.Server.BaseURL = tt.baseURL
			re, err := regexp.Compile(defaultApiLogExcludePattern())
			if err != nil {
				t.Fatalf("compile pattern: %v", err)
			}
			got := re.MatchString(tt.path)
			if got != tt.shouldExclude {
				t.Fatalf("MatchString(%q) = %v, want %v (pattern %q)", tt.path, got, tt.shouldExclude, re.String())
			}
		})
	}
}
