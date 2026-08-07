package users

import "testing"

func TestAPISourcePermsToBackendAndFrontendRoundTrip(t *testing.T) {
	had := SourceConfigLoaded()
	t.Cleanup(func() {
		if !had {
			SetSourceConfig(nil)
			SetSourceNameResolver(nil)
		}
	})
	SetSourceNameResolver(func(name string) (string, error) {
		if name == "media" {
			return "/vol/media", nil
		}
		return "", nil
	})
	SetSourceConfig(&SourceConfigProvider{
		GetSourceByPath: func(path string) (SourceInfo, bool) {
			if path == "/vol/media" {
				return SourceInfo{Path: path, Name: "media"}, true
			}
			return SourceInfo{}, false
		},
		GetSourceByName: func(name string) (SourceInfo, bool) {
			if name == "media" {
				return SourceInfo{Path: "/vol/media", Name: "media"}, true
			}
			return SourceInfo{}, false
		},
	})

	api := map[string]SourceFilePermissions{
		"media": {View: true, Download: true, Modify: false, Create: true, Delete: false},
	}
	backend, err := APISourcePermsToBackend(api)
	if err != nil {
		t.Fatal(err)
	}
	if !backend["/vol/media"].View || !backend["/vol/media"].Create {
		t.Fatalf("backend perms: %+v", backend)
	}

	user := &User{
		BackendScopes: []BackendScope{{
			Path:        "/vol/media",
			Scope:       "/",
			Permissions: backend["/vol/media"],
		}},
	}
	front := user.GetFrontendScopes()
	if len(front) != 1 || front[0].Permissions == nil {
		t.Fatalf("frontend scopes: %+v", front)
	}
	if !front[0].Permissions.View || !front[0].Permissions.Create {
		t.Fatalf("frontend perms: %+v", front[0].Permissions)
	}
}
