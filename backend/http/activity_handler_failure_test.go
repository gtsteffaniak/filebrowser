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

func TestShouldRecordHandlerFailureSkipsActivityEndpoints(t *testing.T) {
	r := newActivityTestRequest(http.MethodGet, "/tools/activity")
	if shouldRecordHandlerFailure(r, http.StatusBadRequest) {
		t.Fatal("expected activity list failures to be skipped")
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
