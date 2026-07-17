package cmd

import (
	"encoding/json"
	"os"
	"path/filepath"
	"testing"

	storm "github.com/asdine/storm/v3"
	"github.com/gtsteffaniak/filebrowser/backend/internal/database/access"
	"github.com/gtsteffaniak/filebrowser/backend/internal/database/share"
	"github.com/gtsteffaniak/filebrowser/backend/internal/database/users"
)

// TestSeedMigrationFixture writes a BoltDB fixture for manual inspection or regeneration.
// It never writes to the committed database.db.old unless SEED_MIGRATION_FIXTURE_OUT is set explicitly.
// Example: SEED_MIGRATION_FIXTURE=1 SEED_MIGRATION_FIXTURE_OUT=/tmp/database.db.old go test ./cmd -run TestSeedMigrationFixture -count=1
func TestSeedMigrationFixture(t *testing.T) {
	if os.Getenv("SEED_MIGRATION_FIXTURE") == "" {
		t.Skip("set SEED_MIGRATION_FIXTURE=1 to write a BoltDB fixture")
	}

	outPath := os.Getenv("SEED_MIGRATION_FIXTURE_OUT")
	if outPath == "" {
		outPath = filepath.Join(t.TempDir(), "database.db.old")
	} else {
		var err error
		outPath, err = filepath.Abs(outPath)
		if err != nil {
			t.Fatal(err)
		}
		committed, err := filepath.Abs(filepath.Join("..", "..", "_docker", "src", "settings", "backend", "database.db.old"))
		if err != nil {
			t.Fatal(err)
		}
		if outPath == committed {
			t.Fatal("refusing to overwrite committed database.db.old; choose a different SEED_MIGRATION_FIXTURE_OUT")
		}
	}
	if err := os.Remove(outPath); err != nil && !os.IsNotExist(err) {
		t.Fatal(err)
	}

	db, err := storm.Open(outPath)
	if err != nil {
		t.Fatal(err)
	}
	defer db.Close()

	adminSidebar := []users.SidebarLink{
		{Name: "playwright + files", Category: "source", Target: "/", SourceName: fixturePlaywrightSource},
		{Name: "docker", Category: "source", Target: "/", SourceName: fixtureDockerSource},
		{Name: "access", Category: "source", Target: "/", SourceName: fixtureAccessSource},
	}

	adminScopes := []users.FrontendScope{
		{Name: fixturePlaywrightSource, Scope: "/"},
		{Name: fixtureDockerSource, Scope: "/"},
		{Name: fixtureAccessSource, Scope: "/"},
	}

	adminTokens := map[string]users.AuthToken{
		"customized": {
			Name:      "customized",
			Token:     "eyJhbGciOiJIUzI1NiIsInR5cCI6IkpXVCJ9.eyJpc3MiOiJGaWxlQnJvd3NlciBRdWFudHVtIiwiZXhwIjoxODE1NDE4MTk0LCJpYXQiOjE3ODQzMTQxOTQsImJlbG9uZ3NUbyI6MSwiUGVybWlzc2lvbnMiOnsiYXBpIjp0cnVlLCJhZG1pbiI6dHJ1ZSwic2hhcmUiOnRydWUsInJlYWx0aW1lIjpmYWxzZX19.dGhlZ3lVuG-mD1tbKM-B1qgsuTW6Lz0vfoZ-Z1QQLTo",
			BelongsTo: 1,
			Permissions: users.Permissions{
				Admin:    true,
				Api:      true,
				Share:    true,
				Realtime: false,
			},
		},
		"full": {
			Name:  "full",
			Token: "eyJhbGciOiJIUzI1NiIsInR5cCI6IkpXVCJ9.eyJpc3MiOiJGaWxlQnJvd3NlciBRdWFudHVtIiwiZXhwIjoxODE1NDE4MTU3LCJpYXQiOjE3ODQzMTQxNTcsIlBlcm1pc3Npb25zIjp7ImFwaSI6ZmFsc2UsImFkbWluIjpmYWxzZSwibW9kaWZ5IjpmYWxzZSwic2hhcmUiOmZhbHNlLCJyZWFsdGltZSI6ZmFsc2UsImRlbGV0ZSI6ZmFsc2UsImNyZWF0ZSI6ZmFsc2UsImRvd25sb2FkIjpmYWxzZX19.kj5ZTq_drjnymXoWurSb2L1yRrUK2mEjAEv9t0RTDGw",
			Permissions: users.Permissions{
				Api:      false,
				Admin:    false,
				Share:    false,
				Realtime: false,
			},
		},
	}

	fixtureUsers := []*users.User{
		{
			ID: 1,
			FrontendUser: users.FrontendUser{
				NonAdminEditable: users.NonAdminEditable{
					Password:     "$2a$10$SdfZXgUfp0xPYxky2KTtJu0Ac7WKgeWU.VgkB2OAEy5Kd4/S8BhEi",
					SidebarLinks: adminSidebar,
				},
				Username:       "admin",
				LoginMethod:    users.LoginMethodPassword,
				FrontendScopes: adminScopes,
				Permissions: users.Permissions{
					Admin:    true,
					Api:      true,
					Share:    true,
					Realtime: false,
					Modify:   true,
					Create:   true,
					Delete:   true,
					Download: true,
				},
			},
			Tokens:  adminTokens,
			Version: 3,
		},
		{
			ID: 2,
			FrontendUser: users.FrontendUser{
				NonAdminEditable: users.NonAdminEditable{
					Password: "$2a$10$IYCsziHjzH0mPc.bZwRuXefQKPVXfFqjdyfVmNcL.XZJsgyfxljDy",
				},
				Username:    "graham",
				LoginMethod: users.LoginMethodPassword,
				FrontendScopes: []users.FrontendScope{
					{Name: fixturePlaywrightSource, Scope: "/myfolder"},
					{Name: fixtureDockerSource, Scope: "/"},
				},
				Permissions: users.Permissions{
					Modify:   true,
					Create:   true,
					Download: true,
				},
			},
			Version: 3,
		},
		{
			ID: 3,
			FrontendUser: users.FrontendUser{
				NonAdminEditable: users.NonAdminEditable{
					Password: "$2a$10$3RxlB2vCkTG2UK.Z1uH8O.q63qWdP9fzIzYcR6bXi9NWy1RwMUk62",
				},
				Username:    "basic",
				LoginMethod: users.LoginMethodPassword,
				FrontendScopes: []users.FrontendScope{
					{Name: fixturePlaywrightSource, Scope: "/"},
					{Name: fixtureAccessSource, Scope: "/"},
					{Name: fixtureDockerSource, Scope: "/"},
				},
				Permissions: users.Permissions{
					Share: true,
				},
			},
			Version: 3,
		},
	}

	for _, user := range fixtureUsers {
		if err = db.Save(user); err != nil {
			t.Fatalf("save user %q: %v", user.Username, err)
		}
	}

	shareLinks := []*share.LegacyShare{
		{
			LegacyRoutingSource: fixturePlaywrightSource,
			Share: share.Share{
				ShareSettings: share.ShareSettings{
					FrontendShareInfo: share.FrontendShareInfo{
						ShareTheme:           "default",
						ShareType:            "normal",
						Title:                "Shared files - myfolder",
						Description:          "A share has been sent to you to view or download.",
						EnforceDarkLightMode: "default",
						ViewMode:             "normal",
						AllowModify:          true,
						AllowCreate:          true,
						AllowDelete:          true,
						SidebarLinks: []users.SidebarLink{
							{Name: "Share QR Code and Info", Category: "shareInfo", Target: "#", Icon: "qr_code"},
							{Name: "Download", Category: "download", Target: "#", Icon: "download"},
						},
					},
				},
				ShareColumns: share.ShareColumns{
					Hash:   "lMhwHkF-hqCN92-QIJJZow",
					Path:   "/myfolder/",
					Expire: 0,
				},
				UserID:  1,
				Version: 1,
			},
		},
		{
			LegacyRoutingSource: fixturePlaywrightSource,
			Share: share.Share{
				ShareSettings: share.ShareSettings{
					FrontendShareInfo: share.FrontendShareInfo{
						ShareTheme:           "default",
						ShareType:            "normal",
						Title:                "Shared files - test & test.txt",
						Description:          "A share has been sent to you to view or download.",
						EnforceDarkLightMode: "default",
						ViewMode:             "normal",
						SidebarLinks: []users.SidebarLink{
							{Name: "Share QR Code and Info", Category: "shareInfo", Target: "#", Icon: "qr_code"},
							{Name: "Download", Category: "download", Target: "#", Icon: "download"},
						},
					},
				},
				ShareColumns: share.ShareColumns{
					Hash:   "dGhQi4AcMhva2Ne-7x7fvw",
					Path:   "/test & test.txt/",
					Expire: 0,
				},
				UserID:  1,
				Version: 1,
			},
		},
	}

	for _, link := range shareLinks {
		if err = db.Set("Link", link.Hash, link); err != nil {
			t.Fatalf("save share %q: %v", link.Hash, err)
		}
	}

	accessRules := struct {
		AllRules      access.SourceRuleMap `json:"all_rules"`
		Groups        access.GroupMap      `json:"groups"`
		RevokedTokens map[string]struct{}  `json:"revoked_tokens"`
		HashedTokens  map[string]uint      `json:"hashed_tokens"`
	}{
		AllRules: access.SourceRuleMap{
			fixturePlaywrightSource: {
				"/text-files/bash.sh/": {
					Deny: access.RuleSet{
						Users: access.StringSet{"admin": {}},
					},
				},
			},
			fixtureAccessSource: {
				"/": {
					Deny: access.RuleSet{
						Users: access.StringSet{"basic": {}},
					},
				},
			},
		},
		Groups:        access.GroupMap{},
		RevokedTokens: map[string]struct{}{},
		HashedTokens: map[string]uint{
			"55dfac4f44e531dec243f08c3e6ebcd87db5b74a6da6d5a84794ee9318e94f17": 1,
			"e5c95f0e2b703e3c64ee56d244d79e6cbbf11538544cbb4a074790e47a08633a": 1,
		},
	}

	data, err := json.Marshal(accessRules)
	if err != nil {
		t.Fatal(err)
	}
	if err := db.Set("access_rules", "rules", data); err != nil {
		t.Fatal(err)
	}

	t.Logf("wrote migration fixture to %s", outPath)
}
