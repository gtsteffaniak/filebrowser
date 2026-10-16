package state

import (
	"testing"

	"github.com/gtsteffaniak/filebrowser/backend/pkg/settings"
)

// The legacy data-saver bitrate migration must run once. It matches "720p + 0
// kbps", which an admin may set deliberately, so re-running it on every startup
// would silently overwrite that choice.
func TestMediaDataSaverMigrationRunsOnce(t *testing.T) {
	initSidebarLinkTestDB(t)

	legacy := settings.SanitizeMediaSettings(settings.CurrentMediaSettings())
	legacy.Transcode.Profiles.DataSaver = settings.TranscodeProfile{
		Enabled:        true,
		MaxResolution:  720,
		AvgBitrateKbps: 0,
	}
	if err := sqlDb.SaveSetting(mediaSettingsDefaultKey, legacy); err != nil {
		t.Fatal(err)
	}
	// InitMediaSettings already marked the migration done during setup.
	if err := sqlDb.DeleteSetting(mediaSettingsMigrationKey); err != nil {
		t.Fatal(err)
	}

	if err := InitMediaSettings(); err != nil {
		t.Fatal(err)
	}
	if got := GetMediaSettings().Transcode.Profiles.DataSaver.AvgBitrateKbps; got != 1500 {
		t.Fatalf("legacy bitrate migration did not apply: avgBitrateKbps = %d, want 1500", got)
	}

	// An admin setting the bitrate back to 0 ("keep the source bitrate") must
	// survive the next startup.
	adminChoice := GetMediaSettings()
	adminChoice.Transcode.Profiles.DataSaver.AvgBitrateKbps = 0
	if err := sqlDb.SaveSetting(mediaSettingsDefaultKey, adminChoice); err != nil {
		t.Fatal(err)
	}
	if err := InitMediaSettings(); err != nil {
		t.Fatal(err)
	}
	if got := GetMediaSettings().Transcode.Profiles.DataSaver.AvgBitrateKbps; got != 0 {
		t.Fatalf("migration overwrote the admin's choice: avgBitrateKbps = %d, want 0", got)
	}
}

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

// A fresh install seeds from config and records the marker without rewriting the
// stored data-saver profile.
func TestMediaMigrationMarkerIsRecordedOnFirstInit(t *testing.T) {
	initSidebarLinkTestDB(t)

	migrations := loadMediaMigrations()
	if !migrations.DataSaverBitrate {
		t.Fatal("expected the data-saver migration to be marked as applied after init")
	}
	if got := GetMediaSettings().Transcode.Profiles.DataSaver.MaxResolution; got != 720 {
		t.Fatalf("data-saver maxResolution = %d, want the 720 default", got)
	}
}
