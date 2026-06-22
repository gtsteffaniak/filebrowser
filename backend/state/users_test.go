package state

import (
	"path/filepath"
	"strings"
	"testing"

	"github.com/gtsteffaniak/filebrowser/backend/common/settings"
	"github.com/gtsteffaniak/filebrowser/backend/database/users"
)

func TestCreateUserValidateUsername(t *testing.T) {
	t.Setenv("FILEBROWSER_ONLYOFFICE_SECRET", "")
	settings.Initialize("../../_docker/src/noauth/backend/config.yaml")
	settings.Env.IsPlaywright = true

	dbPath := filepath.Join(t.TempDir(), "filebrowser.sqlite")
	_, err := Initialize(dbPath)
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = Close() })

	tests := []struct {
		name     string
		username string
		wantErr  string
	}{
		{name: "empty username", username: "", wantErr: "username is empty"},
		{name: "reserved anonymous", username: "anonymous", wantErr: "reserved"},
		{name: "valid username", username: "alice", wantErr: ""},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			u := &users.User{
				FrontendUser: users.FrontendUser{
					Username: tc.username,
				},
			}
			err := CreateUser(u, "password")
			if tc.wantErr == "" {
				if err != nil {
					t.Fatalf("CreateUser(%q): unexpected error: %v", tc.username, err)
				}
				return
			}
			if err == nil {
				t.Fatalf("CreateUser(%q): expected error containing %q", tc.username, tc.wantErr)
			}
			if !strings.Contains(err.Error(), tc.wantErr) {
				t.Fatalf("CreateUser(%q): got %v, want error containing %q", tc.username, err, tc.wantErr)
			}
		})
	}
}
