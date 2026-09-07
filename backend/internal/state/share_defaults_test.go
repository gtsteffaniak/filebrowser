package state

import (
	"testing"
)

func TestPatchShareDefaults_persistsValues(t *testing.T) {
	initSidebarLinkTestDB(t)

	if err := PatchShareDefaults([]byte(`{"shareType":"upload","allowCreate":true}`)); err != nil {
		t.Fatal(err)
	}
	got := GetShareDefaults()
	if got.ShareType != "upload" {
		t.Fatalf("shareType=%q want upload", got.ShareType)
	}
	if !got.AllowCreate {
		t.Fatal("expected allowCreate true")
	}
}

func TestPatchShareDefaultsEnforced_persistsFlags(t *testing.T) {
	initSidebarLinkTestDB(t)

	if err := PatchShareDefaultsEnforced([]byte(`{"allowModify":true}`)); err != nil {
		t.Fatal(err)
	}
	got := GetEnforcedShareDefaults()
	if !got.AllowModify {
		t.Fatal("expected allowModify enforced")
	}
}

func TestPatchShareDefaultsCombined_persistsBoth(t *testing.T) {
	initSidebarLinkTestDB(t)

	if err := PatchShareDefaultsCombined(
		[]byte(`{"shareType":"upload","allowCreate":true}`),
		[]byte(`{"allowModify":true}`),
	); err != nil {
		t.Fatal(err)
	}
	values := GetShareDefaults()
	if values.ShareType != "upload" {
		t.Fatalf("shareType=%q want upload", values.ShareType)
	}
	enforced := GetEnforcedShareDefaults()
	if !enforced.AllowModify {
		t.Fatal("expected allowModify enforced")
	}
}
