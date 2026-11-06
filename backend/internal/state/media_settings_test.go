package state

import (
	"encoding/json"
	"testing"

	"github.com/gtsteffaniak/filebrowser/backend/pkg/settings"
)

// GetMediaSettings must hand out an independent copy. The conversion maps hold
// *bool values, so a caller editing the result would otherwise reconfigure the
// stored settings (and the live config they are applied to).
func TestGetMediaSettingsReturnsIndependentCopy(t *testing.T) {
	initSidebarLinkTestDB(t)

	snapshot := GetMediaSettings()
	if len(snapshot.Convert.ImagePreview) == 0 {
		t.Fatal("expected seeded image preview settings")
	}

	// Replace an entry and flip a value through its pointer.
	flipped := false
	snapshot.Convert.ImagePreview[settings.JPEGImagePreview] = &flipped
	delete(snapshot.Convert.ImagePreview, settings.HEICImagePreview)

	stored := GetMediaSettings()
	if !*stored.Convert.ImagePreview[settings.JPEGImagePreview] {
		t.Fatal("editing a snapshot entry changed the stored settings")
	}
	if _, ok := stored.Convert.ImagePreview[settings.HEICImagePreview]; !ok {
		t.Fatal("deleting from a snapshot changed the stored settings")
	}
	if stored.Convert.ImagePreview[settings.JPEGImagePreview] == snapshot.Convert.ImagePreview[settings.JPEGImagePreview] {
		t.Fatal("two reads share a *bool pointer")
	}
}

// Derived read-time values must never reach the database, even when a caller
// passes a struct that has them populated.
func TestSaveMediaSettingsNeverPersistsDerivedFields(t *testing.T) {
	initSidebarLinkTestDB(t)

	ms := GetMediaSettings()
	ms.Transcode.EnabledProfiles = []string{"quality"}
	ms.ImagePreview = map[settings.ImagePreviewType]bool{settings.JPEGImagePreview: true}
	if err := saveMediaSettings(mediaSettingsDefaultKey, ms); err != nil {
		t.Fatalf("saveMediaSettings: %v", err)
	}

	raw, err := sqlDb.GetSetting(mediaSettingsDefaultKey)
	if err != nil {
		t.Fatalf("GetSetting: %v", err)
	}
	var stored map[string]any
	if err := json.Unmarshal(raw, &stored); err != nil {
		t.Fatalf("decode stored settings: %v", err)
	}
	transcode, ok := stored["transcode"].(map[string]any)
	if !ok {
		t.Fatalf("stored transcode = %T, want object", stored["transcode"])
	}
	if _, ok := transcode["enabledProfiles"]; ok {
		t.Fatal("enabledProfiles (derived) was persisted")
	}
	if _, ok := stored["imagePreview"]; ok {
		t.Fatal("imagePreview (derived) was persisted")
	}
}

// The admin UI echoes its whole payload back on PATCH, derived fields included.
// Those must be dropped before the value reaches memory, not only before the database.
func TestPatchMediaSettingsDropsEchoedDerivedFields(t *testing.T) {
	initSidebarLinkTestDB(t)

	patch := []byte(`{"imagePreview":{"jpeg":true},"transcode":{"enabledProfiles":["quality"]}}`)
	if err := PatchMediaSettings(patch); err != nil {
		t.Fatalf("PatchMediaSettings: %v", err)
	}
	got := GetMediaSettings()
	if got.ImagePreview != nil {
		t.Fatal("echoed imagePreview stayed in memory")
	}
	if got.Transcode.EnabledProfiles != nil {
		t.Fatal("echoed enabledProfiles stayed in memory")
	}
}

// A fresh install seeds the stored media settings from config.
func TestMediaSettingsSeedFromConfigOnFirstInit(t *testing.T) {
	initSidebarLinkTestDB(t)

	if got := GetMediaSettings().Transcode.Profiles.DataSaver.MaxResolution; got != 480 {
		t.Fatalf("data-saver maxResolution = %d, want the 480 default", got)
	}
}
