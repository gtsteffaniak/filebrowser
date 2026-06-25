package http

import (
	"net/http"
	"strings"
	"testing"

	activitydb "github.com/gtsteffaniak/filebrowser/backend/database/activity"
)

func TestResolveActivityPathGlobRejectsTraversal(t *testing.T) {
	_, status, err := resolveActivityPathGlob(&requestContext{}, nil, "/docs", "../secret/*")
	if status != http.StatusBadRequest {
		t.Fatalf("expected 400, got %d", status)
	}
	if err == nil || !strings.Contains(err.Error(), "path traversal") {
		t.Fatalf("expected path traversal error, got %v", err)
	}
}

func TestClampActivityListPagingUsesDefaultsAndMax(t *testing.T) {
	filter := activitydb.QueryFilter{Limit: 0, Page: 0}
	clampActivityListPaging(&filter)
	if filter.Limit != 100 {
		t.Fatalf("expected default limit 100, got %d", filter.Limit)
	}
	if filter.Page != 1 {
		t.Fatalf("expected page 1, got %d", filter.Page)
	}

	filter = activitydb.QueryFilter{Limit: 10000, Page: 2}
	clampActivityListPaging(&filter)
	if filter.Limit != 500 {
		t.Fatalf("expected max limit 500, got %d", filter.Limit)
	}
	if filter.Page != 2 {
		t.Fatalf("expected page unchanged at 2, got %d", filter.Page)
	}
}
