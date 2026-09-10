package share

import (
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/gtsteffaniak/filebrowser/backend/pkg/settings"
)

func TestMintAndValidateDownloadAccessToken(t *testing.T) {
	origKey := settings.Config.Auth.Key
	settings.Config.Auth.Key = "test-auth-key"
	t.Cleanup(func() { settings.Config.Auth.Key = origKey })

	token, expiresAt, err := MintDownloadAccessToken("share123", time.Hour, 0)
	if err != nil {
		t.Fatalf("MintDownloadAccessToken: %v", err)
	}
	if token == "" || expiresAt <= time.Now().Unix() {
		t.Fatalf("unexpected token response: token=%q expiresAt=%d", token, expiresAt)
	}
	if !ValidateDownloadAccessToken(token, "share123") {
		t.Fatal("expected valid token for matching hash")
	}
	if ValidateDownloadAccessToken(token, "other-hash") {
		t.Fatal("expected invalid token for different hash")
	}
}

func TestDownloadTokenRejectedOnListingRoute(t *testing.T) {
	origKey := settings.Config.Auth.Key
	settings.Config.Auth.Key = "test-auth-key"
	t.Cleanup(func() { settings.Config.Auth.Key = origKey })

	token, _, err := MintDownloadAccessToken("share123", time.Hour, 0)
	if err != nil {
		t.Fatalf("MintDownloadAccessToken: %v", err)
	}
	if !ValidateDownloadAccessToken(token, "share123") {
		t.Fatal("expected valid token")
	}

	req := httptest.NewRequest(http.MethodGet, "/public/api/resources?hash=share123&token="+token, nil)
	if RequestAllowsDownloadToken(req) {
		t.Fatal("listing route should not allow download token auth")
	}

	downloadReq := httptest.NewRequest(http.MethodGet, "/public/api/resources/download?hash=share123&token="+token, nil)
	if !RequestAllowsDownloadToken(downloadReq) {
		t.Fatal("download route should allow download token auth")
	}
}

func TestDownloadAccessTokenUseCount(t *testing.T) {
	origKey := settings.Config.Auth.Key
	settings.Config.Auth.Key = "test-auth-key"
	t.Cleanup(func() { settings.Config.Auth.Key = origKey })

	token, _, err := MintDownloadAccessToken("share123", time.Hour, 1)
	if err != nil {
		t.Fatalf("MintDownloadAccessToken: %v", err)
	}
	if !ValidateDownloadAccessToken(token, "share123") {
		t.Fatal("expected valid token before consume")
	}
	ConsumeDownloadAccessToken(token)
	if ValidateDownloadAccessToken(token, "share123") {
		t.Fatal("expected token to be invalid after single use")
	}
}

func TestMintAndValidateShareUISessionToken(t *testing.T) {
	origKey := settings.Config.Auth.Key
	settings.Config.Auth.Key = "test-auth-key"
	t.Cleanup(func() { settings.Config.Auth.Key = origKey })

	token, expiresAt, err := MintShareUISessionToken("share123", time.Hour)
	if err != nil {
		t.Fatalf("MintShareUISessionToken: %v", err)
	}
	if token == "" || expiresAt <= time.Now().Unix() {
		t.Fatalf("unexpected token response: token=%q expiresAt=%d", token, expiresAt)
	}
	if !ValidateShareUISessionToken(token, "share123") {
		t.Fatal("expected valid UI session token for matching hash")
	}
	if ValidateShareUISessionToken(token, "other-hash") {
		t.Fatal("expected invalid UI session token for different hash")
	}
}

func TestParseAccessDurationMax24Hours(t *testing.T) {
	if _, err := ParseAccessDuration(25, "hours"); err == nil {
		t.Fatal("expected error for duration over 24 hours")
	}
	ttl, err := ParseAccessDuration(2, "hours")
	if err != nil {
		t.Fatalf("ParseAccessDuration: %v", err)
	}
	if ttl != 2*time.Hour {
		t.Fatalf("ttl = %v, want 2h", ttl)
	}
}
