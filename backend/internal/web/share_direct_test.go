package web

import (
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/gtsteffaniak/filebrowser/backend/pkg/settings"
)

func TestMintAndValidateShareDownloadAccessToken(t *testing.T) {
	origKey := settings.Config.Auth.Key
	settings.Config.Auth.Key = "test-auth-key"
	t.Cleanup(func() { settings.Config.Auth.Key = origKey })

	token, expiresAt, err := mintShareDownloadAccessToken("share123", time.Hour, 0)
	if err != nil {
		t.Fatalf("mintShareDownloadAccessToken: %v", err)
	}
	if token == "" || expiresAt <= time.Now().Unix() {
		t.Fatalf("unexpected token response: token=%q expiresAt=%d", token, expiresAt)
	}
	if !validateShareDownloadAccessToken(token, "share123") {
		t.Fatal("expected valid token for matching hash")
	}
	if validateShareDownloadAccessToken(token, "other-hash") {
		t.Fatal("expected invalid token for different hash")
	}
}

func TestShareDownloadTokenRejectedOnListingRoute(t *testing.T) {
	origKey := settings.Config.Auth.Key
	settings.Config.Auth.Key = "test-auth-key"
	t.Cleanup(func() { settings.Config.Auth.Key = origKey })

	token, _, err := mintShareDownloadAccessToken("share123", time.Hour, 0)
	if err != nil {
		t.Fatalf("mintShareDownloadAccessToken: %v", err)
	}
	if !validateShareDownloadAccessToken(token, "share123") {
		t.Fatal("expected valid token")
	}

	req := httptest.NewRequest(http.MethodGet, "/public/api/resources?hash=share123&token="+token, nil)
	if shareRequestAllowsDownloadToken(req) {
		t.Fatal("listing route should not allow download token auth")
	}

	downloadReq := httptest.NewRequest(http.MethodGet, "/public/api/resources/download?hash=share123&token="+token, nil)
	if !shareRequestAllowsDownloadToken(downloadReq) {
		t.Fatal("download route should allow download token auth")
	}
}

func TestShareDownloadAccessTokenUseCount(t *testing.T) {
	origKey := settings.Config.Auth.Key
	settings.Config.Auth.Key = "test-auth-key"
	t.Cleanup(func() { settings.Config.Auth.Key = origKey })

	token, _, err := mintShareDownloadAccessToken("share123", time.Hour, 1)
	if err != nil {
		t.Fatalf("mintShareDownloadAccessToken: %v", err)
	}
	if !validateShareDownloadAccessToken(token, "share123") {
		t.Fatal("expected valid token before consume")
	}
	consumeShareDownloadAccessToken(token)
	if validateShareDownloadAccessToken(token, "share123") {
		t.Fatal("expected token to be invalid after single use")
	}
}

func TestMintAndValidateShareUISessionToken(t *testing.T) {
	origKey := settings.Config.Auth.Key
	settings.Config.Auth.Key = "test-auth-key"
	t.Cleanup(func() { settings.Config.Auth.Key = origKey })

	token, expiresAt, err := mintShareUISessionToken("share123", time.Hour)
	if err != nil {
		t.Fatalf("mintShareUISessionToken: %v", err)
	}
	if token == "" || expiresAt <= time.Now().Unix() {
		t.Fatalf("unexpected token response: token=%q expiresAt=%d", token, expiresAt)
	}
	if !validateShareUISessionToken(token, "share123") {
		t.Fatal("expected valid UI session token for matching hash")
	}
	if validateShareUISessionToken(token, "other-hash") {
		t.Fatal("expected invalid UI session token for different hash")
	}
}

func TestParseShareDownloadAccessDurationMax24Hours(t *testing.T) {
	if _, err := parseShareDownloadAccessDuration(25, "hours"); err == nil {
		t.Fatal("expected error for duration over 24 hours")
	}
	ttl, err := parseShareDownloadAccessDuration(2, "hours")
	if err != nil {
		t.Fatalf("parseShareDownloadAccessDuration: %v", err)
	}
	if ttl != 2*time.Hour {
		t.Fatalf("ttl = %v, want 2h", ttl)
	}
}
