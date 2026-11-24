package web

import (
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/gtsteffaniak/filebrowser/backend/internal/adapters/fs/fileutils"
	"github.com/gtsteffaniak/filebrowser/backend/internal/gomedia"
	"github.com/gtsteffaniak/filebrowser/backend/pkg/settings"
	"github.com/stretchr/testify/require"
)

func TestShadowClearResetsStats(t *testing.T) {
	if fileutils.PermDir == 0 {
		fileutils.SetFsPermissions(0o644, 0o755)
	}
	prev := settings.Config.Server.CacheDir
	settings.Config.Server.CacheDir = t.TempDir()
	t.Cleanup(func() { settings.Config.Server.CacheDir = prev })

	gomedia.ShadowStatsObserveForTest("probe")
	require.NotEmpty(t, gomedia.ShadowStats())

	req := httptest.NewRequest(http.MethodDelete, "/api/settings/shadow-regressions", nil)
	rec := httptest.NewRecorder()
	_, err := settingsShadowRegressionsClearHandler(rec, req, nil)
	require.NoError(t, err)
	require.Empty(t, gomedia.ShadowStats())
}
