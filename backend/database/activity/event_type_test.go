package activity

import "testing"

func TestEventTypeValid(t *testing.T) {
	for _, et := range AllEventTypes {
		if !et.Valid() {
			t.Errorf("expected %q to be valid", et)
		}
	}
	if EventType("notARealEvent").Valid() {
		t.Error("expected unknown event type to be invalid")
	}
}

func TestResolveScopeEventTypes(t *testing.T) {
	got, err := ResolveScopeEventTypes("shares", nil)
	if err != nil || got != nil {
		t.Fatalf("shares default: got %v, %v; want nil, nil", got, err)
	}
	got, err = ResolveScopeEventTypes("shares", []EventType{EventDownload})
	if err != nil {
		t.Fatal(err)
	}
	if len(got) != 1 || got[0] != EventDownload {
		t.Fatalf("shares explicit download: got %v", got)
	}
	got, err = ResolveScopeEventTypes("files", nil)
	if err != nil {
		t.Fatal(err)
	}
	if len(got) != len(FileEventTypes) {
		t.Fatalf("files default: got %d types, want %d", len(got), len(FileEventTypes))
	}
}

func TestEventTypeFromAction(t *testing.T) {
	cases := []struct {
		action string
		want   EventType
		ok     bool
	}{
		{"copy", EventCopy, true},
		{"move", EventMove, true},
		{"rename", EventRename, true},
		{"delete", "", false},
	}
	for _, tc := range cases {
		got, ok := EventTypeFromAction(tc.action)
		if ok != tc.ok || got != tc.want {
			t.Errorf("EventTypeFromAction(%q) = (%q, %v), want (%q, %v)", tc.action, got, ok, tc.want, tc.ok)
		}
	}
}
