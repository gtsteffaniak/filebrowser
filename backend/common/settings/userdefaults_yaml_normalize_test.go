package settings

import (
	"strings"
	"testing"

	"github.com/goccy/go-yaml"
)

func TestNormalizeUserDefaultsMap_groupedYAML(t *testing.T) {
	const y = `
userDefaults:
  listingOptions:
    gallerySize: 9
    showHidden: true
  sidebarOptions:
    disableHideSidebar: true
  themeLanguage:
    locale: uk
`

	var root map[string]interface{}
	if err := yaml.Unmarshal([]byte(y), &root); err != nil {
		t.Fatal(err)
	}
	ud := asStringMap(root["userDefaults"])
	_ = NormalizeUserDefaultsMap(ud)
	if toInt(ud["gallerySize"]) != 9 {
		t.Fatalf("gallerySize: %#v", ud["gallerySize"])
	}
	if sh, ok := ud["showHidden"].(bool); !ok || !sh {
		t.Fatalf("showHidden: %#v", ud["showHidden"])
	}
	if ud["locale"] != "uk" {
		t.Fatalf("locale: %#v", ud["locale"])
	}
	pm := asStringMap(ud["preview"])
	if pm == nil {
		t.Fatal("preview map missing")
	}
	if dhs, ok := pm["disableHideSidebar"].(bool); !ok || !dhs {
		t.Fatalf("preview.disableHideSidebar: %#v", pm["disableHideSidebar"])
	}

	cfg := Settings{}
	dec := yaml.NewDecoder(strings.NewReader(mustYAMLRoundTrip(t, ud)), yaml.DisallowUnknownField())
	if err := dec.Decode(&cfg); err != nil {
		t.Fatal(err)
	}
	if cfg.UserDefaults.GallerySize != 9 || !cfg.UserDefaults.ShowHidden {
		t.Fatalf("decode mismatch: gallery=%d hidden=%v", cfg.UserDefaults.GallerySize, cfg.UserDefaults.ShowHidden)
	}
	if cfg.UserDefaults.Locale != "uk" {
		t.Fatalf("locale decode %q", cfg.UserDefaults.Locale)
	}
	if !cfg.UserDefaults.Preview.DisableHideSidebar {
		t.Fatal("preview bool decode")
	}
}

func TestNormalizeUserDefaultsMap_searchAlias(t *testing.T) {
	ud := map[string]interface{}{
		"searchOptions": map[string]interface{}{
			"disableOptions": true,
		},
	}
	NormalizeUserDefaultsMap(ud)
	if v, ok := ud["disableSearchOptions"].(bool); !ok || !v {
		t.Fatalf("disableSearchOptions: %#v", ud["disableSearchOptions"])
	}
}

func mustYAMLRoundTrip(t *testing.T, ud map[string]interface{}) string {
	t.Helper()
	wrapped := map[string]interface{}{"userDefaults": ud}
	b, err := yaml.Marshal(wrapped)
	if err != nil {
		t.Fatal(err)
	}
	return string(b)
}

func toInt(v interface{}) int {
	switch x := v.(type) {
	case int:
		return x
	case int64:
		return int(x)
	case uint64:
		return int(x)
	case float64:
		return int(x)
	default:
		return -999999
	}
}
