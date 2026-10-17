package settings

import (
	"testing"

	"github.com/go-playground/validator/v10"
	"github.com/goccy/go-yaml"
)

func TestCopyPathSourceConfigYAML(t *testing.T) {
	var cfg Settings
	err := yaml.Unmarshal([]byte(`server:
  sources:
    - name: Home
      path: /mnt/home
      config:
        copyPathPrefix: /home/rt
    - name: RT
      path: /mnt/rt
      config:
        copyPathWindows: true
    - name: Legacy
      path: /private/host/path
`), &cfg)
	if err != nil {
		t.Fatal(err)
	}
	if got := cfg.Server.Sources[0].Config; got.CopyPathPrefix != "/home/rt" || got.CopyPathWindows {
		t.Fatalf("Home config = %+v", got)
	}
	if got := cfg.Server.Sources[1].Config; got.CopyPathPrefix != "" || !got.CopyPathWindows {
		t.Fatalf("RT config = %+v", got)
	}
	if got := cfg.Server.Sources[2].Config; got.CopyPathPrefix != "" || got.CopyPathWindows {
		t.Fatalf("legacy source must not infer a mapping from its host path: %+v", got)
	}
}

func TestCopyPathPrefixValidation(t *testing.T) {
	validate := validator.New()
	for _, tt := range []struct {
		prefix string
		valid  bool
	}{
		{"", true},
		{"/", true},
		{"/home/rt/", true},
		{"/D/work", true},
		{"relative/path", false},
		{`D:\work`, false},
	} {
		t.Run(tt.prefix, func(t *testing.T) {
			err := validate.Struct(SourceConfig{CopyPathPrefix: tt.prefix})
			if (err == nil) != tt.valid {
				t.Fatalf("prefix %q: error=%v, want valid=%v", tt.prefix, err, tt.valid)
			}
		})
	}
}
