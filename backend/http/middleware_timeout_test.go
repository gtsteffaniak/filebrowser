package http

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"
)

func TestWithTimeoutReturnsJSONTimeoutReason(t *testing.T) {
	t.Parallel()
	timeout := 50 * time.Millisecond
	handler := withTimeout(timeout, func(w http.ResponseWriter, r *http.Request, data *requestContext) (int, error) {
		<-r.Context().Done()
		return http.StatusOK, nil
	})

	req := httptest.NewRequest(http.MethodGet, "/test", nil)
	rec := httptest.NewRecorder()
	handler(rec, req)

	if rec.Code != http.StatusRequestTimeout {
		t.Fatalf("status = %d, want %d", rec.Code, http.StatusRequestTimeout)
	}
	if ct := rec.Header().Get("Content-Type"); ct != "application/json; charset=utf-8" {
		t.Fatalf("content-type = %q", ct)
	}

	var body HttpResponse
	if err := json.Unmarshal(rec.Body.Bytes(), &body); err != nil {
		t.Fatalf("unmarshal: %v", err)
	}
	if body.Status != http.StatusRequestTimeout {
		t.Fatalf("body.status = %d", body.Status)
	}
	if body.Message != "request timed out after "+timeout.String() {
		t.Fatalf("body.message = %q, want request timed out after %q", body.Message, timeout.String())
	}
}

func TestWithTimeoutHandlerErrorBeforeDeadline(t *testing.T) {
	t.Parallel()
	handler := withTimeout(time.Second, func(w http.ResponseWriter, r *http.Request, data *requestContext) (int, error) {
		return http.StatusBadRequest, context.Canceled
	})

	req := httptest.NewRequest(http.MethodGet, "/test", nil)
	rec := httptest.NewRecorder()
	handler(rec, req)

	if rec.Code != http.StatusBadRequest {
		t.Fatalf("status = %d, want %d", rec.Code, http.StatusBadRequest)
	}
	var body HttpResponse
	if err := json.Unmarshal(rec.Body.Bytes(), &body); err != nil {
		t.Fatalf("unmarshal: %v", err)
	}
	if body.Message != context.Canceled.Error() {
		t.Fatalf("body.message = %q", body.Message)
	}
}
