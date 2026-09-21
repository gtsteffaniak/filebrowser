package web

import (
	"bytes"
	"encoding/json"
	"fmt"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	jwt "github.com/golang-jwt/jwt/v4"
	"github.com/gtsteffaniak/filebrowser/backend/internal/adapters/fs/files"
	"github.com/gtsteffaniak/filebrowser/backend/internal/database/users"
	"github.com/gtsteffaniak/filebrowser/backend/internal/utils"
	"github.com/gtsteffaniak/filebrowser/backend/pkg/indexing/iteminfo"
	"github.com/gtsteffaniak/filebrowser/backend/pkg/settings"
)

func signOnlyOfficeCallbackJWT(secret string, payload map[string]interface{}) string {
	token := jwt.NewWithClaims(jwt.SigningMethodHS256, jwt.MapClaims{
		"payload": payload,
	})
	signed, err := token.SignedString([]byte(secret))
	if err != nil {
		panic(err)
	}
	return signed
}

func TestParseOnlyOfficeCallbackTokenVerifiesAndMapsFields(t *testing.T) {
	orig := settings.Config.Integrations.OnlyOffice
	t.Cleanup(func() { settings.Config.Integrations.OnlyOffice = orig })

	const secret = "onlyoffice-test-secret"
	settings.Config.Integrations.OnlyOffice.Secret = secret

	docURL := "http://office.example/cache/files/doc/output.docx"
	token := signOnlyOfficeCallbackJWT(secret, map[string]interface{}{
		"key":    "doc-key-1",
		"status": 2,
		"url":    docURL,
		"users":  []string{"user-1"},
	})

	callback, err := parseOnlyOfficeCallbackToken(token)
	if err != nil {
		t.Fatalf("parseOnlyOfficeCallbackToken() error = %v", err)
	}
	if callback.Key != "doc-key-1" {
		t.Errorf("Key = %q, want doc-key-1", callback.Key)
	}
	if callback.Status != 2 {
		t.Errorf("Status = %d, want 2", callback.Status)
	}
	if callback.URL != docURL {
		t.Errorf("URL = %q, want %q", callback.URL, docURL)
	}
	if len(callback.Users) != 1 || callback.Users[0] != "user-1" {
		t.Errorf("Users = %v, want [user-1]", callback.Users)
	}
}

func TestParseOnlyOfficeCallbackTokenRejectsInvalidSignature(t *testing.T) {
	orig := settings.Config.Integrations.OnlyOffice
	t.Cleanup(func() { settings.Config.Integrations.OnlyOffice = orig })

	settings.Config.Integrations.OnlyOffice.Secret = "expected-secret"
	token := signOnlyOfficeCallbackJWT("other-secret", map[string]interface{}{
		"key":    "doc-key-1",
		"status": 2,
	})

	_, err := parseOnlyOfficeCallbackToken(token)
	if err == nil {
		t.Fatal("expected error for invalid signature")
	}
}

func TestParseOnlyOfficeCallbackFromJSONTokenBody(t *testing.T) {
	orig := settings.Config.Integrations.OnlyOffice
	t.Cleanup(func() { settings.Config.Integrations.OnlyOffice = orig })

	const secret = "onlyoffice-test-secret"
	settings.Config.Integrations.OnlyOffice.Secret = secret

	token := signOnlyOfficeCallbackJWT(secret, map[string]interface{}{
		"key":    "doc-key-2",
		"status": 1,
		"url":    "http://office.example/cache/save.docx",
	})
	body, err := json.Marshal(map[string]string{"token": token})
	if err != nil {
		t.Fatalf("marshal: %v", err)
	}

	req := httptest.NewRequest(http.MethodPost, "/api/office/callback", bytes.NewReader(body))
	callback, err := parseOnlyOfficeCallbackFromJSON(req)
	if err != nil {
		t.Fatalf("parseOnlyOfficeCallbackFromJSON() error = %v", err)
	}
	if callback.Key != "doc-key-2" || callback.Status != 1 {
		t.Fatalf("callback = %+v, want key doc-key-2 status 1", callback)
	}
	if callback.URL == "" {
		t.Fatal("expected URL from JWT payload")
	}
}

