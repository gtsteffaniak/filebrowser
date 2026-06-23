package http

import (
	"net/http"
	"net/url"
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

func TestParseActivityStatusRange(t *testing.T) {
	min, max, err := parseActivityStatusRange(url.Values{})
	if err != nil || min != 0 || max != 0 {
		t.Fatalf("expected empty range, got min=%d max=%d err=%v", min, max, err)
	}

	min, max, err = parseActivityStatusRange(url.Values{
		"statusMin": []string{"200"},
		"statusMax": []string{"399"},
	})
	if err != nil || min != 200 || max != 399 {
		t.Fatalf("expected 200-399, got min=%d max=%d err=%v", min, max, err)
	}

	_, _, err = parseActivityStatusRange(url.Values{
		"statusMin": []string{"500"},
		"statusMax": []string{"400"},
	})
	if err == nil {
		t.Fatal("expected error when statusMax < statusMin")
	}

	_, _, err = parseActivityStatusRange(url.Values{"statusMin": []string{"99"}})
	if err == nil {
		t.Fatal("expected error for statusMin below 100")
	}

	_, _, err = parseActivityStatusRange(url.Values{"statusMax": []string{"600"}})
	if err == nil {
		t.Fatal("expected error for statusMax above 599")
	}
}

func TestValidateActivityChartParamsRejectsOutcomeWithStatusFilter(t *testing.T) {
	filter := activitydb.QueryFilter{
		From:      0,
		To:        86400,
		SplitBy:   "outcome",
		StatusMin: 200,
		StatusMax: 399,
	}
	if err := validateActivityChartParams(filter); err == nil {
		t.Fatal("expected error when splitBy outcome is combined with status filter")
	}
}
