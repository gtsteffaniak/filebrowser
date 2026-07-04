package web

import (
	"bytes"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"path/filepath"
	"testing"

	activityrec "github.com/gtsteffaniak/filebrowser/backend/internal/activity"
	activitydb "github.com/gtsteffaniak/filebrowser/backend/internal/database/activity"
	"github.com/gtsteffaniak/filebrowser/backend/internal/database/users"
	"github.com/gtsteffaniak/filebrowser/backend/internal/state"
	"github.com/gtsteffaniak/filebrowser/backend/pkg/indexing"
	"github.com/gtsteffaniak/filebrowser/backend/pkg/settings"
)

func setupAccessHTTPTest(t *testing.T) {
	t.Helper()
	dbPath := filepath.Join(t.TempDir(), "access-http-test.sqlite")
	if _, err := state.Initialize(dbPath); err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() {
		state.Close()
	})

	accessStore = state.GetAccessStorage()
	usersStore = state.GetUsersStorage()

	settings.Config.Server.SourceMap = map[string]*settings.Source{
		"/downloads": {
			Path: "/downloads",
			Name: "Downloads",
		},
	}
	settings.Config.Server.NameToSource = map[string]*settings.Source{
		"Downloads": settings.Config.Server.SourceMap["/downloads"],
	}
	settings.InitializeUserResolvers()
	config = &settings.Config

	adminUser := &users.User{
		ID: 1,
		FrontendUser: users.FrontendUser{
			Username:    "admin",
			Permissions: users.Permissions{Admin: true},
		},
	}
	if err := state.CreateUser(adminUser, ""); err != nil {
		t.Fatal(err)
	}
	adminUser.Permissions = users.Permissions{Admin: true}
	if err := state.UpdateUser(adminUser, "", "permissions"); err != nil {
		t.Fatal(err)
	}

	indexing.SetTestIndex("Downloads", "/downloads")
	t.Cleanup(func() {
		indexing.ClearTestIndices()
	})
}

func TestAccessPostHandler_RejectsNonExistentUser(t *testing.T) {
	setupAccessHTTPTest(t)

	body, err := json.Marshal(map[string]any{
		"allow":        false,
		"ruleCategory": "user",
		"value":        "test",
	})
	if err != nil {
		t.Fatal(err)
	}

	req := httptest.NewRequest(http.MethodPost, "/api/access?source=Downloads&path=%2F", bytes.NewReader(body))
	req.Header.Set("Content-Type", "application/json")
	rec := httptest.NewRecorder()

	adminUser, err := state.GetUserByUsername("admin")
	if err != nil {
		t.Fatal(err)
	}

	status, handlerErr := accessPostHandler(rec, req, &requestContext{User: &adminUser})
	if status != http.StatusBadRequest {
		t.Fatalf("expected status 400, got %d (err=%v body=%s)", status, handlerErr, rec.Body.String())
	}
	if handlerErr == nil || handlerErr.Error() != "user not found: test" {
		t.Fatalf("expected user not found error, got: %v", handlerErr)
	}
}

func TestAccessPostHandler_RecordsActivityDetails(t *testing.T) {
	setupAccessHTTPTest(t)

	adminUser, err := state.GetUserByUsername("admin")
	if err != nil {
		t.Fatal(err)
	}
	ctx := &requestContext{User: &adminUser}

	body, err := json.Marshal(map[string]any{
		"allow":        false,
		"ruleCategory": "user",
		"value":        "admin",
	})
	if err != nil {
		t.Fatal(err)
	}

	req := httptest.NewRequest(http.MethodPost, "/api/access?source=Downloads&path=%2F", bytes.NewReader(body))
	req.Header.Set("Content-Type", "application/json")
	req.RemoteAddr = "127.0.0.1:1234"
	rec := httptest.NewRecorder()

	status, handlerErr := accessPostHandler(rec, req, ctx)
	if status != http.StatusOK || handlerErr != nil {
		t.Fatalf("post handler failed: status=%d err=%v body=%s", status, handlerErr, rec.Body.String())
	}

	activityrec.FlushNow()
	rows, total, err := state.ListActivity(activitydb.QueryFilter{
		EventTypes: []activitydb.EventType{activitydb.EventAccessCreate},
		Limit:      10,
	})
	if err != nil {
		t.Fatal(err)
	}
	if total != 1 || len(rows) != 1 {
		t.Fatalf("expected 1 access create activity row, got total=%d len=%d", total, len(rows))
	}
	if rows[0].Details.Changes == nil || len(rows[0].Details.Changes) != 3 {
		t.Fatalf("expected 3 activity detail changes, got %#v", rows[0].Details.Changes)
	}
	assertActivityChange(t, rows[0].Details.Changes, "ruleType", "deny")
	assertActivityChange(t, rows[0].Details.Changes, "ruleCategory", "user")
	assertActivityChange(t, rows[0].Details.Changes, "value", "admin")
}

func TestAccessDeleteHandler_RecordsActivityDetails(t *testing.T) {
	setupAccessHTTPTest(t)

	adminUser, err := state.GetUserByUsername("admin")
	if err != nil {
		t.Fatal(err)
	}
	ctx := &requestContext{User: &adminUser}

	addBody, err := json.Marshal(map[string]any{
		"allow":        false,
		"ruleCategory": "user",
		"value":        "admin",
	})
	if err != nil {
		t.Fatal(err)
	}
	addReq := httptest.NewRequest(http.MethodPost, "/api/access?source=Downloads&path=%2F", bytes.NewReader(addBody))
	addReq.Header.Set("Content-Type", "application/json")
	addReq.RemoteAddr = "127.0.0.1:1234"
	addRec := httptest.NewRecorder()
	if status, handlerErr := accessPostHandler(addRec, addReq, ctx); status != http.StatusOK || handlerErr != nil {
		t.Fatalf("setup post failed: status=%d err=%v", status, handlerErr)
	}

	delReq := httptest.NewRequest(http.MethodDelete, "/api/access?source=Downloads&path=%2F&ruleType=deny&ruleCategory=user&value=admin", nil)
	delReq.RemoteAddr = "127.0.0.1:1234"
	delRec := httptest.NewRecorder()
	status, handlerErr := accessDeleteHandler(delRec, delReq, ctx)
	if status != http.StatusOK || handlerErr != nil {
		t.Fatalf("delete handler failed: status=%d err=%v body=%s", status, handlerErr, delRec.Body.String())
	}

	activityrec.FlushNow()
	rows, total, err := state.ListActivity(activitydb.QueryFilter{
		EventTypes: []activitydb.EventType{activitydb.EventAccessDelete},
		Limit:      10,
	})
	if err != nil {
		t.Fatal(err)
	}
	if total != 1 || len(rows) != 1 {
		t.Fatalf("expected 1 access delete activity row, got total=%d len=%d", total, len(rows))
	}
	if rows[0].Details.Changes == nil {
		t.Fatal("expected delete activity details changes")
	}
	assertActivityChange(t, rows[0].Details.Changes, "ruleType", "deny")
	assertActivityChange(t, rows[0].Details.Changes, "ruleCategory", "user")
	assertActivityChange(t, rows[0].Details.Changes, "value", "admin")
}

func assertActivityChange(t *testing.T, changes []activitydb.FieldChange, field, wantTo string) {
	t.Helper()
	for _, change := range changes {
		if change.Field == field {
			if change.To != wantTo {
				t.Fatalf("change %q: got to=%q want %q", field, change.To, wantTo)
			}
			return
		}
	}
	t.Fatalf("missing change field %q in %#v", field, changes)
}
