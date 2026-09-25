package state

import (
	"strings"
	"testing"

	"github.com/gtsteffaniak/filebrowser/backend/pkg/settings"
)

func TestBootstrapDefaultAdminPasswordUsesConfigWhenSet(t *testing.T) {
	orig := settings.Config.Auth.AdminPassword
	defer func() { settings.Config.Auth.AdminPassword = orig }()

	settings.Config.Auth.AdminPassword = "from-config"
	plain, generated, err := bootstrapDefaultAdminPassword("admin")
	if err != nil {
		t.Fatal(err)
	}
	if generated {
		t.Fatal("expected generated=false when config password is set")
	}
	if plain != "from-config" {
		t.Fatalf("password = %q, want from-config", plain)
	}
}

func TestBootstrapDefaultAdminPasswordGeneratesWhenDefault(t *testing.T) {
	orig := settings.Config.Auth.AdminPassword
	defer func() { settings.Config.Auth.AdminPassword = orig }()

	for _, cfg := range []string{"", "admin"} {
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
