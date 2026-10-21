package activity

import (
	"net/http/httptest"
	"testing"
	"time"

	activitydb "github.com/gtsteffaniak/filebrowser/backend/internal/database/activity"
	"github.com/gtsteffaniak/filebrowser/backend/internal/database/share"
	"github.com/gtsteffaniak/filebrowser/backend/internal/database/users"
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

func TestShareFileEventsRecordedForShareOwner(t *testing.T) {
	store := &mockActivityStore{}
	globalMu.Lock()
	old := globalRecorder
	globalMu.Unlock()
	defer func() {
		globalMu.Lock()
		globalRecorder = old
		globalMu.Unlock()
	}()

	rec := &Recorder{
		store:         store,
		buffer:        make([]activitydb.Entry, 0, 4),
		flushCh:       make(chan struct{}, 1),
		stopCh:        make(chan struct{}),
		doneCh:        make(chan struct{}),
		maxBuffer:     10000,
		flushInterval: time.Hour,
		enabled:       true,
	}
	globalMu.Lock()
	globalRecorder = rec
	globalMu.Unlock()
	go rec.loop()

	actor := &Actor{
		User:  &users.User{FrontendUser: users.FrontendUser{Username: "anonymous"}},
		Share: share.Share{Hash: "share-hash", UserID: 7},
	}
	r := httptest.NewRequest("POST", "/public/api/resources", nil)
	RecordUpload(r, actor, "main", "/pub/new.txt", false)
	RecordDelete(r, actor, "main", "/pub/old.txt")
	RecordBulkDelete(r, actor, []BulkDeleteItem{{Source: "main", Path: "/pub/a.txt"}})
	loggedIn := &Actor{
		User:  &users.User{ID: 5, FrontendUser: users.FrontendUser{Username: "bob"}},
		Share: share.Share{Hash: "share-hash", UserID: 7},
	}
	RecordUpload(r, loggedIn, "main", "/pub/bob.txt", false)
	RecordFileModify(r, loggedIn, "main", "/pub/edited.txt")
	RecordFileModify(r, actor, "main", "/pub/anon-edit.txt")
	rec.Stop()

	var got []activitydb.Entry
	for _, batch := range store.inserts {
		got = append(got, batch...)
	}
	want := []activitydb.EventType{
		activitydb.EventUpload,
		activitydb.EventDelete,
		activitydb.EventBulkDelete,
		activitydb.EventUpload,
		activitydb.EventFileModify,
		activitydb.EventFileModify,
	}
	if len(got) != len(want) {
		t.Fatalf("expected %d entries, got %d: %+v", len(want), len(got), got)
	}
	for i, e := range got {
		if e.EventType != want[i] {
			t.Fatalf("entry %d: event type %q, want %q", i, e.EventType, want[i])
		}
	}
	for i := 0; i < 3; i++ {
		e := got[i]
		if e.UserID != 7 || e.Details.ShareHash != "share-hash" {
			t.Fatalf("anonymous share entry %d: userID=%d shareHash=%q", i, e.UserID, e.Details.ShareHash)
		}
	}
	if got[3].UserID != 5 {
		t.Fatalf("logged-in share visitor upload: userID=%d, want 5", got[3].UserID)
	}
	if got[4].UserID != 5 {
		t.Fatalf("logged-in share visitor modify: userID=%d, want 5", got[4].UserID)
	}
	if got[5].UserID != 7 || got[5].Details.ShareHash != "share-hash" {
		t.Fatalf("anonymous share modify: userID=%d shareHash=%q", got[5].UserID, got[5].Details.ShareHash)
	}
}

func assertFieldChange(t *testing.T, change activitydb.FieldChange, field, from, to string) {
	t.Helper()
	if change.Field != field || change.From != from || change.To != to {
		t.Fatalf("unexpected change: got {field:%q from:%q to:%q}, want {field:%q from:%q to:%q}",
			change.Field, change.From, change.To, field, from, to)
	}
}
