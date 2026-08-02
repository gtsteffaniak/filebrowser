package icons

import (
	"encoding/json"
	"strings"
	"testing"
)

func TestPWAManifestNameUnderLimit(t *testing.T) {
	if got := pwaManifestName("EDIflyer FileBrowser"); got != "EDIflyer FileBrowser" {
		t.Fatalf("expected unchanged name, got %q", got)
	}
}

func TestPWAManifestNameTruncatesOverLimit(t *testing.T) {
	longName := "Baldwin Museum of Science - visit today!"
	want := "Baldwin Museum of Science - vi"
	if got := pwaManifestName(longName); got != want {
		t.Fatalf("expected %q, got %q", want, got)
	}
}

func TestPWAManifestNameTruncatesRunes(t *testing.T) {
	name := strings.Repeat("あ", 40)
	got := pwaManifestName(name)
	if len([]rune(got)) != pwaManifestNameMaxLen {
		t.Fatalf("expected %d runes, got %d: %q", pwaManifestNameMaxLen, len([]rune(got)), got)
	}
}

func TestGeneratePWAManifestUsesCappedNameOnly(t *testing.T) {
	manifest := generatePWAManifest(
		"EDIflyer FileBrowser",
		"File browser",
		"/testing/",
		"#455a64",
		"/testing/public/static/icons/pwa-icon-192.png",
		"/testing/public/static/icons/pwa-icon-256.png",
		"/testing/public/static/icons/pwa-icon-512.png",
	)

	if manifest.Name != "EDIflyer FileBrowser" {
		t.Fatalf("unexpected name: %q", manifest.Name)
	}

	data, err := json.Marshal(manifest)
	if err != nil {
		t.Fatalf("marshal manifest: %v", err)
	}
	if !containsJSONField(data, "name") {
		t.Fatalf("expected name in manifest JSON, got %s", data)
	}
	if containsJSONField(data, "short_name") {
		t.Fatalf("expected short_name to be omitted from manifest JSON, got %s", data)
	}
}

func containsJSONField(data []byte, field string) bool {
	var raw map[string]json.RawMessage
	if err := json.Unmarshal(data, &raw); err != nil {
		return false
	}
	_, ok := raw[field]
	return ok
}
