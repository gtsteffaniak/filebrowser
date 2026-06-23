package activity

import (
	"encoding/json"
	"testing"
)

func TestPrepForFrontendPromotesAuthWithoutDuplicatingDetails(t *testing.T) {
	entry := Entry{
		ID:        1,
		CreatedAt: 1700000000,
		EventType: EventUserUpdate,
		Details: Details{
			TargetUsername: "admin",
			TokenName:      "unique",
			AuthMethod:     "apiKey",
			Changes: []FieldChange{{
				Field: "quickDownload",
				From:  "true",
				To:    "false",
			}},
		},
	}

	fe := entry.PrepForFrontend("admin")
	if fe.TokenName != "unique" {
		t.Fatalf("TokenName = %q, want unique", fe.TokenName)
	}
	if fe.AuthMethod != "apiKey" {
		t.Fatalf("AuthMethod = %q, want apiKey", fe.AuthMethod)
	}
	detailsJSON, err := json.Marshal(fe.Details)
	if err != nil {
		t.Fatal(err)
	}
	var details map[string]any
	if err := json.Unmarshal(detailsJSON, &details); err != nil {
		t.Fatal(err)
	}
	if _, ok := details["tokenName"]; ok {
		t.Fatalf("details must not include tokenName: %s", detailsJSON)
	}
	if _, ok := details["authMethod"]; ok {
		t.Fatalf("details must not include authMethod: %s", detailsJSON)
	}
	if len(fe.Details.Changes) != 1 {
		t.Fatalf("expected 1 change in details, got %d", len(fe.Details.Changes))
	}
}
