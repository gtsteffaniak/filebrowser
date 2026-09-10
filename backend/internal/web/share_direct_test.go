package web

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"golang.org/x/crypto/bcrypt"

	"github.com/gtsteffaniak/filebrowser/backend/internal/database/share"
	"github.com/gtsteffaniak/filebrowser/backend/internal/state"
)

const directShareHash = "direct_share_hash_test"

func createPasswordProtectedShare(t *testing.T, ownerID uint64, hash string) {
	t.Helper()
	passwordHash, err := bcrypt.GenerateFromPassword([]byte("secret"), bcrypt.DefaultCost)
	if err != nil {
		t.Fatalf("bcrypt: %v", err)
	}
	link := &share.Share{
		ShareSettings: share.ShareSettings{
			FrontendShareInfo: share.FrontendShareInfo{ShareType: "normal"},
			ShareLimits:       share.ShareLimits{SourceName: "srv"},
		},
		ShareColumns: share.ShareColumns{
			Hash: hash,
			Path: "/",
		},
		PasswordHash: string(passwordHash),
		SourcePath: "/srv",
		UserID:     ownerID,
		Version:    1,
	}
	if err := state.CreateShare(link); err != nil {
		t.Fatalf("CreateShare: %v", err)
	}
}

func TestShareDirectDownloadHandler_OwnerMintsToken(t *testing.T) {
	owner, attacker, admin := setupShareAuthTestUsers(t)
	createPasswordProtectedShare(t, owner.ID, directShareHash)

	req := httptest.NewRequest(http.MethodGet, "/api/share/direct?hash="+directShareHash, nil)
	req.Header.Set("X-SHARE-PASSWORD", "secret")
	rec := httptest.NewRecorder()
	status, err := shareDirectDownloadHandler(rec, req, &Context{User: owner})
	if status != http.StatusOK {
		t.Fatalf("expected 200, got status=%d err=%v", status, err)
	}

	var resp DirectDownloadResponse
	if err := json.Unmarshal(rec.Body.Bytes(), &resp); err != nil {
		t.Fatalf("unmarshal: %v", err)
	}
	if resp.Hash != directShareHash || resp.Token == "" || resp.URL == "" || resp.ExpiresAt == 0 {
		t.Fatalf("unexpected response: %+v", resp)
	}
	if resp.ExpiresAt <= time.Now().Unix() {
		t.Fatalf("expiresAt should be in the future: %d", resp.ExpiresAt)
	}

	reqAttacker := httptest.NewRequest(http.MethodGet, "/api/share/direct?hash="+directShareHash, nil)
	reqAttacker.Header.Set("X-SHARE-PASSWORD", "secret")
	recAttacker := httptest.NewRecorder()
	status, err = shareDirectDownloadHandler(recAttacker, reqAttacker, &Context{User: attacker})
	if status != http.StatusForbidden {
		t.Fatalf("expected 403 for non-owner, got status=%d err=%v", status, err)
	}

	reqAdmin := httptest.NewRequest(http.MethodGet, "/api/share/direct?hash="+directShareHash, nil)
	reqAdmin.Header.Set("X-SHARE-PASSWORD", "secret")
	recAdmin := httptest.NewRecorder()
	status, err = shareDirectDownloadHandler(recAdmin, reqAdmin, &Context{User: admin})
	if status != http.StatusOK {
		t.Fatalf("expected 200 for admin, got status=%d err=%v", status, err)
	}
}

func TestShareDirectDownloadHandler_RequiresSharePassword(t *testing.T) {
	owner, _, _ := setupShareAuthTestUsers(t)
	createPasswordProtectedShare(t, owner.ID, directShareHash+"pw")

	req := httptest.NewRequest(http.MethodGet, "/api/share/direct?hash="+directShareHash+"pw", nil)
	rec := httptest.NewRecorder()
	status, err := shareDirectDownloadHandler(rec, req, &Context{User: owner})
	if status != http.StatusUnauthorized {
		t.Fatalf("expected 401 without password header, got status=%d err=%v", status, err)
	}

	reqWrong := httptest.NewRequest(http.MethodGet, "/api/share/direct?hash="+directShareHash+"pw", nil)
	reqWrong.Header.Set("X-SHARE-PASSWORD", "wrong")
	recWrong := httptest.NewRecorder()
	status, err = shareDirectDownloadHandler(recWrong, reqWrong, &Context{User: owner})
	if status != http.StatusUnauthorized {
		t.Fatalf("expected 401 for wrong password, got status=%d err=%v", status, err)
	}
}

func TestShareDirectDownloadHandler_RejectsOver24Hours(t *testing.T) {
	owner, _, _ := setupShareAuthTestUsers(t)
	createPasswordProtectedShare(t, owner.ID, directShareHash+"24")

	req := httptest.NewRequest(http.MethodGet, "/api/share/direct?hash="+directShareHash+"24&duration=25&unit=hours", nil)
	req.Header.Set("X-SHARE-PASSWORD", "secret")
	rec := httptest.NewRecorder()
	status, err := shareDirectDownloadHandler(rec, req, &Context{User: owner})
	if status != http.StatusBadRequest {
		t.Fatalf("expected 400, got status=%d err=%v", status, err)
	}
}

func TestShareDirectDownloadHandler_RequiresPasswordProtectedShare(t *testing.T) {
	owner, _, _ := setupShareAuthTestUsers(t)
	createVictimShare(t, owner.ID)

	req := httptest.NewRequest(http.MethodGet, "/api/share/direct?hash="+victimShareHash, nil)
	req.Header.Set("X-SHARE-PASSWORD", "secret")
	rec := httptest.NewRecorder()
	status, err := shareDirectDownloadHandler(rec, req, &Context{User: owner})
	if status != http.StatusBadRequest {
		t.Fatalf("expected 400 for non-password share, got status=%d err=%v", status, err)
	}
}
