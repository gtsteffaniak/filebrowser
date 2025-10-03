package http

import (
	"net/http"
	"net/http/httptest"
	"path/filepath"
	"testing"
	"time"

	storm "github.com/asdine/storm/v3"
	"github.com/gtsteffaniak/filebrowser/backend/common/settings"
	"github.com/gtsteffaniak/filebrowser/backend/common/utils"
	"github.com/gtsteffaniak/filebrowser/backend/database/access"
	"github.com/gtsteffaniak/filebrowser/backend/database/share"
	"github.com/gtsteffaniak/filebrowser/backend/database/storage/bolt"
	"github.com/gtsteffaniak/filebrowser/backend/database/users"
	"github.com/gtsteffaniak/filebrowser/backend/indexing/iteminfo"
)

func setupTestEnv(t *testing.T) {
	dbPath := filepath.Join(t.TempDir(), "db")
	db, err := storm.Open(dbPath)
	if err != nil {
		t.Fatal(err)
	}
	store, err = bolt.NewStorage(db)
	if err != nil {
		t.Fatal(err)
	}
	config = &settings.Config // mocked
	config.Server.SourceMap = map[string]*settings.Source{
		"/srv": &settings.Source{
			Path: "/srv",
			Name: "srv",
		},
	}
	mockFileInfoFaster(t) // Mock FileInfoFasterFunc for this test
}

func mockFileInfoFaster(t *testing.T) {
	// Backup the original function
	originalFileInfoFaster := FileInfoFasterFunc
	// Defer restoration of the original function
	t.Cleanup(func() { FileInfoFasterFunc = originalFileInfoFaster })

	// Mock the function to skip execution
	FileInfoFasterFunc = func(opts utils.FileOptions, access *access.Storage) (*iteminfo.ExtendedFileInfo, error) {
		return &iteminfo.ExtendedFileInfo{
			FileInfo: iteminfo.FileInfo{
				Path: opts.Path,
				ItemInfo: iteminfo.ItemInfo{
					Name: "mocked_file",
					Size: 12345,
				},
			},
		}, nil
	}
}

func TestWithAdminHelper(t *testing.T) {
	setupTestEnv(t)
	// Mock a user who has admin permissions
	adminUser := &users.User{
		ID:          1,
		Username:    "admin",
		Permissions: users.Permissions{Admin: true}, // Ensure the user is an admin
	}
	nonAdminUser := &users.User{
		ID:          2,
		Username:    "non-admin",
		Permissions: users.Permissions{Admin: false}, // Non-admin user
	}
	// Save the users to the mock database
	if err := store.Users.Save(adminUser, true, true); err != nil {
		t.Fatal("failed to save admin user:", err)
	}
	if err := store.Users.Save(nonAdminUser, true, true); err != nil {
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
			token, err := makeSignedTokenAPI(tc.user, "WEB_TOKEN_"+utils.InsecureRandomIdentifier(4), time.Hour*2, tc.user.Perm)
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
				Value: token.Key,
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

func TestPublicShareHandlerAuthentication(t *testing.T) {
	setupTestEnv(t)

	const passwordBcrypt = "$2y$10$TFAmdCbyd/mEZDe5fUeZJu.MaJQXRTwdqb/IQV.eTn6dWrF58gCSe" // bcrypt hashed password

	testCases := []struct {
		name               string
		share              *share.Link
		token              string
		password           string
		extraHeaders       map[string]string
		expectedStatusCode int
	}{
		{
			name: "Public share, no auth required",
			share: &share.Link{
				Hash: "public_hash",
				CommonShare: share.CommonShare{
					Source: "/srv",
				},
			},
			expectedStatusCode: 0, // zero means 200 on helpers
		},
		{
			name: "Private share, valid password when token exists",
			share: &share.Link{
				Hash:         "pw_and_token_hash",
				UserID:       1,
				PasswordHash: passwordBcrypt,
				Token:        "some_random_token",
				CommonShare: share.CommonShare{
					Source: "/srv",
				},
			},
			extraHeaders: map[string]string{
				"X-SHARE-PASSWORD": "password",
			},
			expectedStatusCode: 0, // zero means 200 on helpers
		},
		{
			name: "Private share, no auth provided",
			share: &share.Link{
				Hash:         "private_hash",
				UserID:       1,
				PasswordHash: passwordBcrypt,
				Token:        "123",
			},
			expectedStatusCode: http.StatusUnauthorized,
		},
		{
			name: "Private share, valid token",
			share: &share.Link{
				Hash:         "token_hash",
				UserID:       1,
				PasswordHash: passwordBcrypt,
				Token:        "123",
				CommonShare: share.CommonShare{
					Source: "/srv",
				},
			},
			token:              "123",
			expectedStatusCode: 0, // zero means 200 on helpers
		},
		{
			name: "Private share, invalid password",
			share: &share.Link{
				Hash:         "pw_hash",
				UserID:       1,
				PasswordHash: passwordBcrypt,
				Token:        "123",
			},
			extraHeaders: map[string]string{
				"X-SHARE-PASSWORD": "wrong-password",
			},
			expectedStatusCode: http.StatusUnauthorized,
		},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			// Save the share in the mock store
			if err := store.Share.Save(tc.share); err != nil {
				t.Fatal("failed to save share:", err)
			}

			// Create a response recorder to capture handler output
			recorder := httptest.NewRecorder()

			// Wrap the handler with authentication middleware
			handler := withHashFileHelper(publicShareHandler)
			if err := store.Settings.Save(&settings.Settings{
				Auth: settings.Auth{
					Key: "key",
				},
			}); err != nil {
				t.Fatalf("failed to save settings: %v", err)
			}

			// Prepare the request with query parameters and optional headers
			req := newTestRequest(t, tc.share.Hash, tc.token, tc.password, tc.extraHeaders)

			// Serve the request
			status, _ := handler(recorder, req, &requestContext{})

			// Check if the response matches the expected status code
			if status != tc.expectedStatusCode {
				t.Errorf("expected status code %d, got %d", tc.expectedStatusCode, status)
			}
		})
	}
}

// Helper function to create a new HTTP request with optional parameters
func newTestRequest(t *testing.T, hash, token, password string, headers map[string]string) *http.Request {
	req := newHTTPRequest(t, hash, func(r *http.Request) {
		// Set query parameters based on provided values
		q := r.URL.Query()
		q.Set("path", "/")
		q.Set("hash", hash)
		if token != "" {
			q.Set("token", token)
		}
		if password != "" {
			q.Set("password", password)
		}
		r.URL.RawQuery = q.Encode()

		// Set any extra headers if provided
		for key, value := range headers {
			r.Header.Set(key, value)
		}
	})
	return req
}

func mockHandler(w http.ResponseWriter, r *http.Request, d *requestContext) (int, error) {
	return http.StatusOK, nil // mock response
}

// Modify newHTTPRequest to accept the hash and use it in the URL path.
func newHTTPRequest(t *testing.T, hash string, requestModifiers ...func(*http.Request)) *http.Request {
	t.Helper()
	url := "/public/share/" + hash + "/" // Dynamically include the hash in the URL path
	r, err := http.NewRequest(http.MethodGet, url, http.NoBody)
	if err != nil {
		t.Fatalf("failed to create request: %v", err)
	}
	for _, modify := range requestModifiers {
		modify(r)
	}
	return r
}
