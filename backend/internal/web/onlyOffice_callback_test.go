package web

import (
	"bytes"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"net/http/httptest"
	"strings"
	"sync"
	"testing"
	"time"

	jwt "github.com/golang-jwt/jwt/v4"
	"github.com/gtsteffaniak/filebrowser/backend/internal/adapters/fs/files"
	"github.com/gtsteffaniak/filebrowser/backend/internal/database/share"
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

func TestOnlyOfficeCallbackDeniedWhenShareDisablesOnlyOffice(t *testing.T) {
	initStreamTestSources(t)

	d := &requestContext{
		Share: share.Share{
			ShareColumns: share.ShareColumns{Hash: "abc123"},
			SourcePath:   "/srv",
			ShareSettings: share.ShareSettings{
				FrontendShareInfo: share.FrontendShareInfo{EnableOnlyOffice: false},
			},
		},
	}
	req := httptest.NewRequest(http.MethodPost, "/public/api/office/callback?hash=abc123", nil)
	rec := httptest.NewRecorder()
	if _, err := processOnlyOfficeCallback(rec, req, d, &OnlyOfficeCallback{Key: "k", Status: onlyOfficeStatusDocumentClosedWithNoChanges}); err != nil {
		t.Fatalf("processOnlyOfficeCallback returned err=%v", err)
	}
	if rec.Code != http.StatusForbidden {
		t.Fatalf("status = %d, want %d", rec.Code, http.StatusForbidden)
	}
}

func TestOnlyOfficeCallbackAllowedWhenShareEnablesOnlyOffice(t *testing.T) {
	initStreamTestSources(t)

	origSourceMap := settings.Config.Server.SourceMap
	origFileInfo := files.FileInfoFasterFunc
	t.Cleanup(func() {
		settings.Config.Server.SourceMap = origSourceMap
		files.FileInfoFasterFunc = origFileInfo
	})
	settings.Config.Server.SourceMap = map[string]*settings.Source{
		"/srv": {Path: "/srv", Name: "srv"},
	}

	const realPath = "/srv/docs/doc.docx"
	const docKey = "oo-share-key"
	files.FileInfoFasterFunc = func(utils.FileOptions, *users.User) (*iteminfo.ExtendedFileInfo, error) {
		return &iteminfo.ExtendedFileInfo{RealPath: realPath}, nil
	}
	utils.OnlyOfficeCache.Set(realPath, docKey)
	t.Cleanup(func() { utils.OnlyOfficeCache.Delete(realPath) })

	d := &requestContext{
		Share: share.Share{
			ShareColumns: share.ShareColumns{Hash: "abc123"},
			SourcePath:   "/srv",
			ShareSettings: share.ShareSettings{
				FrontendShareInfo: share.FrontendShareInfo{EnableOnlyOffice: true},
			},
		},
		ShareUser: testUserWithView(1, "srv"),
		IndexPath: "/docs/doc.docx",
	}
	req := httptest.NewRequest(http.MethodPost, "/public/api/office/callback?hash=abc123", nil)
	rec := httptest.NewRecorder()
	if _, err := processOnlyOfficeCallback(rec, req, d, &OnlyOfficeCallback{Key: docKey, Status: onlyOfficeStatusDocumentClosedWithNoChanges}); err != nil {
		t.Fatalf("processOnlyOfficeCallback returned err=%v", err)
	}
	if rec.Code != http.StatusOK {
		t.Fatalf("status = %d, want %d", rec.Code, http.StatusOK)
	}
}

type roundTripFunc func(*http.Request) (*http.Response, error)

func (f roundTripFunc) RoundTrip(r *http.Request) (*http.Response, error) { return f(r) }

func TestOnlyOfficeCallbackRetainsKeyWhenSaveFails(t *testing.T) {
	initStreamTestSources(t)

	const realPath = "/default/doc.docx"
	const docKey = "oo-key-save-fail"

	origOnlyOffice := settings.Config.Integrations.OnlyOffice
	origFileInfo := files.FileInfoFasterFunc
	origWriteFile := files.WriteFileFunc
	origClient := onlyOfficeDownloadClient
	t.Cleanup(func() {
		settings.Config.Integrations.OnlyOffice = origOnlyOffice
		files.FileInfoFasterFunc = origFileInfo
		files.WriteFileFunc = origWriteFile
		onlyOfficeDownloadClient = origClient
		utils.OnlyOfficeCache.Delete(realPath)
	})

	settings.Config.Integrations.OnlyOffice.Url = "http://onlyoffice.test"

	files.FileInfoFasterFunc = func(utils.FileOptions, *users.User) (*iteminfo.ExtendedFileInfo, error) {
		return &iteminfo.ExtendedFileInfo{RealPath: realPath}, nil
	}
	onlyOfficeDownloadClient = &http.Client{Transport: roundTripFunc(func(*http.Request) (*http.Response, error) {
		return &http.Response{
			StatusCode: http.StatusOK,
			Header:     make(http.Header),
			Body:       io.NopCloser(strings.NewReader("updated-document")),
		}, nil
	})}
	utils.OnlyOfficeCache.Set(realPath, docKey)

	writeCalls := 0
	files.WriteFileFunc = func(string, string, io.Reader) error {
		writeCalls++
		return errors.New("write failed")
	}

	user := testUserWithSourcePerms("/default", users.SourceFilePermissions{
		View: true, Download: true, Modify: true,
	})
	d := &requestContext{User: user}
	callback := &OnlyOfficeCallback{
		Key:    docKey,
		Status: onlyOfficeStatusDocumentClosedWithChanges,
		URL:    "http://onlyoffice.test/cache/doc.docx",
	}
	req := httptest.NewRequest(http.MethodPost, "/api/office/callback?source=default&path=/doc.docx", nil)

	rec := httptest.NewRecorder()
	if _, err := processOnlyOfficeCallback(rec, req, d, callback); err != nil {
		t.Fatalf("processOnlyOfficeCallback returned err=%v", err)
	}
	if rec.Code != http.StatusInternalServerError {
		t.Fatalf("expected save failure, got status=%d", rec.Code)
	}
	if _, err := GetOnlyOfficeId(realPath); err != nil {
		t.Fatal("document key was deleted despite failed save; document server retry would be rejected")
	}

	// The document server retries the callback with the same key; the session
	// must still resolve and the save must be allowed to complete.
	files.WriteFileFunc = func(string, string, io.Reader) error {
		writeCalls++
		return nil
	}
	rec = httptest.NewRecorder()
	if _, err := processOnlyOfficeCallback(rec, req, d, callback); err != nil {
		t.Fatalf("retry failed: %v", err)
	}
	if rec.Code != http.StatusOK {
		t.Fatalf("retry failed: status=%d", rec.Code)
	}
	if _, err := GetOnlyOfficeId(realPath); err == nil {
		t.Fatal("document key should be deleted after a successful save")
	}
	if writeCalls != 2 {
		t.Fatalf("writeCalls = %d, want 2", writeCalls)
	}
}

// Concurrent save callbacks for the same document must not overlap: WriteFile
// opens the destination with O_TRUNC, so a second callback racing the first
// could corrupt the in-flight write.
func TestOnlyOfficeCallbackSerializesConcurrentSaves(t *testing.T) {
	initStreamTestSources(t)

	const realPath = "/default/doc.docx"
	const docKey = "oo-key-serialized-save"

	origOnlyOffice := settings.Config.Integrations.OnlyOffice
	origFileInfo := files.FileInfoFasterFunc
	origWriteFile := files.WriteFileFunc
	origClient := onlyOfficeDownloadClient
	t.Cleanup(func() {
		settings.Config.Integrations.OnlyOffice = origOnlyOffice
		files.FileInfoFasterFunc = origFileInfo
		files.WriteFileFunc = origWriteFile
		onlyOfficeDownloadClient = origClient
		utils.OnlyOfficeCache.Delete(realPath)
	})

	settings.Config.Integrations.OnlyOffice.Url = "http://onlyoffice.test"

	files.FileInfoFasterFunc = func(utils.FileOptions, *users.User) (*iteminfo.ExtendedFileInfo, error) {
		return &iteminfo.ExtendedFileInfo{RealPath: realPath}, nil
	}
	onlyOfficeDownloadClient = &http.Client{Transport: roundTripFunc(func(*http.Request) (*http.Response, error) {
		return &http.Response{
			StatusCode: http.StatusOK,
			Header:     make(http.Header),
			Body:       io.NopCloser(strings.NewReader("updated-document")),
		}, nil
	})}
	utils.OnlyOfficeCache.Set(realPath, docKey)

	var writesMu sync.Mutex
	writeCalls, active, maxActive := 0, 0, 0
	writeStarted := make(chan struct{}, 2)
	writeRelease := make(chan struct{})
	files.WriteFileFunc = func(string, string, io.Reader) error {
		writesMu.Lock()
		writeCalls++
		active++
		if active > maxActive {
			maxActive = active
		}
		writesMu.Unlock()
		writeStarted <- struct{}{}
		<-writeRelease
		writesMu.Lock()
		active--
		writesMu.Unlock()
		return nil
	}

	user := testUserWithSourcePerms("/default", users.SourceFilePermissions{
		View: true, Download: true, Modify: true,
	})
	callback := &OnlyOfficeCallback{
		Key:    docKey,
		Status: onlyOfficeStatusDocumentClosedWithChanges,
		URL:    "http://onlyoffice.test/doc.docx",
	}
	call := func() error {
		_, err := processOnlyOfficeCallback(httptest.NewRecorder(),
			httptest.NewRequest(http.MethodPost, "/api/office/callback?source=default&path=/doc.docx", nil),
			&requestContext{User: user}, callback)
		return err
	}

	firstDone := make(chan error, 1)
	go func() { firstDone <- call() }()
	<-writeStarted // first callback is inside WriteFile, holding the doc lock

	secondDone := make(chan error, 1)
	go func() { secondDone <- call() }()

	// Let the second callback validate the (still cached) key and block on the
	// document lock before the first write completes.
	time.Sleep(100 * time.Millisecond)
	close(writeRelease)

	if err := <-firstDone; err != nil {
		t.Fatalf("first callback: %v", err)
	}
	if err := <-secondDone; err != nil {
		t.Fatalf("second callback: %v", err)
	}

	writesMu.Lock()
	defer writesMu.Unlock()
	if writeCalls != 2 {
		t.Fatalf("expected both callbacks to write sequentially, got %d writes", writeCalls)
	}
	if maxActive != 1 {
		t.Fatalf("writes overlapped: maxActive=%d", maxActive)
	}
}

func TestOnlyOfficeCallbackDeletesKeyWhenClosedWithoutChanges(t *testing.T) {
	initStreamTestSources(t)

	const realPath = "/default/doc.docx"
	const docKey = "oo-key-no-changes"

	origFileInfo := files.FileInfoFasterFunc
	t.Cleanup(func() {
		files.FileInfoFasterFunc = origFileInfo
		utils.OnlyOfficeCache.Delete(realPath)
	})

	files.FileInfoFasterFunc = func(utils.FileOptions, *users.User) (*iteminfo.ExtendedFileInfo, error) {
		return &iteminfo.ExtendedFileInfo{RealPath: realPath}, nil
	}
	utils.OnlyOfficeCache.Set(realPath, docKey)

	d := &requestContext{User: testUserWithSourcePerms("/default", users.SourceFilePermissions{
		View: true, Download: true, Modify: true,
	})}
	req := httptest.NewRequest(http.MethodPost, "/api/office/callback?source=default&path=/doc.docx", nil)

	status, err := processOnlyOfficeCallback(httptest.NewRecorder(), req, d, &OnlyOfficeCallback{
		Key:    docKey,
		Status: onlyOfficeStatusDocumentClosedWithNoChanges,
	})
	if err != nil || status != http.StatusOK {
		t.Fatalf("status=%d err=%v", status, err)
	}
	if _, err := GetOnlyOfficeId(realPath); err == nil {
		t.Fatal("document key should be deleted when the document closed without changes")
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
