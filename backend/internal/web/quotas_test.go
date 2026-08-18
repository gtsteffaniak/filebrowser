package web

import (
	"bytes"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"path/filepath"
	"testing"

	activityrec "github.com/gtsteffaniak/filebrowser/backend/internal/activity"
	"github.com/gtsteffaniak/filebrowser/backend/internal/app"
	activitydb "github.com/gtsteffaniak/filebrowser/backend/internal/database/activity"
	"github.com/gtsteffaniak/filebrowser/backend/internal/database/users"
	"github.com/gtsteffaniak/filebrowser/backend/internal/state"
	"github.com/gtsteffaniak/filebrowser/backend/pkg/indexing"
	"github.com/gtsteffaniak/filebrowser/backend/pkg/settings"
)

func setupQuotaHTTPTest(t *testing.T) {
	t.Helper()
	dbPath := filepath.Join(t.TempDir(), "quota-http-test.sqlite")
	if _, err := state.Initialize(dbPath); err != nil {
		t.Fatal(err)
	}
	app.MustWireServices(state.Default())
	t.Cleanup(func() {
		state.Close()
	})

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
	adminUser.BackendScopes = []users.BackendScope{
		{Path: "/downloads", Scope: "/"},
	}
	if err := state.UpdateUser(adminUser, "", "backendScopes"); err != nil {
		t.Fatal(err)
	}

	indexing.SetTestIndex("Downloads", "/downloads")
	t.Cleanup(func() {
		indexing.ClearTestIndices()
	})
}

func TestQuotasPostHandler_RecordsActivityDetails(t *testing.T) {
	setupQuotaHTTPTest(t)

	adminUser, err := state.GetUserByUsername("admin")
	if err != nil {
		t.Fatal(err)
	}
	ctx := &requestContext{User: &adminUser}

	body, err := json.Marshal(map[string]any{
		"source":     "Downloads",
		"path":       "/projects",
		"limitBytes": 1073741824,
		"meter":      "accounted",
	})
	if err != nil {
		t.Fatal(err)
	}

	req := httptest.NewRequest(http.MethodPost, "/api/quotas", bytes.NewReader(body))
	req.Header.Set("Content-Type", "application/json")
	req.RemoteAddr = "127.0.0.1:1234"
	rec := httptest.NewRecorder()

	status, handlerErr := quotasPostHandler(rec, req, ctx)
	if status != http.StatusOK || handlerErr != nil {
		t.Fatalf("post handler failed: status=%d err=%v body=%s", status, handlerErr, rec.Body.String())
	}

	activityrec.FlushNow()
	rows, total, err := state.ListActivity(activitydb.QueryFilter{
		EventTypes: []activitydb.EventType{activitydb.EventQuotaCreate},
		Limit:      10,
	})
	if err != nil {
		t.Fatal(err)
	}
	if total != 1 || len(rows) != 1 {
		t.Fatalf("expected 1 quota create activity row, got total=%d len=%d", total, len(rows))
	}
	if rows[0].Source != "Downloads" || rows[0].Path != "/projects" {
		t.Fatalf("unexpected activity source/path: %q %q", rows[0].Source, rows[0].Path)
	}
	assertActivityChange(t, rows[0].Details.Changes, "limitBytes", "1073741824")
	assertActivityChange(t, rows[0].Details.Changes, "meter", "accounted")
}

func TestQuotasDeleteHandler_RecordsActivityDetails(t *testing.T) {
	setupQuotaHTTPTest(t)

	adminUser, err := state.GetUserByUsername("admin")
	if err != nil {
		t.Fatal(err)
	}
	ctx := &requestContext{User: &adminUser}

	createBody, err := json.Marshal(map[string]any{
		"source":     "Downloads",
		"path":       "/archive",
		"limitBytes": 5368709120,
		"meter":      "accounted",
	})
	if err != nil {
		t.Fatal(err)
	}
	createReq := httptest.NewRequest(http.MethodPost, "/api/quotas", bytes.NewReader(createBody))
	createReq.Header.Set("Content-Type", "application/json")
	createReq.RemoteAddr = "127.0.0.1:1234"
	createRec := httptest.NewRecorder()
	if status, handlerErr := quotasPostHandler(createRec, createReq, ctx); status != http.StatusOK || handlerErr != nil {
		t.Fatalf("setup post failed: status=%d err=%v", status, handlerErr)
	}

	var createdList []map[string]any
	if unmarshalErr := json.Unmarshal(createRec.Body.Bytes(), &createdList); unmarshalErr != nil {
		t.Fatal(unmarshalErr)
	}
	if len(createdList) == 0 || createdList[0]["limitBytes"] == nil {
		t.Fatal("expected limitBytes in create response")
	}

	delReq := httptest.NewRequest(http.MethodDelete, "/api/quotas?source=Downloads&path=/archive", nil)
	delReq.RemoteAddr = "127.0.0.1:1234"
	delRec := httptest.NewRecorder()
	status, handlerErr := quotasDeleteHandler(delRec, delReq, ctx)
	if status != http.StatusNoContent || handlerErr != nil {
		t.Fatalf("delete handler failed: status=%d err=%v body=%s", status, handlerErr, delRec.Body.String())
	}

	activityrec.FlushNow()
	rows, total, err := state.ListActivity(activitydb.QueryFilter{
		EventTypes: []activitydb.EventType{activitydb.EventQuotaDelete},
		Limit:      10,
	})
	if err != nil {
		t.Fatal(err)
	}
	if total != 1 || len(rows) != 1 {
		t.Fatalf("expected 1 quota delete activity row, got total=%d len=%d", total, len(rows))
	}
	assertActivityChange(t, rows[0].Details.Changes, "limitBytes", "5368709120")
	assertActivityChange(t, rows[0].Details.Changes, "meter", "accounted")
}

