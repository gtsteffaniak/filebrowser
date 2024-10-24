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

func TestWithAdminMiddleware(t *testing.T) {
	setupTestEnv(t)
	// Mock a user who has admin permissions
	adminUser := &users.User{
		ID:       1,
		Username: "admin",
		Perm:     users.Permissions{Admin: true}, // Ensure the user is an admin
	}
	nonAdminUser := &users.User{
		ID:       2,
		Username: "non-admin",
		Perm:     users.Permissions{Admin: false}, // Non-admin user
	}
	// Save the users to the mock database
	if err := store.Users.Save(adminUser); err != nil {
		t.Fatal("failed to save admin user:", err)
	}
	if err := store.Users.Save(nonAdminUser); err != nil {
		t.Fatal("failed to save non-admin user:", err)
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
			expectedStatusCode: http.StatusForbidden, // Non-admin should be forbidden
			user:               nonAdminUser,
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
				t.Fatalf("Error making token for request: %v", err)
			}

			// Wrap the usersGetHandler with the middleware
			handler := withAdminHelper(mockHandler)

			// Create a response recorder to capture the handler's output
			recorder := httptest.NewRecorder()
			// Create the request and apply the token as a cookie
			req, err := http.NewRequest(http.MethodGet, "/users", http.NoBody)
			if err != nil {
				t.Fatalf("Error creating request: %v", err)
			}
			req.AddCookie(&http.Cookie{
				Name:  "auth",
				Value: token,
			})

			// Call the handler with the test request and mock context
			status, err := handler(recorder, req, data)
			if err != nil {
				t.Fatalf("unexpected error: %v", err)
			}

			// Verify the status code
			if status != tc.expectedStatusCode {
				t.Errorf("\"%v\" expected status code %d, got %d", tc.name, tc.expectedStatusCode, status)
			}
		})
	}
}

func mockHandler(w http.ResponseWriter, r *http.Request, d *requestContext) (int, error) {
	return http.StatusOK, nil // mock response
}
