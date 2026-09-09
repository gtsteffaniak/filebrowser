package web

import (
	"context"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/gtsteffaniak/filebrowser/backend/internal/database/users"
	"github.com/gtsteffaniak/filebrowser/backend/internal/transcoding"
)

func setupTranscodeHandlerTest(t *testing.T) (*transcoding.Manager, func()) {
	t.Helper()
	ctx, cancel := context.WithCancel(context.Background())
	store, err := transcoding.NewStore(t.TempDir())
	if err != nil {
		t.Fatal(err)
	}
	mgr, err := transcoding.NewManager(ctx, transcoding.NewFakeEngine(), store, transcoding.Options{BaseURL: ""})
	if err != nil {
		t.Fatal(err)
	}
	prev := runtimeDeps.Transcoding
	runtimeDeps.Transcoding = mgr
	return mgr, func() {
		runtimeDeps.Transcoding = prev
		cancel()
		_ = mgr.Close(context.Background())
	}
}

func TestTranscodePlaylistHandlerEnforcesOwnership(t *testing.T) {
	mgr, cleanup := setupTranscodeHandlerTest(t)
	defer cleanup()

	ctx := context.Background()
	other := &users.User{FrontendUser: users.FrontendUser{Username: "other"}}

	info, err := mgr.Start(ctx, transcoding.StartInput{
		Username:  "owner",
		UserLimit: 1,
		Source:    "s",
		Path:      "/clip.mp4",
		RealPath:  t.TempDir() + "/clip.mp4",
		Profile:   transcoding.ProfileQuality,
	})
	if err != nil {
		t.Fatal(err)
	}

	req := httptest.NewRequest(http.MethodGet, "/api/media/transcode/sessions/"+info.ID+"/index.m3u8", nil)
	req.SetPathValue("id", info.ID)
	rec := httptest.NewRecorder()
	status, handlerErr := transcodePlaylistHandler(rec, req, &Context{User: other})
	if handlerErr != transcoding.ErrNotOwner {
		t.Fatalf("err=%v status=%d", handlerErr, status)
	}
	if status != http.StatusNotFound {
		t.Fatalf("status=%d want %d", status, http.StatusNotFound)
	}
}

func TestTranscodeSegmentHandlerEnforcesOwnership(t *testing.T) {
	mgr, cleanup := setupTranscodeHandlerTest(t)
	defer cleanup()

	ctx := context.Background()
	other := &users.User{FrontendUser: users.FrontendUser{Username: "other"}}

	info, err := mgr.Start(ctx, transcoding.StartInput{
		Username:  "owner",
		UserLimit: 1,
		Source:    "s",
		Path:      "/clip.mp4",
		RealPath:  t.TempDir() + "/clip.mp4",
		Profile:   transcoding.ProfileQuality,
	})
	if err != nil {
		t.Fatal(err)
	}

	req := httptest.NewRequest(http.MethodGet, "/api/media/transcode/sessions/"+info.ID+"/seg/00000.m4s", nil)
	req.SetPathValue("id", info.ID)
	req.SetPathValue("name", "00000.m4s")
	rec := httptest.NewRecorder()
	status, handlerErr := transcodeSegmentHandler(rec, req, &Context{User: other})
	if handlerErr != transcoding.ErrNotOwner {
		t.Fatalf("err=%v status=%d", handlerErr, status)
	}
	if status != http.StatusNotFound {
		t.Fatalf("status=%d want %d", status, http.StatusNotFound)
	}
}
