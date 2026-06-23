package activity

import (
	"testing"

	"github.com/gtsteffaniak/filebrowser/backend/database/users"
)

func TestTrimPathForUserScope(t *testing.T) {
	tests := []struct {
		path  string
		scope string
		want  string
	}{
		{path: "/users/alice/docs/file.txt", scope: "/users/alice", want: "/docs/file.txt"},
		{path: "/source/users/graham/path/to/file", scope: "/source/users/graham", want: "/path/to/file"},
		{path: "/users/alice", scope: "/users/alice", want: "/"},
		{path: "/docs/file.txt", scope: "/users/alice", want: "/docs/file.txt"},
		{path: "/docs/file.txt", scope: "/", want: "/docs/file.txt"},
		{path: "", scope: "/users/alice", want: ""},
	}
	for _, tc := range tests {
		got := TrimPathForUserScope(tc.path, tc.scope)
		if got != tc.want {
			t.Fatalf("TrimPathForUserScope(%q, %q) = %q, want %q", tc.path, tc.scope, got, tc.want)
		}
	}
}

func TestFrontendEntryTrimPathsForUser(t *testing.T) {
	actor := &users.User{
		ID: 1,
		BackendScopes: []users.BackendScope{
			{Path: "/data", Scope: "/users/alice"},
		},
	}
	fe := FrontendEntry{
		Source: "/data",
		Path:   "/users/alice/docs/readme.txt",
		Details: FrontendDetails{
			Source: "/data",
			Paths:  []string{"/users/alice/docs/a.txt", "/users/alice/docs/b.txt"},
		},
	}
	fe.TrimPathsForUser(actor)
	if fe.Path != "/docs/readme.txt" {
		t.Fatalf("Path = %q, want /docs/readme.txt", fe.Path)
	}
	if fe.Details.Paths[0] != "/docs/a.txt" || fe.Details.Paths[1] != "/docs/b.txt" {
		t.Fatalf("Details.Paths = %#v", fe.Details.Paths)
	}
}
