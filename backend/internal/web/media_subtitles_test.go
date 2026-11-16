package web

import (
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/gtsteffaniak/filebrowser/backend/internal/adapters/fs/files"
	"github.com/gtsteffaniak/filebrowser/backend/internal/app"
	"github.com/gtsteffaniak/filebrowser/backend/internal/database/share"
	"github.com/gtsteffaniak/filebrowser/backend/internal/database/users"
	"github.com/gtsteffaniak/filebrowser/backend/internal/state"
	"github.com/gtsteffaniak/filebrowser/backend/internal/utils"
	"github.com/gtsteffaniak/filebrowser/backend/pkg/indexing"
	"github.com/gtsteffaniak/filebrowser/backend/pkg/indexing/iteminfo"
	"github.com/gtsteffaniak/filebrowser/backend/pkg/settings"
)

func setupSubtitlesHTTPTest(t *testing.T) (downloadsPath string, admin *users.User) {
	t.Helper()
	lockWebGlobals(t)
	downloadsPath = filepath.Join(t.TempDir(), "downloads")

	dbPath := filepath.Join(t.TempDir(), "subtitles-http-test.sqlite")
	if _, err := state.Initialize(dbPath); err != nil {
		t.Fatal(err)
	}
	app.MustWireServices(state.Default())
	t.Cleanup(func() {
		state.Close()
	})

	settings.Config.Server.SourceMap = map[string]*settings.Source{
		downloadsPath: {
			Path: downloadsPath,
			Name: "Downloads",
		},
	}
	settings.Config.Server.NameToSource = map[string]*settings.Source{
		"Downloads": settings.Config.Server.SourceMap[downloadsPath],
	}
	settings.InitializeUserResolvers()

	adminUser := &users.User{
		ID: 1,
		FrontendUser: users.FrontendUser{
			Username:    "admin",
			Permissions: users.Permissions{Admin: true},
		},
		BackendScopes: []users.BackendScope{
			{Path: downloadsPath, Scope: "/"},
		},
		BackendSourcePermissions: map[string]users.SourceFilePermissions{
			downloadsPath: {
				View:     true,
				Download: true,
				Modify:   true,
				Create:   true,
				Delete:   true,
			},
		},
		Version: users.SourcePermissionsMigrationVersion,
	}
	if err := state.CreateUser(adminUser, ""); err != nil {
		t.Fatal(err)
	}
	adminUser.Permissions = users.Permissions{Admin: true}
	if err := state.UpdateUser(adminUser, "", "permissions"); err != nil {
		t.Fatal(err)
	}
	if err := state.UpdateUser(adminUser, "", "backendScopes"); err != nil {
		t.Fatal(err)
	}
	if err := state.UpdateUser(adminUser, "", "backendSourcePermissions"); err != nil {
		t.Fatal(err)
	}

	if err := os.MkdirAll(downloadsPath, 0755); err != nil {
		t.Fatal(err)
	}

	indexing.SetTestIndex("Downloads", downloadsPath)
	t.Cleanup(func() {
		indexing.ClearTestIndices()
	})

	loaded, err := state.GetUserByUsername("admin")
	if err != nil {
		t.Fatal(err)
	}
	return downloadsPath, &loaded
}

func mockVideoWithExternalSubtitle(t *testing.T, trackName, videoRealPath string) {
	t.Helper()
	prev := files.FileInfoFasterFunc
	t.Cleanup(func() { files.FileInfoFasterFunc = prev })
	files.FileInfoFasterFunc = func(opts utils.FileOptions, user *users.User) (*iteminfo.ExtendedFileInfo, error) {
		return &iteminfo.ExtendedFileInfo{
			FileInfo: iteminfo.FileInfo{
				Path: opts.Path,
				ItemInfo: iteminfo.ItemInfo{
					Type: "video/mp4",
				},
			},
			Subtitles: []utils.SubtitleTrack{
				{Name: trackName, Embedded: false},
			},
			RealPath: videoRealPath,
		}, nil
	}
}

