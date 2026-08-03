package settings

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/gtsteffaniak/filebrowser/backend/internal/database/users"
)

const (
	proxyPlaywrightConfig = "../../../_docker/src/proxy/backend/config.yaml"
	jwtPlaywrightConfig   = "../../../_docker/src/jwt/backend/config.yaml"
)

func TestYAMLConfig_proxy_newStyleUserDefaults(t *testing.T) {
	loadConfigForUserDefaultsTests(t, proxyPlaywrightConfig)

	if !Config.UserDefaults.Account.Permissions.Share {
		t.Fatal("expected account.permissions.share true from organized userDefaults")
	}
	if len(Config.Server.Sources) != 1 {
		t.Fatalf("sources: %d", len(Config.Server.Sources))
	}
	src := Config.Server.Sources[0]
	if !src.Config.CreateUserDir || !src.Config.DefaultEnabled {
		t.Fatalf("source config: createUserDir=%v defaultEnabled=%v", src.Config.CreateUserDir, src.Config.DefaultEnabled)
	}
	p := src.Config.DefaultPermissions
	if !p.View || !p.Download || !p.Modify || !p.Create {
		t.Fatalf("source defaultPermissions: %+v", p)
	}
	ud := Config.UserDefaults
	if ud.Preview.Image == nil || !*ud.Preview.Image || ud.Preview.Video == nil || !*ud.Preview.Video {
		t.Fatalf("expected preview thumbnail defaults true when omitted from yaml: image=%v video=%v", ud.Preview.Image, ud.Preview.Video)
	}
	if ud.FileViewer.AutoplayMedia == nil || !*ud.FileViewer.AutoplayMedia {
		t.Fatalf("expected fileViewer.autoplayMedia true when omitted from yaml: %v", ud.FileViewer.AutoplayMedia)
	}
}

func TestYAMLConfig_proxy_sourceAccessDefaultsMatchYAML(t *testing.T) {
	loadConfigForUserDefaultsTests(t, proxyPlaywrightConfig)
	got := DefaultSourceFilePermissions()
	if !got.View || !got.Download || !got.Modify || !got.Create {
		t.Fatalf("DefaultSourceFilePermissions: %+v", got)
	}
}

func TestYAMLConfig_jwt_organizedUserDefaults(t *testing.T) {
	loadConfigForUserDefaultsTests(t, jwtPlaywrightConfig)

	ud := Config.UserDefaults
	if !ud.Account.Permissions.Share {
		t.Fatal("expected account.permissions.share true")
	}
	if ud.UI.DarkMode == nil || !*ud.UI.DarkMode {
		t.Fatal("expected ui.darkMode true")
	}
	if ud.Listing.SingleClick {
		t.Fatal("expected listing.singleClick false")
	}
	if ud.Preview.Image == nil || !*ud.Preview.Image || ud.Preview.PopUp == nil || !*ud.Preview.PopUp {
		t.Fatalf("preview flags: image=%v popup=%v", ud.Preview.Image, ud.Preview.PopUp)
	}
	got := DefaultSourceFilePermissions()
	if !got.Modify || !got.Create || !got.Download {
		t.Fatalf("source defaultPermissions from jwt yaml: %+v", got)
	}
}

func TestYAMLConfig_validConfig_organizedPermissions(t *testing.T) {
	loadConfigForUserDefaultsTests(t, "./validConfig.yaml")
	if !Config.UserDefaults.Account.Permissions.Admin {
		t.Fatal("validConfig account.permissions.admin should be true")
	}
	if !Config.UserDefaults.Account.Permissions.Share || !Config.UserDefaults.Account.Permissions.Api {
		t.Fatalf("account permissions: %+v", Config.UserDefaults.Account.Permissions)
	}
	if !Config.UserDefaults.Listing.SingleClick {
		t.Fatal("expected listing.singleClick true")
	}
}

func TestYAMLConfig_applyUserDefaultsFrom_proxyAndJwt(t *testing.T) {
	t.Run("proxy", func(t *testing.T) {
		loadConfigForUserDefaultsTests(t, proxyPlaywrightConfig)
		u := &users.User{FrontendUser: users.FrontendUser{Username: "demo-127.0.0.1", LoginMethod: users.LoginMethodProxy}}
		ApplyUserDefaultsFrom(u, Config.UserDefaults)
		if !u.Permissions.Share {
			t.Fatal("Share=false after ApplyUserDefaultsFrom")
		}
		if len(u.BackendScopes) != 1 || u.BackendScopes[0].Scope != "/demo-127.0.0.1" {
			t.Fatalf("createUserDir scope: %+v", u.BackendScopes)
		}
		perms := u.BackendScopes[0].Permissions
		if !perms.View || !perms.Modify || !perms.Create || !perms.Download {
			t.Fatalf("scope permissions: %+v", perms)
		}
	})

	t.Run("jwt", func(t *testing.T) {
		loadConfigForUserDefaultsTests(t, jwtPlaywrightConfig)
		u := &users.User{FrontendUser: users.FrontendUser{Username: "testuser", LoginMethod: users.LoginMethodJwt}}
		ApplyUserDefaultsFrom(u, Config.UserDefaults)
		if !u.Permissions.Share {
			t.Fatal("Share=false after ApplyUserDefaultsFrom")
		}
		if !u.DarkMode {
			t.Fatal("expected darkMode from ui defaults")
		}
		if !u.Preview.Image || !u.Preview.PopUp {
			t.Fatalf("preview: %+v", u.Preview)
		}
		if u.SingleClick {
			t.Fatal("expected listing.singleClick false on user")
		}
		if u.BackendScopes[0].Scope != "/testuser" {
			t.Fatalf("scope=%q", u.BackendScopes[0].Scope)
		}
	})
}