func TestQuotasPatchHandler_UpdatesLimitWithoutRebindingOwner(t *testing.T) {
	setupQuotaHTTPTest(t)

	bob := &users.User{
		ID: 2,
		FrontendUser: users.FrontendUser{
			Username: "bob",
		},
	}
	if err := state.CreateUser(bob, ""); err != nil {
		t.Fatal(err)
	}
	bob.BackendScopes = []users.BackendScope{{Path: "/downloads", Scope: "/"}}
	if err := state.UpdateUser(bob, "", "backendScopes"); err != nil {
		t.Fatal(err)
	}

	adminUser, err := state.GetUserByUsername("admin")
	if err != nil {
		t.Fatal(err)
	}
	ctx := &requestContext{User: &adminUser}

	createBody, err := json.Marshal(map[string]any{
		"source":     "Downloads",
		"path":       "/bob-quota",
		"username":   "bob",
		"limitBytes": 1073741824,
		"meter":      "accounted",
	})
	if err != nil {
		t.Fatal(err)
	}
	createReq := httptest.NewRequest(http.MethodPost, "/api/quotas", bytes.NewReader(createBody))
	createReq.Header.Set("Content-Type", "application/json")
	createRec := httptest.NewRecorder()
	if status, handlerErr := quotasPostHandler(createRec, createReq, ctx); status != http.StatusOK || handlerErr != nil {
		t.Fatalf("setup post failed: status=%d err=%v", status, handlerErr)
	}

	patchBody, err := json.Marshal(map[string]any{
		"source":     "Downloads",
		"path":       "/bob-quota",
		"limitBytes": 2147483648,
	})
	if err != nil {
		t.Fatal(err)
	}
	patchReq := httptest.NewRequest(http.MethodPatch, "/api/quotas?username=bob", bytes.NewReader(patchBody))
	patchReq.Header.Set("Content-Type", "application/json")
	patchRec := httptest.NewRecorder()
	status, handlerErr := quotasPatchHandler(patchRec, patchReq, ctx)
	if status != http.StatusOK || handlerErr != nil {
		t.Fatalf("patch failed: status=%d err=%v body=%s", status, handlerErr, patchRec.Body.String())
	}

	var out []folderQuotaResponse
	if unmarshalErr := json.Unmarshal(patchRec.Body.Bytes(), &out); unmarshalErr != nil {
		t.Fatal(unmarshalErr)
	}
	if len(out) != 1 || out[0].LimitBytes != 2147483648 {
		t.Fatalf("unexpected patch response: %+v", out)
	}

	q, err := state.GetFolderQuotaByPathAndUser("/downloads", "/bob-quota", bob.ID)
	if err != nil {
		t.Fatal(err)
	}
	if q.UserID != bob.ID {
		t.Fatalf("owner rebinding: got userID=%d want %d", q.UserID, bob.ID)
	}
	if q.LimitBytes != 2147483648 {
		t.Fatalf("limit not updated: got %d", q.LimitBytes)
	}
}

func TestQuotasPatchHandler_RebindsOwnerFromBody(t *testing.T) {
	setupQuotaHTTPTest(t)

	bob := &users.User{
		ID: 2,
		FrontendUser: users.FrontendUser{
			Username: "bob",
		},
	}
	if err := state.CreateUser(bob, ""); err != nil {
		t.Fatal(err)
	}

	adminUser, err := state.GetUserByUsername("admin")
	if err != nil {
		t.Fatal(err)
	}
	ctx := &requestContext{User: &adminUser}

	createBody, err := json.Marshal(map[string]any{
		"source":     "Downloads",
		"path":       "/shared-quota",
		"limitBytes": 1073741824,
		"meter":      "accounted",
	})
	if err != nil {
		t.Fatal(err)
	}
	createReq := httptest.NewRequest(http.MethodPost, "/api/quotas", bytes.NewReader(createBody))
	createReq.Header.Set("Content-Type", "application/json")
	createRec := httptest.NewRecorder()
	if status, handlerErr := quotasPostHandler(createRec, createReq, ctx); status != http.StatusOK || handlerErr != nil {
		t.Fatalf("setup post failed: status=%d err=%v", status, handlerErr)
	}

	patchBody, err := json.Marshal(map[string]any{
		"source":   "Downloads",
		"path":     "/shared-quota",
		"username": "bob",
	})
	if err != nil {
		t.Fatal(err)
	}
	patchReq := httptest.NewRequest(http.MethodPatch, "/api/quotas", bytes.NewReader(patchBody))
	patchReq.Header.Set("Content-Type", "application/json")
	patchRec := httptest.NewRecorder()
	status, handlerErr := quotasPatchHandler(patchRec, patchReq, ctx)
	if status != http.StatusOK || handlerErr != nil {
		t.Fatalf("patch failed: status=%d err=%v body=%s", status, handlerErr, patchRec.Body.String())
	}

	q, err := state.GetFolderQuotaByPathAndUser("/downloads", "/shared-quota", bob.ID)
	if err != nil {
		t.Fatal(err)
	}
	if q.UserID != bob.ID {
		t.Fatalf("owner not rebound: got userID=%d want %d", q.UserID, bob.ID)
	}
}
