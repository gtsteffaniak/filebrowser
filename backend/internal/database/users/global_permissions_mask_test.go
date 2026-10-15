package users

import (
	"encoding/json"
	"testing"
)

func TestGlobalPermissionsFieldMaskFromJSON(t *testing.T) {
	tests := []struct {
		name string
		raw  string
		want GlobalPermissionsFieldMask
	}{
		{name: "empty object", raw: `{}`, want: GlobalPermissionsFieldMask{}},
		{name: "null", raw: `null`, want: GlobalPermissionsFieldMask{}},
		{name: "admin only", raw: `{"admin":true}`, want: GlobalPermissionsFieldMask{Admin: true}},
		{name: "partial mixed case", raw: `{"Share":false,"api":true}`, want: GlobalPermissionsFieldMask{Api: true, Share: true}},
		{name: "explicit false still present", raw: `{"share":false}`, want: GlobalPermissionsFieldMask{Share: true}},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			got := GlobalPermissionsFieldMaskFromJSON(json.RawMessage(tc.raw))
			if got != tc.want {
				t.Fatalf("got %+v, want %+v", got, tc.want)
			}
		})
	}
}

func TestParseGlobalPermissionsFieldMaskFromCreateRequest(t *testing.T) {
	body := []byte(`{"data":{"username":"u","permissions":{"admin":true}}}`)
	got := ParseGlobalPermissionsFieldMaskFromCreateRequest(body)
	want := GlobalPermissionsFieldMask{Admin: true}
	if got != want {
		t.Fatalf("got %+v, want %+v", got, want)
	}

	omitted := []byte(`{"data":{"username":"u"}}`)
	if m := ParseGlobalPermissionsFieldMaskFromCreateRequest(omitted); m.Any() {
		t.Fatalf("expected empty mask when permissions omitted, got %+v", m)
	}
}

func TestMergeGlobalPermissionsOnCreate(t *testing.T) {
	defaults := Permissions{Admin: false, Api: true, Share: true, Realtime: true}

	t.Run("omitted permissions keep defaults", func(t *testing.T) {
		requested := Permissions{}
		got := MergeGlobalPermissionsOnCreate(defaults, requested, GlobalPermissionsFieldMask{})
		if got != defaults {
			t.Fatalf("got %+v, want defaults %+v", got, defaults)
		}
	})

	t.Run("partial mask merges only present keys", func(t *testing.T) {
		requested := Permissions{Admin: true, Api: false}
		mask := GlobalPermissionsFieldMask{Admin: true, Api: true}
		got := MergeGlobalPermissionsOnCreate(defaults, requested, mask)
		want := Permissions{Admin: true, Api: false, Share: true, Realtime: true}
		if got != want {
			t.Fatalf("got %+v, want %+v", got, want)
		}
	})

	t.Run("explicit false on masked field", func(t *testing.T) {
		requested := Permissions{Share: false}
		mask := GlobalPermissionsFieldMask{Share: true}
		got := MergeGlobalPermissionsOnCreate(defaults, requested, mask)
		want := Permissions{Admin: false, Api: true, Share: false, Realtime: true}
		if got != want {
			t.Fatalf("got %+v, want %+v", got, want)
		}
	})

	t.Run("programmatic all true without mask", func(t *testing.T) {
		requested := Permissions{Admin: true, Api: true, Share: true, Realtime: true}
		got := MergeGlobalPermissionsOnCreate(defaults, requested, GlobalPermissionsFieldMask{})
		if got != requested {
			t.Fatalf("got %+v, want %+v", got, requested)
		}
	})
}
