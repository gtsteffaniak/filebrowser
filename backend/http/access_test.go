package http

import (
	"bytes"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"path/filepath"
	"testing"

	"github.com/gtsteffaniak/filebrowser/backend/common/settings"
	"github.com/gtsteffaniak/filebrowser/backend/database/users"
	"github.com/gtsteffaniak/filebrowser/backend/indexing"
	"github.com/gtsteffaniak/filebrowser/backend/state"
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

	status, handlerErr := accessPostHandler(rec, req, &requestContext{user: &adminUser})
	if status != http.StatusBadRequest {
		t.Fatalf("expected status 400, got %d (err=%v body=%s)", status, handlerErr, rec.Body.String())
	}
	if handlerErr == nil || handlerErr.Error() != "user not found: test" {
		t.Fatalf("expected user not found error, got: %v", handlerErr)
	}
}
