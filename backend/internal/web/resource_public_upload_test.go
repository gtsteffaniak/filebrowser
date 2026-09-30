package web

import (
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/gtsteffaniak/filebrowser/backend/internal/database/share"
)

func TestPublicUploadHandlerDeniesOverrideWhenReplacementsDisallowed(t *testing.T) {
	t.Parallel()

	d := &Context{
		Share: share.Share{
			ShareSettings: share.ShareSettings{
				FrontendShareInfo: share.FrontendShareInfo{
					ShareType:         "upload",
					AllowReplacements: false,
				},
			},
		},
	}

	for _, query := range []string{"override=true", "action=override"} {
		req := httptest.NewRequest(http.MethodPost, "/public/api/resources?"+query, nil)
		status, err := publicUploadHandler(httptest.NewRecorder(), req, d)
		if status != http.StatusForbidden {
			t.Fatalf("query %q: expected 403, got %d", query, status)
		}
		if err == nil || err.Error() != "cannot overwrite files for this share" {
			t.Fatalf("query %q: unexpected err: %v", query, err)
		}
	}
}

func TestPublicUploadHandlerAllowsOverrideQueryWhenReplacementsAllowed(t *testing.T) {
	t.Parallel()

	d := &Context{
		Share: share.Share{
			ShareSettings: share.ShareSettings{
				FrontendShareInfo: share.FrontendShareInfo{
					ShareType:         "upload",
					AllowReplacements: true,
				},
			},
		},
	}
	req := httptest.NewRequest(http.MethodPost, "/public/api/resources?override=true", nil)
	status, _ := publicUploadHandler(httptest.NewRecorder(), req, d)
	if status == http.StatusForbidden {
		t.Fatal("expected override guard not to forbid when AllowReplacements is true")
	}
}
