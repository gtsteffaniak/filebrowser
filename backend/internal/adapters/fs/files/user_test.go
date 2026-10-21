package files

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/gtsteffaniak/filebrowser/backend/internal/database/users"
	"github.com/gtsteffaniak/filebrowser/backend/pkg/settings"
)

func TestMakeUserDirsSkipsAdmins(t *testing.T) {
	root := t.TempDir()
	saved := settings.Config
	t.Cleanup(func() { settings.Config = saved })
	settings.Config = settings.Settings{
		Server: settings.Server{
			Sources: []*settings.Source{{
				Path: root,
				Config: settings.SourceConfig{
					CreateUserDir: true,
				},
			}},
			SourceMap: map[string]*settings.Source{
				root: {
					Path: root,
					Config: settings.SourceConfig{
						CreateUserDir: true,
					},
				},
			},
		},
	}

	u := &users.User{
		FrontendUser: users.FrontendUser{
			Username: "admin",
			Permissions: users.Permissions{
				Admin: true,
			},
		},
		BackendScopes: []users.BackendScope{{Path: root, Scope: "/"}},
	}

	if err := MakeUserDirs(u, true); err != nil {
		t.Fatalf("MakeUserDirs: %v", err)
	}
	if got := u.BackendScopes[0].Scope; got != "/" {
		t.Fatalf("scope=%q want /", got)
	}
	if _, err := os.Stat(filepath.Join(root, "admin")); !os.IsNotExist(err) {
		t.Fatalf("admin directory stat error=%v, want not-exist", err)
	}
}
