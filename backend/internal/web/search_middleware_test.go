package web

import (
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/gtsteffaniak/filebrowser/backend/internal/auth"
	"github.com/gtsteffaniak/filebrowser/backend/internal/database/users"
	"github.com/gtsteffaniak/filebrowser/backend/internal/state"
	"github.com/gtsteffaniak/filebrowser/backend/internal/toolaccess"
	"github.com/gtsteffaniak/filebrowser/backend/internal/utils"
)

func TestWithSearchToolAccess_largestRequiresSizeViewer(t *testing.T) {
	setupTestEnv(t)

	doc := toolaccess.InitialToolAccessDefaultsDocument()
	for i := range doc.Items {
		switch doc.Items[i].ToolID {
		case users.ToolAdvancedSearch:
			doc.Items[i].Enabled = true
			doc.Items[i].Enforced = false
		case users.ToolSizeViewer:
			doc.Items[i].Enabled = false
			doc.Items[i].Enforced = true
		}
	}
	if err := state.PatchToolAccessDefaults(doc); err != nil {
		t.Fatal(err)
	}

	searchUser := &users.User{
		FrontendUser: users.FrontendUser{
			Username:    "search-user",
			Permissions: users.Permissions{Api: true},
		},
	}
	if err := state.CreateUser(searchUser, ""); err != nil {
		t.Fatal("failed to create search user:", err)
	}
	searchUser.Permissions = users.Permissions{Api: true}
	if err := state.UpdateUser(searchUser, "", "permissions"); err != nil {
		t.Fatal("failed to set search user permissions:", err)
	}

	tokenString, _, err := auth.MakeSignedTokenAPI(searchUser, "WEB_TOKEN_"+utils.InsecureRandomIdentifier(4), time.Hour*2, searchUser.Permissions, false)
	if err != nil {
		t.Fatalf("failed to make token: %v", err)
	}

	handler := withUser(withSearchToolAccess(func(w http.ResponseWriter, r *http.Request, data *requestContext) (int, error) {
		return http.StatusOK, nil
	}))

	t.Run("normal search allowed", func(t *testing.T) {
		recorder := httptest.NewRecorder()
		req, err := http.NewRequest(http.MethodGet, "/tools/search?q=test", http.NoBody)
		if err != nil {
			t.Fatal(err)
		}
		req.AddCookie(&http.Cookie{Name: "filebrowser_quantum_jwt", Value: tokenString})
		handler(recorder, req)
		if recorder.Code != http.StatusOK {
			t.Fatalf("expected 200, got %d", recorder.Code)
		}
	})

	t.Run("largest search forbidden without sizeViewer", func(t *testing.T) {
		recorder := httptest.NewRecorder()
		req, err := http.NewRequest(http.MethodGet, "/tools/search?q=test&largest=true", http.NoBody)
		if err != nil {
			t.Fatal(err)
		}
		req.AddCookie(&http.Cookie{Name: "filebrowser_quantum_jwt", Value: tokenString})
		handler(recorder, req)
		if recorder.Code != http.StatusForbidden {
			t.Fatalf("expected 403, got %d", recorder.Code)
		}
	})
}
