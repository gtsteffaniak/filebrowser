package state

import (
	"strings"
	"testing"

	"github.com/gtsteffaniak/filebrowser/backend/pkg/settings"
)

func TestBootstrapDefaultAdminPasswordUsesNestedConfigWhenSet(t *testing.T) {
	origNested := settings.Config.Auth.Methods.PasswordAuth.AdminPassword
	origLegacy := settings.Config.Auth.AdminPassword
	defer func() {
		settings.Config.Auth.Methods.PasswordAuth.AdminPassword = origNested
		settings.Config.Auth.AdminPassword = origLegacy
	}()

	settings.Config.Auth.Methods.PasswordAuth.AdminPassword = "from-nested"
	settings.Config.Auth.AdminPassword = "from-legacy"
	plain, generated, err := bootstrapDefaultAdminPassword("admin")
	if err != nil {
		t.Fatal(err)
	}
	if generated {
		t.Fatal("expected generated=false when nested config password is set")
	}
	if plain != "from-nested" {
		t.Fatalf("password = %q, want from-nested", plain)
	}
}

func TestBootstrapDefaultAdminPasswordUsesTopLevelWhenNestedUnset(t *testing.T) {
	origNested := settings.Config.Auth.Methods.PasswordAuth.AdminPassword
	origLegacy := settings.Config.Auth.AdminPassword
	defer func() {
		settings.Config.Auth.Methods.PasswordAuth.AdminPassword = origNested
		settings.Config.Auth.AdminPassword = origLegacy
	}()

	settings.Config.Auth.Methods.PasswordAuth.AdminPassword = ""
	settings.Config.Auth.AdminPassword = "from-legacy"
	plain, generated, err := bootstrapDefaultAdminPassword("admin")
	if err != nil {
		t.Fatal(err)
	}
	if generated {
		t.Fatal("expected generated=false when top-level config password is set")
	}
	if plain != "from-legacy" {
		t.Fatalf("password = %q, want from-legacy", plain)
	}
}

func TestBootstrapDefaultAdminPasswordGeneratesWhenDefault(t *testing.T) {
	origNested := settings.Config.Auth.Methods.PasswordAuth.AdminPassword
	origLegacy := settings.Config.Auth.AdminPassword
	defer func() {
		settings.Config.Auth.Methods.PasswordAuth.AdminPassword = origNested
		settings.Config.Auth.AdminPassword = origLegacy
	}()

	for _, cfg := range []string{"", "admin"} {
		settings.Config.Auth.Methods.PasswordAuth.AdminPassword = cfg
		settings.Config.Auth.AdminPassword = cfg
		plain, generated, err := bootstrapDefaultAdminPassword("bootstrap-user")
		if err != nil {
			t.Fatalf("cfg=%q: %v", cfg, err)
		}
		if !generated {
			t.Fatalf("cfg=%q: expected generated password", cfg)
		}
		if plain == "" || plain == "admin" {
			t.Fatalf("cfg=%q: unexpected password %q", cfg, plain)
		}
		if len(plain) != bootstrapAdminPasswordHexBytes*2 {
			t.Fatalf("cfg=%q: password length %d, want %d", cfg, len(plain), bootstrapAdminPasswordHexBytes*2)
		}
		if strings.Trim(plain, "0123456789abcdef") != "" {
			t.Fatalf("cfg=%q: expected hex password, got %q", cfg, plain)
		}
	}
}
