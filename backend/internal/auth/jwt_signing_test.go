package auth

import (
	"testing"

	"github.com/gtsteffaniak/filebrowser/backend/pkg/settings"
)

func TestJWTSigningKeyFuncRejectsEmptyKey(t *testing.T) {
	orig := settings.Config.Auth.Key
	settings.Config.Auth.Key = ""
	t.Cleanup(func() { settings.Config.Auth.Key = orig })

	_, err := JWTSigningKeyFunc()(nil)
	if err == nil {
		t.Fatal("expected error for empty signing key")
	}
}

func TestJWTSigningKeyFuncAcceptsConfiguredKey(t *testing.T) {
	orig := settings.Config.Auth.Key
	settings.Config.Auth.Key = "test-key"
	t.Cleanup(func() { settings.Config.Auth.Key = orig })

	key, err := JWTSigningKeyFunc()(nil)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if string(key.([]byte)) != "test-key" {
		t.Fatalf("got key %q", key)
	}
}