func TestYAMLConfig_profileMerge_matchesApplyUserDefaults(t *testing.T) {
	loadConfigForUserDefaultsTests(t, jwtPlaywrightConfig)
	fromDefaults := ProfileFromUserDefaults(Config.UserDefaults)
	u := &users.User{FrontendUser: users.FrontendUser{Username: "merge-check"}}
	ExpandProfileIntoUser(u, fromDefaults)
	if !u.DarkMode || !u.Preview.Image {
		t.Fatalf("ExpandProfileIntoUser: darkMode=%v preview.image=%v", u.DarkMode, u.Preview.Image)
	}
	u2 := &users.User{FrontendUser: users.FrontendUser{Username: "merge-check"}}
	ApplyFullProfileFromDefaults(u2, Config.UserDefaults)
	if u2.DarkMode != u.DarkMode || u2.Preview.Image != u.Preview.Image {
		t.Fatalf("ApplyFullProfileFromDefaults diverged from ExpandProfileIntoUser")
	}
}

func TestYAMLConfig_seedSourcePermissionsAfterMigration(t *testing.T) {
	loadConfigForUserDefaultsTests(t, proxyPlaywrightConfig)
	defaults := DefaultSourceFilePermissions()
	user := &users.User{
		Version: users.SourcePermissionsMigrationVersion,
		BackendScopes: []users.BackendScope{
			{Path: Config.Server.Sources[0].Path, Scope: "/extra"},
		},
	}
	if !users.SeedSourcePermissionsForPath(user, Config.Server.Sources[0].Path, defaults) {
		t.Fatal("expected seed")
	}
	p := user.BackendScopes[0].Permissions
	if !p.Modify || !p.Create {
		t.Fatalf("seeded permissions: %+v", p)
	}
}

func TestLoadConfig_partialUserDefaults_preservesPreviewAndAutoplayDefaults(t *testing.T) {
	loadPartialUserDefaultsConfig(t, partialUserDefaultsYAML(""))
	assertPreviewDefaultsTrue(t, Config.UserDefaults)
	assertAutoplayDefaultTrue(t, Config.UserDefaults)
	if !Config.UserDefaults.Account.Permissions.Share {
		t.Fatal("expected account.permissions.share true from config")
	}
}

func TestLoadConfig_partialUserDefaults_preservesOtherSetDefaults(t *testing.T) {
	loadPartialUserDefaultsConfig(t, partialUserDefaultsYAML(""))
	ud := Config.UserDefaults
	if ud.UI.DarkMode == nil || !*ud.UI.DarkMode {
		t.Fatalf("ui.darkMode: %v", ud.UI.DarkMode)
	}
	if ud.Sidebar.ShowTools == nil || !*ud.Sidebar.ShowTools {
		t.Fatalf("sidebar.showTools: %v", ud.Sidebar.ShowTools)
	}
	if !ud.Listing.DeleteAfterArchive {
		t.Fatal("expected listing.deleteAfterArchive true")
	}
	if !ud.Sidebar.Sticky {
		t.Fatal("expected sidebar.sticky true")
	}
}

func TestLoadConfig_partialUserDefaults_explicitFalseOverride(t *testing.T) {
	loadPartialUserDefaultsConfig(t, partialUserDefaultsYAML(`
  preview:
    image: false
  fileViewer:
    autoplayMedia: false
`))
	ud := Config.UserDefaults
	if ud.Preview.Image == nil || *ud.Preview.Image {
		t.Fatalf("preview.image: %v", ud.Preview.Image)
	}
	if ud.Preview.Video == nil || !*ud.Preview.Video {
		t.Fatalf("preview.video should stay true: %v", ud.Preview.Video)
	}
	if ud.FileViewer.AutoplayMedia == nil || *ud.FileViewer.AutoplayMedia {
		t.Fatalf("fileViewer.autoplayMedia: %v", ud.FileViewer.AutoplayMedia)
	}
}

func partialUserDefaultsYAML(extra string) string {
	return `server:
  sources:
    - path: "."
userDefaults:
  account:
    permissions:
      share: true
` + extra
}

func loadPartialUserDefaultsConfig(t *testing.T, content string) {
	t.Helper()
	testDir := t.TempDir()
	configFile := filepath.Join(testDir, "config.yaml")
	if err := os.WriteFile(configFile, []byte(content), 0644); err != nil {
		t.Fatalf("write config: %v", err)
	}
	if err := LoadConfigWithDefaultsForTest(configFile); err != nil {
		t.Fatalf("LoadConfigWithDefaultsForTest: %v", err)
	}
}

func assertPreviewDefaultsTrue(t *testing.T, ud UserDefaults) {
	t.Helper()
	check := func(name string, ptr *bool) {
		if ptr == nil || !*ptr {
			t.Fatalf("preview.%s: %v", name, ptr)
		}
	}
	check("image", ud.Preview.Image)
	check("video", ud.Preview.Video)
	check("audio", ud.Preview.Audio)
	check("popup", ud.Preview.PopUp)
	check("office", ud.Preview.Office)
	check("folder", ud.Preview.Folder)
	check("models", ud.Preview.Models)
	check("motionVideoPreview", ud.Preview.MotionVideoPreview)
	check("highQuality", ud.Preview.HighQuality)
}

func assertAutoplayDefaultTrue(t *testing.T, ud UserDefaults) {
	t.Helper()
	if ud.FileViewer.AutoplayMedia == nil || !*ud.FileViewer.AutoplayMedia {
		t.Fatalf("fileViewer.autoplayMedia: %v", ud.FileViewer.AutoplayMedia)
	}
}
