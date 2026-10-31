package web

import (
	"bytes"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"github.com/gtsteffaniak/filebrowser/backend/internal/database/share"
	"github.com/gtsteffaniak/filebrowser/backend/internal/database/users"
	"github.com/gtsteffaniak/filebrowser/backend/internal/state"
	"github.com/gtsteffaniak/filebrowser/backend/pkg/settings"
)

func enableSharePolicy(t *testing.T) {
	t.Helper()
	orig := settings.Config.SharePolicy
	t.Cleanup(func() { settings.Config.SharePolicy = orig })
	settings.Config.SharePolicy = settings.SharePolicy{MaxExpirationHours: 168, RequirePassword: true}
}

func postShareBody(t *testing.T, user *users.User, body map[string]interface{}) (int, error) {
	t.Helper()
	raw, err := json.Marshal(body)
	if err != nil {
		t.Fatalf("marshal body: %v", err)
	}
	req := httptest.NewRequest(http.MethodPost, "/api/share", bytes.NewReader(raw))
	return sharePostHandler(httptest.NewRecorder(), req, &Context{User: user})
}

func createPolicyShare(t *testing.T, ownerID uint64, expire int64, passwordHash string) {
	t.Helper()
	err := state.CreateShare(&share.Share{
		ShareSettings: share.ShareSettings{
			FrontendShareInfo: share.FrontendShareInfo{ShareType: "normal"},
			ShareLimits:       share.ShareLimits{SourceName: "srv"},
		},
		ShareColumns: share.ShareColumns{Hash: victimShareHash, Path: "/", Expire: expire},
		SourcePath:   "/srv",
		UserID:       ownerID,
		PasswordHash: passwordHash,
		Version:      1,
	})
	if err != nil {
		t.Fatalf("CreateShare: %v", err)
	}
}

func wantPolicyRejection(t *testing.T, status int, err error, fragment string) {
	t.Helper()
	if status != http.StatusBadRequest || err == nil || !strings.Contains(err.Error(), fragment) {
		t.Fatalf("status=%d err=%v, want 400 mentioning %q", status, err, fragment)
	}
}

func inHours(h int) int64 { return time.Now().Add(time.Duration(h) * time.Hour).Unix() }

func TestSharePolicy_CreateRejections(t *testing.T) {
	owner, _, _ := setupShareAuthTestUsers(t)
	enableSharePolicy(t)
	base := map[string]interface{}{"path": "/", "source": "srv"}
	with := func(extra map[string]interface{}) map[string]interface{} {
		m := map[string]interface{}{}
		for k, v := range base {
			m[k] = v
		}
		for k, v := range extra {
			m[k] = v
		}
		return m
	}
	cases := []struct {
		name     string
		body     map[string]interface{}
		fragment string
	}{
		{"no password", with(map[string]interface{}{"expires": "7", "unit": "days"}), "password"},
		{"empty password", with(map[string]interface{}{"expires": "7", "unit": "days", "password": ""}), "password"},
		{"no expiry", with(map[string]interface{}{"password": "secret"}), "expire"},
		{"expiry over the limit", with(map[string]interface{}{"expires": "8", "unit": "days", "password": "secret"}), "expire"},
		{"limit reached through another unit", with(map[string]interface{}{"expires": "169", "unit": "hours", "password": "secret"}), "expire"},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			status, err := postShareBody(t, owner, c.body)
			wantPolicyRejection(t, status, err, c.fragment)
		})
	}
}

func TestSharePolicy_ValuesWithinLimitsPass(t *testing.T) {
	owner, _, _ := setupShareAuthTestUsers(t)
	enableSharePolicy(t)
	if err := checkSharePolicy(owner, inHours(168)-60, true); err != nil {
		t.Fatalf("a share within the policy was rejected: %v", err)
	}
}

func TestSharePolicy_HugeLimitDoesNotOverflow(t *testing.T) {
	owner, _, _ := setupShareAuthTestUsers(t)
	enableSharePolicy(t)
	settings.Config.SharePolicy.MaxExpirationHours = 3_000_000 // about 342 years, past the range of time.Duration
	if err := checkSharePolicy(owner, inHours(1), true); err != nil {
		t.Fatalf("a large limit rejected a short share: %v", err)
	}
}

func TestSharePolicy_UpdateCannotClearPassword(t *testing.T) {
	owner, _, _ := setupShareAuthTestUsers(t)
	enableSharePolicy(t)
	createPolicyShare(t, owner.ID, inHours(24), "hash")
	status, err := postShareBody(t, owner, map[string]interface{}{"hash": victimShareHash, "password": ""})
	wantPolicyRejection(t, status, err, "password")
	got, err := state.GetShare(victimShareHash)
	if err != nil {
		t.Fatalf("GetShare: %v", err)
	}
	if got.PasswordHash != "hash" {
		t.Fatal("the rejected update cleared the password")
	}
}

func TestSharePolicy_UpdateCannotExtendPastLimit(t *testing.T) {
	owner, _, _ := setupShareAuthTestUsers(t)
	enableSharePolicy(t)
	createPolicyShare(t, owner.ID, inHours(24), "hash")
	status, err := postShareBody(t, owner, map[string]interface{}{"hash": victimShareHash, "expires": "30", "unit": "days"})
	wantPolicyRejection(t, status, err, "expire")
}

func TestSharePolicy_UpdateKeepingCompliantValuesPasses(t *testing.T) {
	owner, _, _ := setupShareAuthTestUsers(t)
	enableSharePolicy(t)
	createPolicyShare(t, owner.ID, inHours(24), "hash")
	status, err := postShareBody(t, owner, map[string]interface{}{"hash": victimShareHash, "allowDelete": true})
	if status != http.StatusOK {
		t.Fatalf("status=%d err=%v, want 200", status, err)
	}
}

// A share created before the policy has to comply the next time it is edited.
func TestSharePolicy_UpdateRejectsNonCompliantOldShare(t *testing.T) {
	owner, _, _ := setupShareAuthTestUsers(t)
	enableSharePolicy(t)
	createPolicyShare(t, owner.ID, 0, "")
	status, err := postShareBody(t, owner, map[string]interface{}{"hash": victimShareHash, "allowDelete": true})
	wantPolicyRejection(t, status, err, "password")
}

func TestSharePolicy_AdminIsExempt(t *testing.T) {
	_, _, admin := setupShareAuthTestUsers(t)
	enableSharePolicy(t)
	if err := checkSharePolicy(admin, 0, false); err != nil {
		t.Fatalf("admin rejected: %v", err)
	}
}

func TestSharePolicy_DisabledByDefault(t *testing.T) {
	owner, _, _ := setupShareAuthTestUsers(t)
	orig := settings.Config.SharePolicy
	t.Cleanup(func() { settings.Config.SharePolicy = orig })
	settings.Config.SharePolicy = settings.SharePolicy{}
	if err := checkSharePolicy(owner, 0, false); err != nil {
		t.Fatalf("no policy configured, but rejected: %v", err)
	}
}
