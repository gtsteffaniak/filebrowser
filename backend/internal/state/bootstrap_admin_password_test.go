package state

import (
	"regexp"
	"strings"
	"testing"

	"github.com/gtsteffaniak/filebrowser/backend/pkg/settings"
)

func TestBootstrapWordsCount(t *testing.T) {
	if len(bootstrapWords) != 100 {
		t.Fatalf("bootstrapWords len = %d, want 100", len(bootstrapWords))
	}
}

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

var speakableBootstrapPasswordPattern = regexp.MustCompile(`^[a-z]+-[abcdefghjkmnpqrstuvwxyz23456789]{5}-[abcdefghjkmnpqrstuvwxyz23456789]{2}$`)

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
		if !speakableBootstrapPasswordPattern.MatchString(plain) {
			t.Fatalf("cfg=%q: password %q does not match speakable pattern", cfg, plain)
		}
		word := strings.Split(plain, "-")[0]
		found := false
		for _, w := range bootstrapWords {
			if w == word {
				found = true
				break
			}
		}
		if !found {
			t.Fatalf("cfg=%q: word %q not in bootstrap word list", cfg, word)
		}
	}
}

func TestGenerateSpeakableBootstrapPassword(t *testing.T) {
	for i := 0; i < 20; i++ {
		plain, err := generateSpeakableBootstrapPassword()
		if err != nil {
			t.Fatal(err)
		}
		if !speakableBootstrapPasswordPattern.MatchString(plain) {
			t.Fatalf("password %q does not match speakable pattern", plain)
		}
	}
}
