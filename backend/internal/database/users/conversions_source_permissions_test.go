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

func TestFilePermsForSourcePathLongestPrefix(t *testing.T) {
	tests := []struct {
		name       string
		scopes     []BackendScope
		source     string
		path       string
		wantView   bool
		wantCreate bool
		wantOK     bool
	}{
		{
			name: "nested scopes - child permissions override parent",
			scopes: []BackendScope{
				{
					Path:        "/srv",
					Scope:       "/khoi5",
					Permissions: SourceFilePermissions{View: true, Create: false, Modify: false, Delete: false},
				},
				{
					Path:        "/srv",
					Scope:       "/khoi5/5A",
					Permissions: SourceFilePermissions{View: true, Create: true, Modify: true, Delete: true},
				},
			},
			source:     "/srv",
			path:       "/khoi5/5A/test.txt",
			wantView:   true,
			wantCreate: true,
			wantOK:     true,
		},
		{
			name: "nested scopes - parent permissions for path outside child",
			scopes: []BackendScope{
				{
					Path:        "/srv",
					Scope:       "/khoi5",
					Permissions: SourceFilePermissions{View: true, Create: false},
				},
				{
					Path:        "/srv",
					Scope:       "/khoi5/5A",
					Permissions: SourceFilePermissions{View: true, Create: true},
				},
			},
			source:     "/srv",
			path:       "/khoi5/5B/test.txt",
			wantView:   true,
			wantCreate: false,
			wantOK:     true,
		},
		{
			name: "root scope with child - child wins for deeper path, root used outside",
			scopes: []BackendScope{
				{
					Path:        "/srv",
					Scope:       "/",
					Permissions: SourceFilePermissions{View: true, Create: false},
				},
				{
					Path:        "/srv",
					Scope:       "/khoi5/5A",
					Permissions: SourceFilePermissions{View: true, Create: true},
				},
			},
			source:     "/srv",
			path:       "/khoi5/5B/doc.txt",
			wantView:   true,
			wantCreate: false,
			wantOK:     true,
		},
		{
			name: "prefix boundary enforced (/khoi5 does not match /khoi50)",
			scopes: []BackendScope{
				{
					Path:        "/srv",
					Scope:       "/khoi5",
					Permissions: SourceFilePermissions{View: true, Create: true},
				},
			},
			source:     "/srv",
			path:       "/khoi50/doc.txt",
			wantView:   false,
			wantCreate: false,
			wantOK:     false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			user := &User{BackendScopes: tt.scopes}
			perms, ok := user.FilePermsForSourcePath(tt.source, tt.path)
			if ok != tt.wantOK {
				t.Fatalf("FilePermsForSourcePath() ok = %v, want %v", ok, tt.wantOK)
			}
			if perms.View != tt.wantView {
				t.Errorf("FilePermsForSourcePath() View = %v, want %v", perms.View, tt.wantView)
			}
			if perms.Create != tt.wantCreate {
				t.Errorf("FilePermsForSourcePath() Create = %v, want %v", perms.Create, tt.wantCreate)
			}
		})
	}
}