func TestParseOnlyOfficeCallbackFromJSONRejectsUnsignedWhenSecretSet(t *testing.T) {
	orig := settings.Config.Integrations.OnlyOffice
	t.Cleanup(func() { settings.Config.Integrations.OnlyOffice = orig })

	settings.Config.Integrations.OnlyOffice.Secret = "onlyoffice-test-secret"
	body := []byte(`{"key":"doc-key","status":2,"url":"http://office.example/x"}`)
	req := httptest.NewRequest(http.MethodPost, "/api/office/callback", bytes.NewReader(body))

	_, err := parseOnlyOfficeCallbackFromJSON(req)
	if err == nil {
		t.Fatal("expected error for unsigned callback body")
	}
	if !strings.Contains(err.Error(), "unsigned callback rejected") {
		t.Fatalf("error = %v, want unsigned callback rejection", err)
	}
}

func TestParseOnlyOfficeCallbackFromJSONPlainWhenNoSecret(t *testing.T) {
	orig := settings.Config.Integrations.OnlyOffice
	t.Cleanup(func() { settings.Config.Integrations.OnlyOffice = orig })

	settings.Config.Integrations.OnlyOffice.Secret = ""
	body := []byte(`{"key":"doc-key","status":2,"url":"http://office.example/x"}`)
	req := httptest.NewRequest(http.MethodPost, "/api/office/callback", bytes.NewReader(body))

	callback, err := parseOnlyOfficeCallbackFromJSON(req)
	if err != nil {
		t.Fatalf("parseOnlyOfficeCallbackFromJSON() error = %v", err)
	}
	if callback.Key != "doc-key" || callback.Status != 2 {
		t.Fatalf("callback = %+v", callback)
	}
}

func TestValidateOnlyOfficeCallbackKey(t *testing.T) {
	const realPath = "/data/docs/report.docx"
	const cacheKey = "oo-session-key"
	user := &users.User{FrontendUser: users.FrontendUser{Username: "alice"}}

	origFunc := files.FileInfoFasterFunc
	t.Cleanup(func() { files.FileInfoFasterFunc = origFunc })

	t.Run("match", func(t *testing.T) {
		files.FileInfoFasterFunc = func(utils.FileOptions, *users.User) (*iteminfo.ExtendedFileInfo, error) {
			return &iteminfo.ExtendedFileInfo{RealPath: realPath}, nil
		}
		utils.OnlyOfficeCache.Set(realPath, cacheKey)
		t.Cleanup(func() { utils.OnlyOfficeCache.Delete(realPath) })

		err := validateOnlyOfficeCallbackKey("source", "/report.docx", user, &OnlyOfficeCallback{Key: cacheKey})
		if err != nil {
			t.Fatalf("validateOnlyOfficeCallbackKey() error = %v", err)
		}
	})

	t.Run("mismatch", func(t *testing.T) {
		files.FileInfoFasterFunc = func(utils.FileOptions, *users.User) (*iteminfo.ExtendedFileInfo, error) {
			return &iteminfo.ExtendedFileInfo{RealPath: realPath}, nil
		}
		utils.OnlyOfficeCache.Set(realPath, cacheKey)
		t.Cleanup(func() { utils.OnlyOfficeCache.Delete(realPath) })

		err := validateOnlyOfficeCallbackKey("source", "/report.docx", user, &OnlyOfficeCallback{Key: "other-key"})
		if err == nil {
			t.Fatal("expected document key mismatch error")
		}
	})

	t.Run("cache miss", func(t *testing.T) {
		files.FileInfoFasterFunc = func(utils.FileOptions, *users.User) (*iteminfo.ExtendedFileInfo, error) {
			return &iteminfo.ExtendedFileInfo{RealPath: realPath}, nil
		}
		utils.OnlyOfficeCache.Delete(realPath)

		err := validateOnlyOfficeCallbackKey("source", "/report.docx", user, &OnlyOfficeCallback{Key: cacheKey})
		if err == nil {
			t.Fatal("expected error for cache miss")
		}
	})

	t.Run("file lookup failure", func(t *testing.T) {
		files.FileInfoFasterFunc = func(utils.FileOptions, *users.User) (*iteminfo.ExtendedFileInfo, error) {
			return nil, fmt.Errorf("not found")
		}

		err := validateOnlyOfficeCallbackKey("source", "/report.docx", user, &OnlyOfficeCallback{Key: cacheKey})
		if err == nil {
			t.Fatal("expected error when file lookup fails")
		}
	})
}
