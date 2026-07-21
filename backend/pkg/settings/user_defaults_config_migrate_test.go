package settings

import "testing"

func TestMigrateUserDefaultsConfig_flatToNested(t *testing.T) {
	raw := map[string]interface{}{
		"hideFilesInTree": true,
		"stickySidebar":   true,
		"darkMode":        true,
		"permissions": map[string]interface{}{
			"share":    true,
			"modify":   true,
			"download": true,
		},
		"preview": map[string]interface{}{
			"disableHideSidebar": true,
			"image":              true,
			"autoplayMedia":      false,
		},
	}
	got, err := MigrateUserDefaultsConfig(raw)
	if err != nil {
		t.Fatal(err)
	}
	if !got.Changed {
		t.Fatal("expected changed=true")
	}
	sidebar, ok := got.UserDefaults["sidebar"].(map[string]interface{})
	if !ok || sidebar["hideFiles"] != true || sidebar["sticky"] != true || sidebar["disableHideOnPreview"] != true {
		t.Fatalf("sidebar=%#v", got.UserDefaults["sidebar"])
	}
	ui, ok := got.UserDefaults["ui"].(map[string]interface{})
	if !ok || ui["darkMode"] != true {
		t.Fatalf("ui=%#v", got.UserDefaults["ui"])
	}
	account, ok := got.UserDefaults["account"].(map[string]interface{})
	if !ok {
		t.Fatalf("account=%#v", got.UserDefaults["account"])
	}
	perms, ok := account["permissions"].(map[string]interface{})
	if !ok || perms["share"] != true {
		t.Fatalf("permissions=%#v", account["permissions"])
	}
	if len(got.Warnings) == 0 {
		t.Fatal("expected warnings for legacy source permissions")
	}
	preview, ok := got.UserDefaults["preview"].(map[string]interface{})
	if !ok || preview["image"] != true {
		t.Fatalf("preview=%#v", got.UserDefaults["preview"])
	}
	fileViewer, ok := got.UserDefaults["fileViewer"].(map[string]interface{})
	if !ok || fileViewer["autoplayMedia"] != false {
		t.Fatalf("fileViewer=%#v", got.UserDefaults["fileViewer"])
	}
}

func TestNeedsUserDefaultsConfigMigration(t *testing.T) {
	if !NeedsUserDefaultsConfigMigration(map[string]interface{}{"hideFilesInTree": true}) {
		t.Fatal("expected legacy flat key to need migration")
	}
	if NeedsUserDefaultsConfigMigration(map[string]interface{}{
		"sidebar": map[string]interface{}{"hideFiles": true},
	}) {
		t.Fatal("expected nested config not to need migration")
	}
}
