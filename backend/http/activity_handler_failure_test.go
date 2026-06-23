package http

import (
	"net/http"
	"net/url"
	"testing"

	activitydb "github.com/gtsteffaniak/filebrowser/backend/database/activity"
)

func TestInferActivityEventTypeFromRequestUsersPut(t *testing.T) {
	r := newActivityTestRequest(http.MethodPut, "/users?username=admin")
	got, ok := inferActivityEventTypeFromRequest(r)
	if !ok {
		t.Fatal("expected mapped event type")
	}
	if got != activitydb.EventUserUpdate {
		t.Fatalf("got %q, want %q", got, activitydb.EventUserUpdate)
	}
}

func TestInferActivityEventTypeFromRequestDuplicateFinder(t *testing.T) {
	r := newActivityTestRequest(http.MethodGet, "/tools/duplicateFinder")
	got, ok := inferActivityEventTypeFromRequest(r)
	if !ok {
		t.Fatal("expected mapped event type")
	}
	if got != activitydb.EventDuplicateFinder {
		t.Fatalf("got %q, want %q", got, activitydb.EventDuplicateFinder)
	}
}

func TestInferActivityEventTypeFromRequestUnknownRoute(t *testing.T) {
	r := newActivityTestRequest(http.MethodGet, "/settings")
	if _, ok := inferActivityEventTypeFromRequest(r); ok {
		t.Fatal("expected unmapped route to skip activity recording")
	}
}

func TestInferActivityEventTypeFromRequestResourceGetWithoutDownload(t *testing.T) {
	r := newActivityTestRequest(http.MethodGet, "/resources?path=/")
	if _, ok := inferActivityEventTypeFromRequest(r); ok {
		t.Fatal("expected generic resource GET to skip activity recording")
	}
}

func TestInferActivityEventTypeFromRequestAccessMethods(t *testing.T) {
	cases := []struct {
		method string
		want   activitydb.EventType
	}{
		{http.MethodPost, activitydb.EventAccessCreate},
		{http.MethodPatch, activitydb.EventAccessUpdate},
		{http.MethodDelete, activitydb.EventAccessDelete},
	}
	for _, tc := range cases {
		r := newActivityTestRequest(tc.method, "/access?source=Downloads&path=%2F")
		got, ok := inferActivityEventTypeFromRequest(r)
		if !ok {
			t.Fatalf("%s: expected mapped event type", tc.method)
		}
		if got != tc.want {
			t.Fatalf("%s: got %q, want %q", tc.method, got, tc.want)
		}
	}
}

func TestShouldRecordHandlerFailureSkipsActivityEndpoints(t *testing.T) {
	r := newActivityTestRequest(http.MethodGet, "/tools/activity")
	if shouldRecordHandlerFailure(r, http.StatusBadRequest) {
		t.Fatal("expected activity list failures to be skipped")
	}
}

func TestAccessFailureSourcePathFromQuery(t *testing.T) {
	r := newActivityTestRequest(http.MethodPost, "/access?source=Downloads&path=%2F")
	source, path := accessFailureSourcePath(r)
	if source != "Downloads" {
		t.Fatalf("source = %q, want Downloads", source)
	}
	if path != "/" {
		t.Fatalf("path = %q, want /", path)
	}
}

func TestAccessFailureSourcePathIgnoresNonAccessRoutes(t *testing.T) {
	r := newActivityTestRequest(http.MethodPost, "/users?username=admin")
	source, path := accessFailureSourcePath(r)
	if source != "" || path != "" {
		t.Fatalf("expected empty source/path, got %q %q", source, path)
	}
}

func newActivityTestRequest(method, target string) *http.Request {
	u := mustParseTestURL("http://example.test" + target)
	return &http.Request{
		Method: method,
		URL:    u,
	}
}

func mustParseTestURL(raw string) *url.URL {
	u, err := url.Parse(raw)
	if err != nil {
		panic(err)
	}
	return u
}