func TestSubtitlesHandler_RejectsEscapingSidecarSymlink(t *testing.T) {
	downloadsPath, adminUser := setupSubtitlesHTTPTest(t)
	videoPath := filepath.Join(downloadsPath, "clip.mov")
	mockVideoWithExternalSubtitle(t, "clip.srt", videoPath)

	outside := filepath.Join(filepath.Dir(downloadsPath), "outside-secret")
	if err := os.MkdirAll(outside, 0755); err != nil {
		t.Fatal(err)
	}
	secret := filepath.Join(outside, "secret.srt")
	if err := os.WriteFile(secret, []byte("SECRET_OUTSIDE_SUBTITLE"), 0644); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(videoPath, []byte("video"), 0644); err != nil {
		t.Fatal(err)
	}
	if err := os.Symlink(secret, filepath.Join(downloadsPath, "clip.srt")); err != nil {
		t.Fatal(err)
	}

	req := httptest.NewRequest(http.MethodGet, "/api/media/subtitles?source=Downloads&path=/clip.mov&name=clip.srt&embedded=false", nil)
	rec := httptest.NewRecorder()
	status, handlerErr := subtitlesHandler(rec, req, &Context{User: adminUser})
	if status != http.StatusForbidden {
		t.Fatalf("status=%d err=%v body=%q", status, handlerErr, rec.Body.String())
	}
	if strings.Contains(rec.Body.String(), "SECRET_OUTSIDE_SUBTITLE") {
		t.Fatalf("response leaked outside subtitle content")
	}
}

func TestSubtitlesHandler_ReturnsInScopeSidecar(t *testing.T) {
	downloadsPath, adminUser := setupSubtitlesHTTPTest(t)
	videoPath := filepath.Join(downloadsPath, "clip.mov")
	mockVideoWithExternalSubtitle(t, "clip.srt", videoPath)

	srtBody := "1\n00:00:01,000 --> 00:00:02,000\nHello in scope"
	if err := os.WriteFile(videoPath, []byte("video"), 0644); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(downloadsPath, "clip.srt"), []byte(srtBody), 0644); err != nil {
		t.Fatal(err)
	}

	req := httptest.NewRequest(http.MethodGet, "/api/media/subtitles?source=Downloads&path=/clip.mov&name=clip.srt&embedded=false", nil)
	rec := httptest.NewRecorder()
	status, handlerErr := subtitlesHandler(rec, req, &Context{User: adminUser})
	if status != http.StatusOK || handlerErr != nil {
		t.Fatalf("status=%d err=%v body=%q", status, handlerErr, rec.Body.String())
	}
	if !strings.Contains(rec.Body.String(), "Hello in scope") {
		t.Fatalf("body=%q want in-scope subtitle text", rec.Body.String())
	}
}

func TestSubtitlesHandler_ShareScopedOwnerLoadsSidecar(t *testing.T) {
	sourceRoot, adminUser := setupSubtitlesHTTPTest(t)
	aliceRoot := filepath.Join(sourceRoot, "users", "alice", "shared")
	if err := os.MkdirAll(aliceRoot, 0755); err != nil {
		t.Fatal(err)
	}
	srtBody := "1\n00:00:01,000 --> 00:00:02,000\nScoped share OK"
	videoPath := filepath.Join(aliceRoot, "clip.mov")
	if err := os.WriteFile(videoPath, []byte("video"), 0644); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(aliceRoot, "clip.srt"), []byte(srtBody), 0644); err != nil {
		t.Fatal(err)
	}
	mockVideoWithExternalSubtitle(t, "clip.srt", videoPath)

	adminUser.BackendScopes = []users.BackendScope{{Path: sourceRoot, Scope: "/users/alice"}}
	req := httptest.NewRequest(http.MethodGet, "/public/api/media/subtitles?hash=h&path=clip.mov&name=clip.srt&embedded=false", nil)
	rec := httptest.NewRecorder()
	status, handlerErr := subtitlesHandler(rec, req, &Context{
		User:      adminUser,
		ShareUser: adminUser,
		IndexPath: "/shared/clip.mov/",
		Share: share.Share{
			ShareColumns: share.ShareColumns{
				Hash: "h",
				Path: "/users/alice/shared",
			},
			SourcePath: sourceRoot,
		},
	})
	if status != http.StatusOK || handlerErr != nil {
		t.Fatalf("status=%d err=%v body=%q", status, handlerErr, rec.Body.String())
	}
	if !strings.Contains(rec.Body.String(), "Scoped share OK") {
		t.Fatalf("body=%q", rec.Body.String())
	}
}

func TestSubtitlesHandler_ShareSingleFileLoadsSidecar(t *testing.T) {
	sourceRoot, adminUser := setupSubtitlesHTTPTest(t)
	srtBody := "single-file share subtitle"
	videoPath := filepath.Join(sourceRoot, "clip.mov")
	if err := os.WriteFile(videoPath, []byte("video"), 0644); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(sourceRoot, "clip.srt"), []byte(srtBody), 0644); err != nil {
		t.Fatal(err)
	}
	mockVideoWithExternalSubtitle(t, "clip.srt", videoPath)

	req := httptest.NewRequest(http.MethodGet, "/public/api/media/subtitles?hash=h&path=/&name=clip.srt&embedded=false", nil)
	rec := httptest.NewRecorder()
	status, handlerErr := subtitlesHandler(rec, req, &Context{
		User:      adminUser,
		ShareUser: adminUser,
		IndexPath: "/clip.mov/",
		Share: share.Share{
			ShareColumns: share.ShareColumns{
				Hash: "h",
				Path: "/clip.mov",
			},
			SourcePath: sourceRoot,
		},
	})
	if status != http.StatusOK || handlerErr != nil {
		t.Fatalf("status=%d err=%v body=%q", status, handlerErr, rec.Body.String())
	}
	if !strings.Contains(rec.Body.String(), "single-file share subtitle") {
		t.Fatalf("body=%q", rec.Body.String())
	}
}

