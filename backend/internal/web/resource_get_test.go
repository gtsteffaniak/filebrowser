package web

import (
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/gtsteffaniak/filebrowser/backend/internal/database/users"
)

func TestResourceGetHandler_MissingOrEmptyPathReturns400(t *testing.T) {
	d := &Context{
		User: &users.User{
			FrontendUser: users.FrontendUser{
				Username:    "admin",
				Permissions: users.Permissions{Admin: true},
			},
		},
	}

	cases := []struct {
		name  string
		query string
	}{
		{"missing path", "source=srv"},
		{"empty path", "source=srv&path="},
		{"path traversal", "source=srv&path=../../etc/passwd"},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			req := httptest.NewRequest(http.MethodGet, "/api/resources?"+tc.query, nil)
			status, err := resourceGetHandler(httptest.NewRecorder(), req, d)
			if err == nil {
				t.Fatal("expected error")
			}
			if status != http.StatusBadRequest {
				t.Fatalf("expected 400, got %d (%v)", status, err)
			}
		})
	}
}
