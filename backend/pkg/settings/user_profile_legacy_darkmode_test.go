package settings

import (
	"testing"

	"github.com/gtsteffaniak/filebrowser/backend/internal/database/users"
)

// v2.2.0 stores ui.darkMode as "auto", "dark", or "light". v2.1.6 must still load those
// profiles so a rollback does not stop startup.
func TestApplyProfileToUser_legacyStringDarkMode(t *testing.T) {
	tests := []struct {
		name  string
		value string
		want  users.DarkMode
	}{
		{"dark", `"dark"`, users.DarkModeDark},
		{"light", `"light"`, users.DarkModeLight},
		{"auto is preserved", `"auto"`, users.DarkModeAuto},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			u := &users.User{}
			profile := []byte(`{"ui":{"darkMode":` + tt.value + `}}`)
			if err := ApplyProfileToUser(u, profile); err != nil {
				t.Fatalf("ApplyProfileToUser: %v", err)
			}
			if u.DarkMode != tt.want {
				t.Fatalf("DarkMode = %v, want %v", u.DarkMode, tt.want)
			}
		})
	}
}

func TestApplyProfileToUser_unknownStringDarkModeStillFails(t *testing.T) {
	u := &users.User{}
	if err := ApplyProfileToUser(u, []byte(`{"ui":{"darkMode":"purple"}}`)); err == nil {
		t.Fatal("expected unrecognized darkMode string to be rejected")
	}
}

func TestApplyProfileToUser_boolDarkModeUnchanged(t *testing.T) {
	u := &users.User{}
	if err := ApplyProfileToUser(u, []byte(`{"ui":{"darkMode":false}}`)); err != nil {
		t.Fatalf("ApplyProfileToUser: %v", err)
	}
	if u.DarkMode != users.DarkModeLight {
		t.Fatalf("DarkMode = %v, want %v", u.DarkMode, users.DarkModeLight)
	}
}

// v2.2.0-shaped profile, as written to user_data.profile by the v2.2.0 release. Only ui.darkMode
// differs between the cases. The rest must still load, so a rollback keeps the user's other settings.
func TestApplyProfileToUser_v220ProfileRollback(t *testing.T) {
	tests := []struct {
		name     string
		darkMode string
		want     users.DarkMode
	}{
		{"dark", `"dark"`, users.DarkModeDark},
		{"light", `"light"`, users.DarkModeLight},
		{"auto", `"auto"`, users.DarkModeAuto},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			profile := []byte(`{
				"sidebar": {"hideFiles": false},
				"listing": {"viewMode": "list"},
				"preview": {"image": true},
				"fileViewer": {"autoplayMedia": true},
				"search": {"disableOptions": false},
				"ui": {"darkMode": ` + tt.darkMode + `, "themeColor": "var(--blue)", "customTheme": "alternative", "locale": "en"},
				"fileLoading": {},
				"account": {"lockPassword": false}
			}`)
			u := &users.User{}
			if err := ApplyProfileToUser(u, profile); err != nil {
				t.Fatalf("ApplyProfileToUser failed for darkMode %s: %v", tt.darkMode, err)
			}
			if u.DarkMode != tt.want {
				t.Errorf("DarkMode = %v, want %v", u.DarkMode, tt.want)
			}
			if u.ThemeColor != "var(--blue)" {
				t.Errorf("ThemeColor = %q, want var(--blue)", u.ThemeColor)
			}
			if u.CustomTheme != "alternative" {
				t.Errorf("CustomTheme = %q, want alternative", u.CustomTheme)
			}
			if u.Locale != "en" {
				t.Errorf("Locale = %q, want en", u.Locale)
			}
		})
	}
}
