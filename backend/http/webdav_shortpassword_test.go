package http

import (
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/gtsteffaniak/filebrowser/backend/auth"
	"github.com/gtsteffaniak/filebrowser/backend/common/utils"
	"github.com/gtsteffaniak/filebrowser/backend/database/users"
)

// TestWebDAVShortPasswordAuth verifies that WebDAV Basic Auth accepts both the full
// JWT token (legacy behavior, username ignored) and the short password derived from it
// (new behavior, requires the matching username).
func TestWebDAVShortPasswordAuth(t *testing.T) {
	setupTestEnv(t)

	user := &users.User{
		ID:          1,
		Username:    "webdavuser",
		Permissions: users.Permissions{Download: true},
	}
	if err := store.Users.Save(user, true, true); err != nil {
		t.Fatal("failed to save user:", err)
	}

	// Create a real signed token and store it on the user like the API handler does.
	tokenString, meta, err := auth.MakeSignedTokenAPI(user, "webdav", time.Hour*2, user.Permissions, false)
	if err != nil {
		t.Fatalf("failed to make token: %v", err)
	}
	if err := store.Users.AddApiToken(user.ID, "webdav", tokenString, meta); err != nil {
		t.Fatalf("failed to add api token: %v", err)
	}

	shortPassword := utils.WebdavShortPassword(tokenString)

	testCases := []struct {
		name               string
		username           string
		password           string
		expectedStatusCode int
	}{
		{
			name:               "Full token without username still works (legacy)",
			username:           "",
			password:           tokenString,
			expectedStatusCode: http.StatusOK,
		},
		{
			name:               "Short password with matching username works",
			username:           "webdavuser",
			password:           shortPassword,
			expectedStatusCode: http.StatusOK,
		},
		{
			name:               "Short password with wrong username fails",
			username:           "someoneelse",
			password:           shortPassword,
			expectedStatusCode: http.StatusUnauthorized,
		},
		{
			name:               "Short password without username fails",
			username:           "",
			password:           shortPassword,
			expectedStatusCode: http.StatusUnauthorized,
		},
		{
			name:               "Non-matching password of short-password length fails",
			username:           "webdavuser",
			password:           "0123456789abcdef", // 16 chars, but matches no token
			expectedStatusCode: http.StatusUnauthorized,
		},
	}

	handler := withBasicAuthHelper(mockHandler)

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			req := httptest.NewRequest(http.MethodGet, "/dav/srv/", http.NoBody)
			req.SetBasicAuth(tc.username, tc.password)

			recorder := httptest.NewRecorder()
			data := &requestContext{}

			status, _ := handler(recorder, req, data)
			if status != tc.expectedStatusCode {
				t.Errorf("expected status %d, got %d", tc.expectedStatusCode, status)
			}
		})
	}
}
