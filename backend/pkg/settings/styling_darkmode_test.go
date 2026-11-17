package settings

import (
	"encoding/json"
	"testing"

	"github.com/gtsteffaniak/filebrowser/backend/internal/database/users"
)

func TestDefaultDarkModePreferencePreservesAuto(t *testing.T) {
	original := Config.UserDefaults.UI.DarkMode
	t.Cleanup(func() { Config.UserDefaults.UI.DarkMode = original })

	Config.UserDefaults.UI.DarkMode = users.DarkModeAuto
	if got := DefaultDarkModePreference(); got != users.DarkModeAuto {
		t.Fatalf("DefaultDarkModePreference() = %q, want auto", got)
	}
	if DefaultDarkMode() {
		t.Fatal("DefaultDarkMode() should be false when preference is auto")
	}
	if !DefaultDarkModeIsAuto() {
		t.Fatal("DefaultDarkModeIsAuto() should be true when preference is auto")
	}

	raw, err := json.Marshal(DefaultDarkModePreference())
	if err != nil {
		t.Fatal(err)
	}
	if string(raw) != `"auto"` {
		t.Fatalf("json.Marshal preference = %s, want \"auto\"", raw)
	}
}
