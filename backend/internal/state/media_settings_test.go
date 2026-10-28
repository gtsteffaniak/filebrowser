package state

import (
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

// A fresh install seeds the stored media settings from config.
func TestMediaSettingsSeedFromConfigOnFirstInit(t *testing.T) {
	initSidebarLinkTestDB(t)

	if got := GetMediaSettings().Transcode.Profiles.DataSaver.MaxResolution; got != 480 {
		t.Fatalf("data-saver maxResolution = %d, want the 480 default", got)
	}
}
