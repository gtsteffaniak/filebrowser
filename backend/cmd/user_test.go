package cmd

import (
	"reflect"
	"testing"

	"github.com/gtsteffaniak/filebrowser/backend/common/settings"
	"github.com/gtsteffaniak/filebrowser/backend/database/users"
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

	// ---------------------
	// Test Cases Definition
	// ---------------------
	testCases := []struct {
		name           string
		user           *users.User
		expectedPhase1 []users.SourceScope
		expectedPhase2 []users.SourceScope
	}{
		{
			name: "Single valid scope",
			user: &users.User{
				Scopes: []users.SourceScope{
					{Scope: "/home", Name: "/pathA"},
				},
			},
			expectedPhase1: []users.SourceScope{
				{Scope: "/home", Name: "/pathA"},
				{Scope: "/defaultB", Name: "/pathB"},
			},
			expectedPhase2: []users.SourceScope{
				{Scope: "/home", Name: "/pathA"},
				{Scope: "/defaultB", Name: "/pathB"},
			},
		},
		{
			name: "Single empty scope path",
			user: &users.User{
				Scopes: []users.SourceScope{
					{Scope: "", Name: "/pathB"},
				},
			},
			expectedPhase1: []users.SourceScope{
				{Scope: "/defaultB", Name: "/pathB"},
			},
			expectedPhase2: []users.SourceScope{
				{Scope: "/defaultA", Name: "/pathA"},
				{Scope: "/defaultB", Name: "/pathB"},
			},
		},
		{
			name: "Two scopes, one includes username in path",
			user: &users.User{
				Username: "user123",
				Scopes: []users.SourceScope{
					{Scope: "/home/user123", Name: "/pathA"},
					{Scope: "/data", Name: "/pathB"},
				},
			},
			expectedPhase1: []users.SourceScope{
				{Scope: "/home/user123", Name: "/pathA"},
				{Scope: "/data", Name: "/pathB"},
			},
			expectedPhase2: []users.SourceScope{
				{Scope: "/home/user123", Name: "/pathA"},
				{Scope: "/data", Name: "/pathB"},
			},
		},
		{
			name: "Two scopes, one with empty name",
			user: &users.User{
				Scopes: []users.SourceScope{
					{Scope: "/home", Name: "/pathB"},
					{Scope: "/data", Name: "/somethingElse"},
				},
			},
			expectedPhase1: []users.SourceScope{
				{Scope: "/home", Name: "/pathB"},
				{Scope: "/data", Name: "/somethingElse"},
			},
			expectedPhase2: []users.SourceScope{
				{Scope: "/defaultA", Name: "/pathA"},
				{Scope: "/home", Name: "/pathB"},
				{Scope: "/data", Name: "/somethingElse"},
			},
		},
		{
			name: "No scopes at all",
			user: &users.User{
				Scopes: []users.SourceScope{},
			},
			expectedPhase1: []users.SourceScope{
				{Scope: "/defaultB", Name: "/pathB"},
			},
			expectedPhase2: []users.SourceScope{
				{Scope: "/defaultA", Name: "/pathA"},
				{Scope: "/defaultB", Name: "/pathB"},
			},
		},
		{
			name: "All user Scope and source change",
			user: &users.User{
				Username: "user123",
				Scopes: []users.SourceScope{
					{Scope: "/defaultC/user123", Name: "/pathC"},
					{Scope: "/defaultA/user123", Name: "/pathA"},
					{Scope: "/defaultB/user123", Name: "/pathB"},
				},
			},
			expectedPhase1: []users.SourceScope{
				{Scope: "/defaultA/user123", Name: "/pathA"},
				{Scope: "/defaultB/user123", Name: "/pathB"},
				{Scope: "/defaultC/user123", Name: "/pathC"},
			},
			expectedPhase2: []users.SourceScope{
				{Scope: "/defaultA/user123", Name: "/pathA"},
				{Scope: "/defaultB/user123", Name: "/pathB"},
				{Scope: "/defaultC/user123", Name: "/pathC"},
			},
		},
	}

	// ---------------------
	// Phase 1 Tests
	// ---------------------
	for _, tc := range testCases {
		t.Run(tc.name+"_Phase1", func(t *testing.T) {
			originalScopes := tc.user.Scopes
			updated := updateUserScopes(tc.user)
			assert.Equal(t, tc.expectedPhase1, tc.user.Scopes, "Phase1 scope mismatch for test case: %s", tc.name)
			assert.Equal(t, updated, !reflect.DeepEqual(originalScopes, tc.user.Scopes), "Phase2 scope change detection failed:\t %s vs expected:\t %s", tc.user.Scopes, tc.expectedPhase1)
		})
	}

	// ---------------------
	// Phase 2: rename sources but keep same paths
	// ---------------------
	sourceA = settings.Source{Path: "/pathA", Name: "sourceA", Config: settings.SourceConfig{
		DefaultEnabled:   true,
		DefaultUserScope: "/defaultA",
		CreateUserDir:    true,
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
			originalScopes := tc.user.Scopes
			updated := updateUserScopes(tc.user)
			assert.Equal(t, tc.expectedPhase2, tc.user.Scopes, "Phase2 scope mismatch for test case: %s", tc.name)
			assert.Equal(t, updated, !reflect.DeepEqual(originalScopes, tc.user.Scopes), "Phase2 scope change detection failed:\t %s vs expected:\t %s", tc.user.Scopes, tc.expectedPhase1)

		})
	}
}
