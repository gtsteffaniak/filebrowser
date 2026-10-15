package web

import (
	"bytes"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/gtsteffaniak/filebrowser/backend/internal/database/users"
	"github.com/gtsteffaniak/filebrowser/backend/internal/state"
	"github.com/gtsteffaniak/filebrowser/backend/pkg/settings"
)

func TestUsersPostPartialGlobalPermissions(t *testing.T) {
	setupTestEnv(t)

	settings.Config.UserDefaults.Account.Permissions.Share = true
	settings.Config.UserDefaults.Account.Permissions.Api = true
	settings.Config.UserDefaults.Account.Permissions.Realtime = true
	state.SetEffectiveUserDefaultsForTest(t, settings.Config.UserDefaults)

	admin := &users.User{
		ID: 1,
		FrontendUser: users.FrontendUser{
			Username:    "admin",
			LoginMethod: users.LoginMethodOidc,
			Permissions: users.Permissions{Admin: true},
		},
	}
	if err := state.CreateUser(admin, ""); err != nil {
		t.Fatal(err)
	}
	admin.Permissions = users.Permissions{Admin: true}
	if err := state.UpdateUser(admin, "", "permissions"); err != nil {
		t.Fatal(err)
	}

	body := []byte(`{"data":{"username":"newbie","password":"secret","loginMethod":"password","permissions":{"admin":true}}}`)
	req := httptest.NewRequest(http.MethodPost, "/api/users", bytes.NewReader(body))
	rec := httptest.NewRecorder()
	status, err := usersPostHandler(rec, req, &Context{User: admin})
	if err != nil || status != http.StatusCreated {
		t.Fatalf("usersPostHandler() status=%d err=%v", status, err)
	}

	created, err := state.GetUserByUsername("newbie")
	if err != nil {
		t.Fatal(err)
	}
	want := users.Permissions{Admin: true, Share: true, Api: true, Realtime: true}
	if created.Permissions != want {
		t.Fatalf("permissions = %+v, want %+v", created.Permissions, want)
	}
}
