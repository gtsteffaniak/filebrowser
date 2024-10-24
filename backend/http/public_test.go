package http

import (
	"net/http"
	"net/http/httptest"
	"path/filepath"
	"testing"

	"github.com/asdine/storm/v3"
	"github.com/gtsteffaniak/filebrowser/diskcache"
	"github.com/gtsteffaniak/filebrowser/img"
	"github.com/gtsteffaniak/filebrowser/settings"
	"github.com/gtsteffaniak/filebrowser/storage"
	"github.com/gtsteffaniak/filebrowser/storage/bolt"
	"github.com/gtsteffaniak/filebrowser/users"
)

func setupTestEnv(t *testing.T) {
	dbPath := filepath.Join(t.TempDir(), "db")
	db, err := storm.Open(dbPath)
	if err != nil {
		t.Fatal(err)
	}
	authStore, userStore, shareStore, settingsStore, err := bolt.NewStorage(db)
	if err != nil {
		t.Fatal(err)
	}
	store = &storage.Storage{
		Auth:     authStore,
		Users:    userStore,
		Share:    shareStore,
		Settings: settingsStore,
	}
	fileCache = diskcache.NewNoOp() // mocked
	imgSvc = img.New(1)             // mocked
	config = &settings.Config       // mocked
}

func TestUsersGetHandlerWithAdmin(t *testing.T) {
	t.Parallel()
	setupTestEnv(t) // Ensure this is setting up the environment correctly

	// Mock a user who has admin permissions
	adminUser := &users.User{
		ID:       1,
		Username: "admin",
		Perm:     users.Permissions{Admin: true}, // Ensure the user is an admin
	}

	// Test cases for different scenarios
	testCases := []struct {
		name               string
		expectedStatusCode int
		user               *users.User
	}{
		{
			name:               "Admin access allowed",
			expectedStatusCode: http.StatusOK, // Admin should be able to access
			user:               adminUser,
		},
		{
			name:               "Non-admin access forbidden",
			expectedStatusCode: http.StatusUnauthorized, // Non-admin should be forbidden
			user: &users.User{
				ID:       2,
				Username: "non-admin",
				Perm:     users.Permissions{Admin: false}, // Non-admin user
			},
		},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			// Mock the context with the current user
			data := &requestContext{
				user: tc.user,
			}
			token, err := makeSignedToken(tc.user)
			if err != nil {
				t.Fatalf("Error making token for request")
			}
			// Wrap the usersGetHandler with the middleware
			handler := withAdminHelper(usersGetHandler)
			// Create a response recorder to capture the handler's output
			recorder := httptest.NewRecorder()
			// apply token to request, token should be set as cookie in request, as auth=${token}
			// replace applyCookie with actual method of injecting cookies
			req := newHTTPRequest(t, applyCookie("auth="+token))

			// Call the handler with the test request and mock context
			status, err := handler(recorder, req, data)
			if err != nil {
				t.Fatalf("unexpected status (%v) error: %v", status, err)
			}

			// Verify the status code
			if status != tc.expectedStatusCode {
				t.Errorf("expected status code %d, got %d", tc.expectedStatusCode, status)
			}
		})
	}
}

// Helper function to simulate HTTP requests
func newHTTPRequest(t *testing.T, requestModifiers ...func(*http.Request)) *http.Request {
	t.Helper()
	r, err := http.NewRequest(http.MethodGet, "/users", http.NoBody)
	if err != nil {
		t.Fatalf("failed to construct request: %v", err)
	}
	for _, modify := range requestModifiers {
		modify(r)
	}
	return r
}
