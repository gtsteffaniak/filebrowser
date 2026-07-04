package cmd

import (
	"reflect"
	"testing"

	"github.com/gtsteffaniak/filebrowser/backend/pkg/settings"
	"github.com/gtsteffaniak/filebrowser/backend/internal/database/users"
	"github.com/stretchr/testify/assert"
)

func TestUpdateUserScopes_Phases(t *testing.T) {

	sourceA := settings.Source{Path: "/pathA", Name: "sourceA", Config: settings.SourceConfig{
		DefaultEnabled:   false,
		DefaultUserScope: "/defaultA",
	}}
	sourceB := settings.Source{Path: "/pathB", Name: "sourceB", Config: settings.SourceConfig{DefaultEnabled: true, DefaultUserScope: "/defaultB"}}
	settings.Config.Server.Sources = []*settings.Source{&sourceA, &sourceB}
	settings.Config.Server.SourceMap = map[string]*settings.Source{
		"/pathA": &sourceA,
		"/pathB": &sourceB,
	}
	settings.Config.Server.NameToSource = map[string]*settings.Source{
		"sourceA": &sourceA,
		"sourceB": &sourceB,
	}
	settings.InitializeUserResolvers()

	// ---------------------
	// Test Cases Definition
	// ---------------------
	testCases := []struct {
		name           string
		user           *users.User
		expectedPhase1 []users.BackendScope
		expectedPhase2 []users.BackendScope
	}{
		{
			name: "Single valid scope",
			user: &users.User{
				BackendScopes: []users.BackendScope{
					{Scope: "/home", Path: "/pathA"},
				},
			},
			expectedPhase1: []users.BackendScope{
				{Scope: "/home", Path: "/pathA"},
				{Scope: "/defaultB", Path: "/pathB"},
			},
			expectedPhase2: []users.BackendScope{
				{Scope: "/home", Path: "/pathA"},
				{Scope: "/defaultB", Path: "/pathB"},
			},
		},
		{
			name: "Single empty scope path",
			user: &users.User{
				BackendScopes: []users.BackendScope{
					{Scope: "", Path: "/pathB"},
				},
			},
			expectedPhase1: []users.BackendScope{
				{Scope: "/defaultB", Path: "/pathB"},
			},
			expectedPhase2: []users.BackendScope{
				{Scope: "/defaultA", Path: "/pathA"},
				{Scope: "/defaultB", Path: "/pathB"},
			},
		},
		{
			name: "Two scopes, one includes username in path",
			user: &users.User{
				FrontendUser: users.FrontendUser{Username: "user123"},
				BackendScopes: []users.BackendScope{
					{Scope: "/home/user123", Path: "/pathA"},
					{Scope: "/data", Path: "/pathB"},
				},
			},
			expectedPhase1: []users.BackendScope{
				{Scope: "/home/user123", Path: "/pathA"},
				{Scope: "/data", Path: "/pathB"},
			},
			expectedPhase2: []users.BackendScope{
				{Scope: "/home/user123", Path: "/pathA"},
				{Scope: "/data", Path: "/pathB"},
			},
		},
		{
			name: "Two scopes, one with empty name",
			user: &users.User{
				BackendScopes: []users.BackendScope{
					{Scope: "/home", Path: "/pathB"},
					{Scope: "/data", Path: "/somethingElse"},
				},
			},
			expectedPhase1: []users.BackendScope{
				{Scope: "/home", Path: "/pathB"},
				{Scope: "/data", Path: "/somethingElse"},
			},
			expectedPhase2: []users.BackendScope{
				{Scope: "/defaultA", Path: "/pathA"},
				{Scope: "/home", Path: "/pathB"},
				{Scope: "/data", Path: "/somethingElse"},
			},
		},
		{
			name: "No scopes at all",
			user: &users.User{
				BackendScopes: []users.BackendScope{},
			},
			expectedPhase1: []users.BackendScope{
				{Scope: "/defaultB", Path: "/pathB"},
			},
			expectedPhase2: []users.BackendScope{
				{Scope: "/defaultA", Path: "/pathA"},
				{Scope: "/defaultB", Path: "/pathB"},
			},
		},
		{
			name: "All user Scope and source change",
			user: &users.User{
				FrontendUser: users.FrontendUser{Username: "user123"},
				BackendScopes: []users.BackendScope{
					{Scope: "/defaultC/user123", Path: "/pathC"},
					{Scope: "/defaultA/user123", Path: "/pathA"},
					{Scope: "/defaultB/user123", Path: "/pathB"},
				},
			},
			// updateUserScopes emits configured sources first (Sources order), then unknown paths.
			expectedPhase1: []users.BackendScope{
				{Scope: "/defaultA/user123", Path: "/pathA"},
				{Scope: "/defaultB/user123", Path: "/pathB"},
				{Scope: "/defaultC/user123", Path: "/pathC"},
			},
			expectedPhase2: []users.BackendScope{
				{Scope: "/defaultA/user123", Path: "/pathA"},
				{Scope: "/defaultB/user123", Path: "/pathB"},
				{Scope: "/defaultC/user123", Path: "/pathC"},
			},
		},
	}

	// ---------------------
	// Phase 1 Tests
	// ---------------------
	for _, tc := range testCases {
		t.Run(tc.name+"_Phase1", func(t *testing.T) {
			originalScopes := tc.user.BackendScopes
			updated := updateUserScopes(tc.user)
			assert.Equal(t, tc.expectedPhase1, tc.user.BackendScopes, "Phase1 scope mismatch for test case: %s", tc.name)
			assert.Equal(t, updated, !reflect.DeepEqual(originalScopes, tc.user.BackendScopes), "Phase2 scope change detection failed:\t %s vs expected:\t %s", tc.user.BackendScopes, tc.expectedPhase1)
		})
	}

	// ---------------------
	// Phase 2: rename sources but keep same paths
	// ---------------------
	sourceA = settings.Source{Path: "/pathA", Name: "sourceA", Config: settings.SourceConfig{
		DefaultEnabled:   true,
		DefaultUserScope: "/defaultA",
	}}
	sourceB = settings.Source{Path: "/pathB", Name: "sourceB", Config: settings.SourceConfig{DefaultEnabled: true, DefaultUserScope: "/defaultB"}}
	settings.Config.Server.Sources = []*settings.Source{&sourceA, &sourceB}
	settings.Config.Server.SourceMap = map[string]*settings.Source{
		"/pathA": &sourceA,
		"/pathB": &sourceB,
	}
	settings.Config.Server.NameToSource = map[string]*settings.Source{
		"sourceA": &sourceA,
		"sourceB": &sourceB,
	}

	// Run again without resetting user objects to test idempotency + renaming
	for _, tc := range testCases {
		t.Run(tc.name+"_Phase2", func(t *testing.T) {
			originalScopes := tc.user.BackendScopes
			updated := updateUserScopes(tc.user)
			assert.Equal(t, tc.expectedPhase2, tc.user.BackendScopes, "Phase2 scope mismatch for test case: %s", tc.name)
			assert.Equal(t, updated, !reflect.DeepEqual(originalScopes, tc.user.BackendScopes), "Phase2 scope change detection failed:\t %s vs expected:\t %s", tc.user.BackendScopes, tc.expectedPhase1)

		})
	}
}
