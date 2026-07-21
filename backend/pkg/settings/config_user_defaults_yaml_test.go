package settings

import (
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
