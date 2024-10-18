package http

import (
	"fmt"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/gtsteffaniak/filebrowser/share"
)

func TestPublicShareHandlerAuthentication(t *testing.T) {
	t.Parallel()

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
		for handlerName, handler := range map[string]handleFunc{
			"public share handler": publicShareHandler,
			"public dl handler":    publicDlHandler,
		} {
			t.Run(fmt.Sprintf("%s: %s", handlerName, name), func(t *testing.T) {
				t.Parallel()

				// Create a response recorder to capture the handler's output
				recorder := httptest.NewRecorder()

				// Set up a mock requestContext
				ctx := &requestContext{
					raw: &share.Link{
						Hash: tc.share.Hash,
					},
				}

				// Call the handler with the test request and mock context
				handler(recorder, tc.req, ctx)

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