func TestSubtitlesHandler_ShareDeniesSidecarSymlinkToDeniedTarget(t *testing.T) {
	sourceRoot, adminUser := setupSubtitlesHTTPTest(t)
	sharedDir := filepath.Join(sourceRoot, "shared")
	privateDir := filepath.Join(sourceRoot, "private")
	for _, dir := range []string{sharedDir, privateDir} {
		if err := os.MkdirAll(dir, 0755); err != nil {
			t.Fatal(err)
		}
	}
	secret := filepath.Join(privateDir, "clip.srt")
	if err := os.WriteFile(secret, []byte("SECRET_DENIED_SUBTITLE"), 0644); err != nil {
		t.Fatal(err)
	}
	videoPath := filepath.Join(sharedDir, "clip.mov")
	if err := os.WriteFile(videoPath, []byte("video"), 0644); err != nil {
		t.Fatal(err)
	}
	if err := os.Symlink(filepath.Join("..", "private", "clip.srt"), filepath.Join(sharedDir, "clip.srt")); err != nil {
		t.Fatal(err)
	}
	if err := state.DenyUser(sourceRoot, utils.IndexPathFromNormalized("/private", true), "admin"); err != nil {
		t.Fatal(err)
	}
	mockVideoWithExternalSubtitle(t, "clip.srt", videoPath)

	adminUser.BackendScopes = []users.BackendScope{{Path: sourceRoot, Scope: "/"}}
	req := httptest.NewRequest(http.MethodGet, "/public/api/media/subtitles?hash=h&path=clip.mov&name=clip.srt&embedded=false", nil)
	rec := httptest.NewRecorder()
	status, handlerErr := subtitlesHandler(rec, req, &Context{
		User:      adminUser,
		ShareUser: adminUser,
		IndexPath: "/shared/clip.mov/",
		Share: share.Share{
			ShareColumns: share.ShareColumns{
				Hash: "h",
				Path: "/shared",
			},
			SourcePath: sourceRoot,
		},
	})
	if status != http.StatusForbidden {
		t.Fatalf("status=%d err=%v body=%q", status, handlerErr, rec.Body.String())
	}
	if strings.Contains(rec.Body.String(), "SECRET_DENIED_SUBTITLE") {
		t.Fatalf("response leaked denied subtitle")
	}
}

func TestSubtitlesHandler_ShareRejectsSidecarPastEscapingDirSymlink(t *testing.T) {
	sourceRoot, adminUser := setupSubtitlesHTTPTest(t)
	videoIndexPath := "/shared/link/clip.mov/"
	mockVideoWithExternalSubtitle(t, "clip.srt", filepath.Join(sourceRoot, "shared", "link", "clip.mov"))

	sharedDir := filepath.Join(sourceRoot, "shared")
	if err := os.MkdirAll(sharedDir, 0755); err != nil {
		t.Fatal(err)
	}
	outside := filepath.Join(filepath.Dir(sourceRoot), "outside-share")
	if err := os.MkdirAll(outside, 0755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(outside, "clip.srt"), []byte("SECRET_PAST_DIR_SYMLINK"), 0644); err != nil {
		t.Fatal(err)
	}
	if err := os.Symlink(outside, filepath.Join(sharedDir, "link")); err != nil {
		t.Fatal(err)
	}

	req := httptest.NewRequest(http.MethodGet, "/public/api/media/subtitles?hash=h&path=link/clip.mov&name=clip.srt&embedded=false", nil)
	rec := httptest.NewRecorder()
	status, handlerErr := subtitlesHandler(rec, req, &Context{
		User:      adminUser,
		ShareUser: adminUser,
		IndexPath: videoIndexPath,
		Share: share.Share{
			ShareColumns: share.ShareColumns{
				Hash: "h",
				Path: "/shared",
			},
			SourcePath: sourceRoot,
		},
	})
	if status != http.StatusForbidden {
		t.Fatalf("status=%d err=%v body=%q", status, handlerErr, rec.Body.String())
	}
	if strings.Contains(rec.Body.String(), "SECRET_PAST_DIR_SYMLINK") {
		t.Fatalf("response leaked subtitle past directory symlink")
	}
}
