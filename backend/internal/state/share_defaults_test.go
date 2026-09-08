package state

import (
	"errors"
	"path/filepath"
	"testing"

	"github.com/gtsteffaniak/filebrowser/backend/pkg/settings"
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

func TestPatchShareDefaultsCombined_rollsBackBothOnEnforcedSaveFailure(t *testing.T) {
	t.Setenv("FILEBROWSER_ONLYOFFICE_SECRET", "")
	settings.Initialize("../../../_docker/src/noauth/backend/config.yaml")
	settings.Env.IsPlaywright = true

	dbPath := filepath.Join(t.TempDir(), "filebrowser.sqlite")
	if _, err := Initialize(dbPath); err != nil {
		t.Fatal(err)
	}

	if err := PatchShareDefaults([]byte(`{"shareType":"view","allowCreate":false}`)); err != nil {
		t.Fatal(err)
	}
	if err := PatchShareDefaultsEnforced([]byte(`{"allowModify":false}`)); err != nil {
		t.Fatal(err)
	}

	valuesBefore := GetShareDefaults()
	enforcedBefore := GetEnforcedShareDefaults()

	errInjectShareDefaultsEnforcedSave = errors.New("simulated enforced save failure")
	t.Cleanup(func() {
		errInjectShareDefaultsEnforcedSave = nil
	})

	err := PatchShareDefaultsCombined(
		[]byte(`{"shareType":"upload","allowCreate":true}`),
		[]byte(`{"allowModify":true}`),
	)
	if err == nil {
		t.Fatal("expected combined patch failure")
	}
	if !IsShareDefaultsPersistenceError(err) {
		t.Fatalf("expected persistence error, got %v", err)
	}

	if got := GetShareDefaults(); got.ShareType != valuesBefore.ShareType || got.AllowCreate != valuesBefore.AllowCreate {
		t.Fatalf("in-memory values changed after failed patch: got shareType=%q allowCreate=%v want shareType=%q allowCreate=%v",
			got.ShareType, got.AllowCreate, valuesBefore.ShareType, valuesBefore.AllowCreate)
	}
	if got := GetEnforcedShareDefaults(); got.AllowModify != enforcedBefore.AllowModify {
		t.Fatalf("in-memory enforced changed after failed patch: got allowModify=%v want %v", got.AllowModify, enforcedBefore.AllowModify)
	}

	if err := Close(); err != nil {
		t.Fatal(err)
	}
	if _, err := Initialize(dbPath); err != nil {
		t.Fatal(err)
	}

	if got := GetShareDefaults(); got.ShareType != valuesBefore.ShareType || got.AllowCreate != valuesBefore.AllowCreate {
		t.Fatalf("persisted values changed after failed patch: got shareType=%q allowCreate=%v want shareType=%q allowCreate=%v",
			got.ShareType, got.AllowCreate, valuesBefore.ShareType, valuesBefore.AllowCreate)
	}
	if got := GetEnforcedShareDefaults(); got.AllowModify != enforcedBefore.AllowModify {
		t.Fatalf("persisted enforced changed after failed patch: got allowModify=%v want %v", got.AllowModify, enforcedBefore.AllowModify)
	}
}
