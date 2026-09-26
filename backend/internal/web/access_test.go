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

func setupAccessHTTPTest(t *testing.T) {
	t.Helper()
	dbPath := filepath.Join(t.TempDir(), "access-http-test.sqlite")
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

// groupCtx builds a request context for the named user; admin controls the Admin permission.
func groupCtx(t *testing.T, username string, admin bool) *requestContext {
	t.Helper()
	if admin {
		u, err := state.GetUserByUsername(username)
		if err != nil {
			t.Fatal(err)
		}
		return &requestContext{User: &u}
	}
	return &requestContext{User: &users.User{FrontendUser: users.FrontendUser{Username: username}}}
}

// doGroupRequest runs handler with a JSON or empty body and returns the status, recorder and handler error.
func doGroupRequest(t *testing.T, handler func(http.ResponseWriter, *http.Request, *requestContext) (int, error), method, target, body string, ctx *requestContext) (int, *httptest.ResponseRecorder, error) {
	t.Helper()
	req := httptest.NewRequest(method, target, bytes.NewReader([]byte(body)))
	req.Header.Set("Content-Type", "application/json")
	rec := httptest.NewRecorder()
	status, err := handler(rec, req, ctx)
	return status, rec, err
}

// assertJSONMessage checks the response body is valid JSON carrying the given message.
func assertJSONMessage(t *testing.T, rec *httptest.ResponseRecorder, want string) {
	t.Helper()
	var got map[string]string
	if err := json.Unmarshal(rec.Body.Bytes(), &got); err != nil {
		t.Fatalf("response body is not JSON (%q): %v", rec.Body.String(), err)
	}
	if got["message"] != want {
		t.Fatalf("expected message %q, got %q", want, got["message"])
	}
}

func TestGroupPutHandler_RequiresAdmin(t *testing.T) {
	setupAccessHTTPTest(t)
	status, _, _ := doGroupRequest(t, groupPutHandler, http.MethodPut, "/api/access/group", `{"group":"g","members":[]}`, groupCtx(t, "bob", false))
	if status != http.StatusForbidden {
		t.Fatalf("expected 403 for non-admin, got %d", status)
	}
	if len(state.GetAllGroups()) != 0 {
		t.Fatal("non-admin request must not create a group")
	}
}

func TestGroupPutHandler_Validation(t *testing.T) {
	setupAccessHTTPTest(t)
	ctx := groupCtx(t, "admin", true)
	for name, body := range map[string]string{
		"invalid json": `{not json`,
		"empty name":   `{"group":"","members":["a"]}`,
		"blank name":   `{"group":"   ","members":["a"]}`,
	} {
		status, _, _ := doGroupRequest(t, groupPutHandler, http.MethodPut, "/api/access/group", body, ctx)
		if status != http.StatusBadRequest {
			t.Fatalf("%s: expected 400, got %d", name, status)
		}
	}
	if len(state.GetAllGroups()) != 0 {
		t.Fatal("invalid requests must not create groups")
	}
}

func TestGroupPutHandler_ReplacesMembersAndReturnsJSON(t *testing.T) {
	setupAccessHTTPTest(t)
	ctx := groupCtx(t, "admin", true)

	status, rec, err := doGroupRequest(t, groupPutHandler, http.MethodPut, "/api/access/group", `{"group":" editors ","members":["alice","bob"]}`, ctx)
	if status != http.StatusOK || err != nil {
		t.Fatalf("expected 200, got %d (err=%v)", status, err)
	}
	assertJSONMessage(t, rec, "group saved")

	if _, _, err = doGroupRequest(t, groupPutHandler, http.MethodPut, "/api/access/group", `{"group":"editors","members":["carol"]}`, ctx); err != nil {
		t.Fatal(err)
	}
	got := state.GetGroupMembers()["editors"]
	if len(got) != 1 || got[0] != "carol" {
		t.Fatalf("expected membership replaced with [carol], got %v", got)
	}
}

func TestGroupDeleteHandler_MemberVersusWholeGroup(t *testing.T) {
	setupAccessHTTPTest(t)
	ctx := groupCtx(t, "admin", true)
	if err := state.SetGroupMembers("editors", []string{"alice", "bob"}); err != nil {
		t.Fatal(err)
	}

	status, rec, err := doGroupRequest(t, groupDeleteHandler, http.MethodDelete, "/api/access/group?group=editors&user=alice", "", ctx)
	if status != http.StatusOK || err != nil {
		t.Fatalf("expected 200, got %d (err=%v)", status, err)
	}
	assertJSONMessage(t, rec, "user removed from group")
	if got := state.GetGroupMembers()["editors"]; len(got) != 1 || got[0] != "bob" {
		t.Fatalf("expected only bob left, got %v", got)
	}

	status, rec, err = doGroupRequest(t, groupDeleteHandler, http.MethodDelete, "/api/access/group?group=editors", "", ctx)
	if status != http.StatusOK || err != nil {
		t.Fatalf("expected 200, got %d (err=%v)", status, err)
	}
	assertJSONMessage(t, rec, "group deleted")
	if _, ok := state.GetGroupMembers()["editors"]; ok {
		t.Fatal("group should be deleted when no user is given")
	}

	status, _, _ = doGroupRequest(t, groupDeleteHandler, http.MethodDelete, "/api/access/group", "", ctx)
	if status != http.StatusBadRequest {
		t.Fatalf("expected 400 without group, got %d", status)
	}
	status, _, _ = doGroupRequest(t, groupDeleteHandler, http.MethodDelete, "/api/access/group?group=x", "", groupCtx(t, "bob", false))
	if status != http.StatusForbidden {
		t.Fatalf("expected 403 for non-admin, got %d", status)
	}
}

func TestGroupPostHandler_ReturnsJSON(t *testing.T) {
	setupAccessHTTPTest(t)
	status, rec, err := doGroupRequest(t, groupPostHandler, http.MethodPost, "/api/access/group?group=editors&user=alice", "", groupCtx(t, "admin", true))
	if status != http.StatusOK || err != nil {
		t.Fatalf("expected 200, got %d (err=%v)", status, err)
	}
	assertJSONMessage(t, rec, "user added to group")
	if got := state.GetUserGroups("alice"); len(got) != 1 || got[0] != "editors" {
		t.Fatalf("expected alice in editors, got %v", got)
	}
}

func TestGroupGetHandler_MembersParam(t *testing.T) {
	setupAccessHTTPTest(t)
	ctx := groupCtx(t, "admin", true)
	if err := state.SetGroupMembers("editors", []string{"alice"}); err != nil {
		t.Fatal(err)
	}

	decode := func(target string) GroupListResponse {
		t.Helper()
		status, rec, err := doGroupRequest(t, groupGetHandler, http.MethodGet, target, "", ctx)
		if status != http.StatusOK || err != nil {
			t.Fatalf("expected 200, got %d (err=%v)", status, err)
		}
		var resp GroupListResponse
		if err := json.Unmarshal(rec.Body.Bytes(), &resp); err != nil {
			t.Fatal(err)
		}
		return resp
	}

	plain := decode("/api/access/groups")
	if len(plain.Groups) != 1 || plain.Members != nil {
		t.Fatalf("expected names only without members=true, got %+v", plain)
	}
	withMembers := decode("/api/access/groups?members=true")
	if got := withMembers.Members["editors"]; len(got) != 1 || got[0] != "alice" {
		t.Fatalf("expected members[editors]=[alice], got %+v", withMembers)
	}
}
