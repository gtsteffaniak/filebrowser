package users

import (
	"testing"
)

func TestGetScopeForSourcePathLongestPrefix(t *testing.T) {
	tests := []struct {
		name      string
		scopes    []BackendScope
		source    string
		path      string
		wantScope string
		wantOK    bool
	}{
		{
			name: "single scope matches exact path",
			scopes: []BackendScope{
				{Path: "/srv", Scope: "/khoi5"},
			},
			source:    "/srv",
			path:      "/khoi5/5A",
			wantScope: "/khoi5",
			wantOK:    true,
		},
		{
			name: "nested scopes - child scope wins (longest prefix)",
			scopes: []BackendScope{
				{Path: "/srv", Scope: "/khoi5"},                  // parent: read-only
				{Path: "/srv", Scope: "/khoi5/5A"},               // child: write
			},
			source:    "/srv",
			path:      "/khoi5/5A/doc.txt",
			wantScope: "/khoi5/5A",  // Should pick deepest scope
			wantOK:    true,
		},
		{
			name: "nested scopes - parent scope when path in parent only",
			scopes: []BackendScope{
				{Path: "/srv", Scope: "/khoi5"},           // parent
				{Path: "/srv", Scope: "/khoi5/5A"},        // child
			},
			source:    "/srv",
			path:      "/khoi5/5B",  // Path not in child scope
			wantScope: "/khoi5",      // Fall back to parent
			wantOK:    true,
		},
		{
			name: "multiple children - pick longest matching",
			scopes: []BackendScope{
				{Path: "/srv", Scope: "/khoi5"},
				{Path: "/srv", Scope: "/khoi5/5A"},
				{Path: "/srv", Scope: "/khoi5/5A/class1"},
			},
			source:    "/srv",
			path:      "/khoi5/5A/class1/doc.txt",
			wantScope: "/khoi5/5A/class1",  // Deepest match
			wantOK:    true,
		},
		{
			name: "no matching scope",
			scopes: []BackendScope{
				{Path: "/srv", Scope: "/khoi5"},
			},
			source:    "/srv",
			path:      "/other/path",
			wantScope: "",
			wantOK:    false,
		},
		{
			name: "exact scope match",
			scopes: []BackendScope{
				{Path: "/srv", Scope: "/khoi5"},
			},
			source:    "/srv",
			path:      "/khoi5",
			wantScope: "/khoi5",
			wantOK:    true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			user := &User{
				BackendScopes:             tt.scopes,
				BackendSourcePermissions: nil,
			}

			scope, err := user.GetScopeForSourcePath(tt.source, tt.path)

			if (err == nil) != tt.wantOK {
				t.Errorf("GetScopeForSourcePath() err = %v, wantOK %v", err, tt.wantOK)
			}
			if scope != tt.wantScope {
				t.Errorf("GetScopeForSourcePath() scope = %q, want %q", scope, tt.wantScope)
			}
		})
	}
}

func TestFilePermsForSourcePathLongestPrefix(t *testing.T) {
	tests := []struct {
		name      string
		scopes    []BackendScope
		source    string
		path      string
		wantView  bool
		wantCreate bool
	}{
		{
			name: "nested scopes - child permissions override parent",
			scopes: []BackendScope{
				{
					Path:  "/srv",
					Scope: "/khoi5",
					Permissions: SourceFilePermissions{
						View:   true,
						Create: false,
						Modify: false,
						Delete: false,
					},
				},
				{
					Path:  "/srv",
					Scope: "/khoi5/5A",
					Permissions: SourceFilePermissions{
						View:   true,
						Create: true,  // Different from parent!
						Modify: true,
						Delete: true,
					},
				},
			},
			source:     "/srv",
			path:       "/khoi5/5A/test.txt",
			wantView:   true,
			wantCreate: true,  // Should get child permissions
		},
		{
			name: "nested scopes - parent permissions for path outside child",
			scopes: []BackendScope{
				{
					Path:  "/srv",
					Scope: "/khoi5",
					Permissions: SourceFilePermissions{
						View:   true,
						Create: false,
					},
				},
				{
					Path:  "/srv",
					Scope: "/khoi5/5A",
					Permissions: SourceFilePermissions{
						View:   true,
						Create: true,
					},
				},
			},
			source:     "/srv",
			path:       "/khoi5/5B/test.txt",
			wantView:   true,
			wantCreate: false,  // Should get parent permissions
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			user := &User{
				BackendScopes:             tt.scopes,
				BackendSourcePermissions: nil,
			}

			perms, ok := user.FilePermsForSourcePath(tt.source, tt.path)

			if !ok {
				t.Fatalf("FilePermsForSourcePath() ok = false, want true")
			}
			if perms.View != tt.wantView {
				t.Errorf("View = %v, want %v", perms.View, tt.wantView)
			}
			if perms.Create != tt.wantCreate {
				t.Errorf("Create = %v, want %v", perms.Create, tt.wantCreate)
			}
		})
	}
}
