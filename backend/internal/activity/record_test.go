package activity

import (
	"testing"

	activitydb "github.com/gtsteffaniak/filebrowser/backend/internal/database/activity"
)

func TestAccessRuleCreateChanges(t *testing.T) {
	changes := AccessRuleCreateChanges(false, "user", "admin")
	if len(changes) != 3 {
		t.Fatalf("expected 3 changes, got %d", len(changes))
	}
	assertFieldChange(t, changes[0], "ruleType", "", "deny")
	assertFieldChange(t, changes[1], "ruleCategory", "", "user")
	assertFieldChange(t, changes[2], "value", "", "admin")
}

func TestAccessRuleDeleteChanges(t *testing.T) {
	changes := AccessRuleDeleteChanges("deny", "user", "admin", true, 3)
	if len(changes) != 5 {
		t.Fatalf("expected 5 changes, got %d", len(changes))
	}
	assertFieldChange(t, changes[0], "ruleType", "", "deny")
	assertFieldChange(t, changes[1], "ruleCategory", "", "user")
	assertFieldChange(t, changes[2], "value", "", "admin")
	assertFieldChange(t, changes[3], "cascade", "", "true")
	assertFieldChange(t, changes[4], "count", "", "3")
}

func assertFieldChange(t *testing.T, change activitydb.FieldChange, field, from, to string) {
	t.Helper()
	if change.Field != field || change.From != from || change.To != to {
		t.Fatalf("unexpected change: got {field:%q from:%q to:%q}, want {field:%q from:%q to:%q}",
			change.Field, change.From, change.To, field, from, to)
	}
}
