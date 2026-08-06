package state

import (
	"path/filepath"
	"testing"

	"github.com/gtsteffaniak/filebrowser/backend/internal/database/users"
	"github.com/gtsteffaniak/filebrowser/backend/pkg/settings"
)

func TestInitSourceAccessDefaults_overlaysConfigOnRestart(t *testing.T) {
	t.Setenv("FILEBROWSER_ONLYOFFICE_SECRET", "")
	t.Setenv("FILEBROWSER_PLAYWRIGHT_TEST", "true")
	settings.Initialize("../../../_docker/src/jwt/backend/config.yaml")

	dbPath := filepath.Join(t.TempDir(), "filebrowser.sqlite")
	_, err := Initialize(dbPath)
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = Close() })

	access := GetSourceAccessDefaults()
	if !access.Modify || !access.Create {
		t.Fatalf("initial source access defaults: %+v", access)
	}

	// Simulate admin changing modify in SQLite; config should re-overlay on next init.
	sourceAccessMu.Lock()
	doc := sourceAccessSettingsDocument{
		DefaultPermissions: users.SourceFilePermissions{
			View: true, Download: true, Modify: false, Create: true, Delete: false,
		},
		EnforcedPermissions: sourceAccessEnforcedDefault,
	}
	if saveErr := saveSourceAccessSettingsDocument(doc); saveErr != nil {
		sourceAccessMu.Unlock()
		t.Fatal(saveErr)
	}
	sourceAccessMu.Unlock()

	if err := InitSourceAccessDefaults(); err != nil {
		t.Fatal(err)
	}

	access = GetSourceAccessDefaults()
	if !access.Modify {
		t.Fatal("expected modify re-overlaid from jwt config on restart")
	}
	if settings.Env.ConfigSourceDefaultPermissions == nil {
		t.Fatal("expected config source default permissions for jwt yaml")
	}
}
