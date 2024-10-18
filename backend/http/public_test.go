package http

import (
	"fmt"
	"net/http"
	"net/http/httptest"
	"path/filepath"
	"testing"

	"github.com/asdine/storm/v3"
	"github.com/gtsteffaniak/filebrowser/diskcache"
	"github.com/gtsteffaniak/filebrowser/img"
	"github.com/gtsteffaniak/filebrowser/settings"
	"github.com/gtsteffaniak/filebrowser/share"
	"github.com/gtsteffaniak/filebrowser/storage"
	"github.com/gtsteffaniak/filebrowser/storage/bolt"
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
func TestPublicShareHandlerAuthentication(t *testing.T) {
	t.Parallel()
	setupTestEnv(t)
	const passwordBcrypt = "$2y$10$TFAmdCbyd/mEZDe5fUeZJu.MaJQXRTwdqb/IQV.eTn6dWrF58gCSe" //nolint:gosec
	testCases := map[string]struct {
		share              *share.Link
		req                *http.Request
		expectedStatusCode int
	}{
		"Public share, no auth required": {
			share:              &share.Link{Hash: "h"},
			req:                newHTTPRequest(t),
			expectedStatusCode: 200,
		},
		"Private share, no auth provided, 401": {
			share:              &share.Link{Hash: "h", UserID: 1, PasswordHash: passwordBcrypt, Token: "123"},
			req:                newHTTPRequest(t),
			expectedStatusCode: 401,
		},
		"Private share, authentication via token": {
			share:              &share.Link{Hash: "h", UserID: 1, PasswordHash: passwordBcrypt, Token: "123"},
			req:                newHTTPRequest(t, func(r *http.Request) { r.URL.RawQuery = "token=123" }),
			expectedStatusCode: 200,
		},
		"Private share, authentication via invalid token, 401": {
			share:              &share.Link{Hash: "h", UserID: 1, PasswordHash: passwordBcrypt, Token: "123"},
			req:                newHTTPRequest(t, func(r *http.Request) { r.URL.RawQuery = "token=1234" }),
			expectedStatusCode: 401,
		},
		"Private share, authentication via password": {
			share:              &share.Link{Hash: "h", UserID: 1, PasswordHash: passwordBcrypt, Token: "123"},
			req:                newHTTPRequest(t, func(r *http.Request) { r.Header.Set("X-SHARE-PASSWORD", "password") }),
			expectedStatusCode: 200,
		},
		"Private share, authentication via invalid password, 401": {
			share:              &share.Link{Hash: "h", UserID: 1, PasswordHash: passwordBcrypt, Token: "123"},
			req:                newHTTPRequest(t, func(r *http.Request) { r.Header.Set("X-SHARE-PASSWORD", "wrong-password") }),
			expectedStatusCode: 401,
		},
	}

	for name, tc := range testCases {
		for handlerName, handler := range map[string]func(http.HandlerFunc) http.HandlerFunc{
			"public share handler": func(h http.HandlerFunc) http.HandlerFunc { return withHashFile(publicShareHandler) },
			"public dl handler":    func(h http.HandlerFunc) http.HandlerFunc { return withHashFile(publicDlHandler) },
		} {
			t.Run(fmt.Sprintf("%s: %s", handlerName, name), func(t *testing.T) {
				t.Parallel()

				// Create a response recorder to capture the handler's output
				recorder := httptest.NewRecorder()

				// Wrap the handler with the necessary middleware
				wrappedHandler := handler(nil) // the handler expects http.HandlerFunc, but middleware already wraps
				// Create a test server to serve the request
				testServer := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
					wrappedHandler(w, r)
				}))
				defer testServer.Close()

				// Call the handler with the test request and mock context
				wrappedHandler(recorder, tc.req)

				// Get the result
				result := recorder.Result()
				defer result.Body.Close()

				// Check the status code
				if result.StatusCode != tc.expectedStatusCode {
					t.Errorf("expected status code %d, got %d", tc.expectedStatusCode, result.StatusCode)
				}
			})
		}
	}
}

// Helper function to create an HTTP request with optional modifications
func newHTTPRequest(t *testing.T, requestModifiers ...func(*http.Request)) *http.Request {
	t.Helper()
	r, err := http.NewRequest(http.MethodGet, "h", http.NoBody)
	if err != nil {
		t.Fatalf("failed to construct request: %v", err)
	}
	for _, modify := range requestModifiers {
		modify(r)
	}
	return r
}
