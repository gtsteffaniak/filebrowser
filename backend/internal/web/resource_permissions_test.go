package web

import (
	"net/http"
	"testing"

	"github.com/gtsteffaniak/filebrowser/backend/internal/database/users"
)

func TestResourcePostPermCheck(t *testing.T) {
	t.Parallel()

	full := users.SourceFilePermissions{View: true, Download: true, Modify: true, Create: true, Delete: true}
	createOnly := users.SourceFilePermissions{View: true, Download: true, Create: true}
	modifyOnly := users.SourceFilePermissions{View: true, Download: true, Modify: true}

	tests := []struct {
		name     string
		exists   bool
		override bool
		perms    users.SourceFilePermissions
		wantErr  bool
	}{
		{name: "new target requires create", exists: false, perms: createOnly},
		{name: "new target denied without create", exists: false, perms: modifyOnly, wantErr: true},
		{name: "override existing requires modify", exists: true, override: true, perms: full},
		{name: "override existing denied without modify", exists: true, override: true, perms: createOnly, wantErr: true},
		{name: "existing without override skips modify gate", exists: true, override: false, perms: createOnly},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			status, err := resourcePostPermCheck(tt.exists, tt.override, tt.perms)
			if tt.wantErr {
				if err == nil || status != http.StatusForbidden {
					t.Fatalf("status=%d err=%v, want forbidden", status, err)
				}
				return
			}
			if err != nil || status != 0 {
				t.Fatalf("status=%d err=%v, want allow", status, err)
			}
		})
	}
}

func TestResourcePatchPermCheck(t *testing.T) {
	t.Parallel()

	full := users.SourceFilePermissions{View: true, Download: true, Modify: true, Create: true, Delete: true}
	downloadOnly := users.SourceFilePermissions{View: true, Download: true}
	createOnly := users.SourceFilePermissions{View: true, Create: true}
	modifyOnly := users.SourceFilePermissions{View: true, Modify: true}

	tests := []struct {
		name       string
		action     string
		fromSource string
		toSource   string
		fromPerms  users.SourceFilePermissions
		toPerms    users.SourceFilePermissions
		wantMsg    string
	}{
		{
			name: "copy requires download and create",
			action: "copy", fromSource: "a", toSource: "b",
			fromPerms: full, toPerms: full,
		},
		{
			name: "copy denied without source download",
			action: "copy", fromSource: "a", toSource: "b",
			fromPerms: createOnly, toPerms: full,
			wantMsg: "user is not allowed to copy",
		},
		{
			name: "copy denied without destination create",
			action: "copy", fromSource: "a", toSource: "b",
			fromPerms: downloadOnly, toPerms: modifyOnly,
			wantMsg: "user is not allowed to copy",
		},
		{
			name: "same-source move requires modify",
			action: "move", fromSource: "a", toSource: "a",
			fromPerms: modifyOnly, toPerms: modifyOnly,
		},
		{
			name: "same-source rename requires modify",
			action: "rename", fromSource: "a", toSource: "a",
			fromPerms: modifyOnly, toPerms: modifyOnly,
		},
		{
			name: "move denied without source modify",
			action: "move", fromSource: "a", toSource: "a",
			fromPerms: downloadOnly, toPerms: downloadOnly,
			wantMsg: "user is not allowed to modify",
		},
		{
			name: "cross-source move requires destination modify",
			action: "move", fromSource: "a", toSource: "b",
			fromPerms: full, toPerms: downloadOnly,
			wantMsg: "user is not allowed to modify destination source",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := resourcePatchPermCheck(tt.action, tt.fromSource, tt.toSource, tt.fromPerms, tt.toPerms)
			if got != tt.wantMsg {
				t.Fatalf("resourcePatchPermCheck() = %q, want %q", got, tt.wantMsg)
			}
		})
	}
}

func TestWebDAVMethodPermissionMatrix(t *testing.T) {
	t.Parallel()

	type check struct {
		method string
		perms  users.SourceFilePermissions
		allow  bool
	}

	tests := []check{
		{method: "PROPFIND", perms: users.SourceFilePermissions{View: true}, allow: true},
		{method: http.MethodOptions, perms: users.SourceFilePermissions{View: true}, allow: true},
		{method: http.MethodGet, perms: users.SourceFilePermissions{View: true}, allow: false},
		{method: http.MethodHead, perms: users.SourceFilePermissions{Download: true}, allow: true},
		{method: "PROPFIND", perms: users.SourceFilePermissions{Download: true}, allow: false},
		{method: "MKCOL", perms: users.SourceFilePermissions{Create: true}, allow: true},
		{method: "MKCOL", perms: users.SourceFilePermissions{Modify: true}, allow: false},
		{method: http.MethodPut, perms: users.SourceFilePermissions{Modify: true}, allow: true},
		{method: http.MethodPut, perms: users.SourceFilePermissions{Create: true}, allow: false},
		{method: http.MethodDelete, perms: users.SourceFilePermissions{Delete: true}, allow: true},
		{method: http.MethodDelete, perms: users.SourceFilePermissions{Modify: true}, allow: false},
		{method: "COPY", perms: users.SourceFilePermissions{Download: true, Create: true}, allow: true},
		{method: "COPY", perms: users.SourceFilePermissions{Download: true, Modify: true}, allow: false},
		{method: "COPY", perms: users.SourceFilePermissions{Create: true, Modify: true}, allow: false},
		{method: "MOVE", perms: users.SourceFilePermissions{Modify: true}, allow: true},
		{method: "MOVE", perms: users.SourceFilePermissions{Create: true, Download: true}, allow: false},
		{method: "PROPPATCH", perms: users.SourceFilePermissions{View: true, Download: true, Modify: true, Create: true, Delete: true}, allow: false},
		{method: "LOCK", perms: users.SourceFilePermissions{View: true, Download: true, Modify: true, Create: true, Delete: true}, allow: false},
	}

	for _, tt := range tests {
		name := tt.method
		if !tt.allow {
			name += "_denied"
		}
		t.Run(name, func(t *testing.T) {
			status, err := webDAVMethodPermission(tt.method, tt.perms)
			if tt.allow {
				if err != nil || status != 0 {
					t.Fatalf("status=%d err=%v, want allow", status, err)
				}
				return
			}
			if err == nil || status != http.StatusForbidden {
				t.Fatalf("status=%d err=%v, want forbidden", status, err)
			}
		})
	}
}
