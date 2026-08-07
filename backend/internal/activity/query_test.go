package activity

import (
	"net/http"
	"strings"
	"testing"
)

func TestResolveActivityPathGlobRejectsTraversal(t *testing.T) {
	_, status, err := resolveActivityPathGlob(&Actor{}, nil, "/docs", "../secret/*")
	if status != http.StatusBadRequest {
		t.Fatalf("expected 400, got %d", status)
	}
	if err == nil || !strings.Contains(err.Error(), "path traversal") {
		t.Fatalf("expected path traversal error, got %v", err)
	}
}
