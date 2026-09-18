package auth_test

import (
	"net/http"
	"testing"

	"github.com/gtsteffaniak/filebrowser/backend/internal/auth"
)

func TestExtractGroupsFromHeader_SingleValue(t *testing.T) {
	req, err := http.NewRequest(http.MethodGet, "/", http.NoBody)
	if err != nil {
		t.Fatal(err)
	}
	req.Header.Set("X-Cosmos-Role", "2")

	groups, present := auth.ExtractGroupsFromHeader(req, "x-cosmos-role")
	if !present {
		t.Fatal("expected groups to be present")
	}
	if len(groups) != 1 || groups[0] != "2" {
		t.Fatalf("groups = %#v, want [\"2\"]", groups)
	}
}

func TestExtractGroupsFromHeader_CommaSeparated(t *testing.T) {
	req, err := http.NewRequest(http.MethodGet, "/", http.NoBody)
	if err != nil {
		t.Fatal(err)
	}
	req.Header.Set("X-Groups", "1, 2")

	groups, present := auth.ExtractGroupsFromHeader(req, "X-Groups")
	if !present {
		t.Fatal("expected groups to be present")
	}
	if len(groups) != 2 || groups[0] != "1" || groups[1] != "2" {
		t.Fatalf("groups = %#v, want [\"1\", \"2\"]", groups)
	}
}

func TestExtractGroupsFromHeader_JSONArray(t *testing.T) {
	req, err := http.NewRequest(http.MethodGet, "/", http.NoBody)
	if err != nil {
		t.Fatal(err)
	}
	req.Header.Set("X-Groups", `["admin","users"]`)

	groups, present := auth.ExtractGroupsFromHeader(req, "X-Groups")
	if !present {
		t.Fatal("expected groups to be present")
	}
	if len(groups) != 2 || groups[0] != "admin" || groups[1] != "users" {
		t.Fatalf("groups = %#v, want [\"admin\", \"users\"]", groups)
	}
}

func TestExtractGroupsFromHeader_Missing(t *testing.T) {
	req, err := http.NewRequest(http.MethodGet, "/", http.NoBody)
	if err != nil {
		t.Fatal(err)
	}

	groups, present := auth.ExtractGroupsFromHeader(req, "X-Groups")
	if present {
		t.Fatalf("expected groups to be absent, got %#v", groups)
	}
}

func TestExtractGroupsFromHeader_EmptyHeaderName(t *testing.T) {
	req, err := http.NewRequest(http.MethodGet, "/", http.NoBody)
	if err != nil {
		t.Fatal(err)
	}
	req.Header.Set("X-Groups", "admin")

	groups, present := auth.ExtractGroupsFromHeader(req, "")
	if present {
		t.Fatalf("expected groups to be absent, got %#v", groups)
	}
}
