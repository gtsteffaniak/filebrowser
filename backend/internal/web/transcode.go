package web

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"os"
	"path/filepath"
	"strings"
	"time"

	"github.com/gtsteffaniak/go-logger/logger"

	liberrors "github.com/gtsteffaniak/filebrowser/backend/internal/errors"
	"github.com/gtsteffaniak/filebrowser/backend/internal/state"
	"github.com/gtsteffaniak/filebrowser/backend/internal/transcoding"
	"github.com/gtsteffaniak/filebrowser/backend/internal/utils"
	"github.com/gtsteffaniak/filebrowser/backend/pkg/indexing"
	"github.com/gtsteffaniak/filebrowser/backend/pkg/settings"
)

func transcodeManager() *transcoding.Manager {
	if runtimeDeps.Transcoding == nil {
		return nil
	}
	return runtimeDeps.Transcoding
}

func transcodeStartHandler(w http.ResponseWriter, r *http.Request, d *Context) (int, error) {
	mgr := transcodeManager()
	if mgr == nil || !mgr.Enabled() {
		return http.StatusServiceUnavailable, transcoding.ErrDisabled
	}
	if d.Share.Hash != "" {
		return http.StatusNotFound, fmt.Errorf("transcoding not available for shares")
	}

	var req transcoding.StartRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		return http.StatusBadRequest, err
	}
	req.Source = strings.TrimSpace(req.Source)
	req.Path = strings.TrimSpace(req.Path)
	if req.Path != "" && !strings.HasPrefix(req.Path, "/") {
		req.Path = "/" + req.Path
	}
	if req.Source == "" || req.Path == "" {
		return http.StatusBadRequest, fmt.Errorf("source and path are required")
	}
	if err := ValidateViewGrant(strings.TrimSpace(req.ViewToken), d, req.Source); err != nil {
		return http.StatusForbidden, err
	}

	profile, err := transcoding.ParseProfile(req.Profile)
	if err != nil {
		if req.Profile == "" {
			profile = transcoding.ProfileQuality
		} else {
			return http.StatusUnprocessableEntity, err
		}
	}

	realPath, fileName, status, resolveErr := resolveTranscodeFile(d, req.Source, req.Path)
	if resolveErr != nil {
		return status, resolveErr
	}

	userLimit := effectiveUserLimit(d.User.MaxConcurrentTranscodes)

	logger.Infof("[transcode] POST sessions user=%s source=%q path=%q profile=%q replace=%q client=%q startSec=%.2f",
		d.User.Username, req.Source, req.Path, profile, req.ReplaceSessionID, req.ClientID, req.StartSec)
	info, startErr := mgr.Start(r.Context(), transcoding.StartInput{
		Username:         d.User.Username,
		UserLimit:        userLimit,
		Source:           req.Source,
		Path:             req.Path,
		RealPath:         realPath,
		FileName:         fileName,
		Profile:          profile,
		ReplaceSessionID: strings.TrimSpace(req.ReplaceSessionID),
		ClientID:         req.ClientID,
		StartSec:         req.StartSec,
	})
	if startErr != nil {
		logger.Infof("[transcode] POST sessions failed user=%s err=%v", d.User.Username, startErr)
		return transcodeErrorStatus(startErr), startErr
	}
	logger.Infof("[transcode] POST sessions ok user=%s session=%s state=%s reused=%v delivery=%s",
		d.User.Username, info.ID, info.State, info.Reused, info.DeliveryBaseURL)

	snap := mgr.Snapshot(d.User.Username, false, userLimit)
	resp := struct {
		transcoding.SessionInfo
		Snapshot transcoding.SnapshotResponse `json:"snapshot"`
	}{
		SessionInfo: info,
		Snapshot:    snap,
	}
	code := http.StatusCreated
	if info.Reused {
		code = http.StatusOK
	}
	w.Header().Set("Content-Type", "application/json")
	if err := json.NewEncoder(w).Encode(resp); err != nil {
		return http.StatusInternalServerError, err
	}
	return code, nil
}

func transcodeSessionsHandler(w http.ResponseWriter, r *http.Request, d *Context) (int, error) {
	mgr := transcodeManager()
	if mgr == nil {
		return http.StatusServiceUnavailable, transcoding.ErrDisabled
	}
	adminAll := d.User.Permissions.Admin && strings.EqualFold(r.URL.Query().Get("all"), "true")
	snap := mgr.Snapshot(d.User.Username, adminAll, effectiveUserLimit(d.User.MaxConcurrentTranscodes))
	w.Header().Set("Content-Type", "application/json")
	if err := json.NewEncoder(w).Encode(snap); err != nil {
		return http.StatusInternalServerError, err
	}
	return http.StatusOK, nil
}

func transcodeSessionDeleteHandler(w http.ResponseWriter, r *http.Request, d *Context) (int, error) {
	mgr := transcodeManager()
	if mgr == nil {
		return http.StatusServiceUnavailable, transcoding.ErrDisabled
	}
	id := strings.TrimSpace(r.PathValue("id"))
	if id == "" {
		return http.StatusBadRequest, fmt.Errorf("session id required")
	}
	if err := mgr.Stop(d.User.Username, id); err != nil {
		return transcodeErrorStatus(err), err
	}
	w.WriteHeader(http.StatusNoContent)
	return http.StatusNoContent, nil
}

func transcodeSessionHeartbeatHandler(w http.ResponseWriter, r *http.Request, d *Context) (int, error) {
	mgr := transcodeManager()
	if mgr == nil {
		return http.StatusServiceUnavailable, transcoding.ErrDisabled
	}
	id := strings.TrimSpace(r.PathValue("id"))
	var req transcoding.HeartbeatRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		return http.StatusBadRequest, err
	}
	if err := mgr.Heartbeat(d.User.Username, id, req); err != nil {
		return transcodeErrorStatus(err), err
	}
	w.WriteHeader(http.StatusNoContent)
	return http.StatusNoContent, nil
}

func transcodeEventsHandler(w http.ResponseWriter, r *http.Request, d *Context) (int, error) {
	mgr := transcodeManager()
	if mgr == nil {
		return http.StatusServiceUnavailable, transcoding.ErrDisabled
	}
	flusher, ok := w.(http.Flusher)
	if !ok {
		return http.StatusInternalServerError, fmt.Errorf("streaming unsupported")
	}
	w.Header().Set("Content-Type", "text/event-stream")
	w.Header().Set("Cache-Control", "no-cache")
	w.Header().Set("Connection", "keep-alive")

	events, cancel := mgr.Subscribe()
	defer cancel()

	userLimit := effectiveUserLimit(d.User.MaxConcurrentTranscodes)
	snap := mgr.Snapshot(d.User.Username, false, userLimit)
	initPayload, _ := json.Marshal(transcoding.EventPayload{
		ID:      "0",
		Type:    "snapshot",
		Summary: snap,
	})
	fmt.Fprintf(w, "id: 0\nevent: transcode\ndata: %s\n\n", initPayload)
	flusher.Flush()

	heartbeat := time.NewTicker(30 * time.Second)
	defer heartbeat.Stop()
	for {
		select {
		case <-r.Context().Done():
			return http.StatusOK, nil
		case <-heartbeat.C:
			fmt.Fprintf(w, ": keepalive\n\n")
			flusher.Flush()
		case evt, ok := <-events:
			if !ok {
				return http.StatusOK, nil
			}
			if evt.Session != nil && evt.Session.Username != d.User.Username && !d.User.Permissions.Admin {
				continue
			}
			payload, _ := json.Marshal(evt)
			fmt.Fprintf(w, "id: %s\nevent: transcode\ndata: %s\n\n", evt.ID, payload)
			flusher.Flush()
		}
	}
}

func transcodePlaylistHandler(w http.ResponseWriter, r *http.Request, d *Context) (int, error) {
	mgr := transcodeManager()
	if mgr == nil {
		return http.StatusServiceUnavailable, transcoding.ErrDisabled
	}
	id := strings.TrimSpace(r.PathValue("id"))
	deadline := time.Now().Add(15 * time.Second)
	var lastErr error
	for {
		if mgr.Failed(d.User.Username, id) {
			logger.Warningf("[transcode] playlist failed session=%s user=%s (producer failed)", id, d.User.Username)
			return http.StatusServiceUnavailable, fmt.Errorf("transcode producer failed")
		}
		data, err := mgr.Playlist(d.User.Username, id)
		if err == nil && len(data) > 0 {
			logger.Infof("[transcode] playlist session=%s user=%s bytes=%d", id, d.User.Username, len(data))
			w.Header().Set("Content-Type", "application/vnd.apple.mpegurl")
			w.Header().Set("Cache-Control", "no-cache")
			_, writeErr := w.Write(data)
			if writeErr != nil {
				return http.StatusInternalServerError, writeErr
			}
			return http.StatusOK, nil
		}
		lastErr = err
		if time.Now().After(deadline) {
			if lastErr == nil {
				// The session exists but never produced a playlist (encoder failed or is stalled).
				logger.Warningf("[transcode] playlist not ready session=%s user=%s", id, d.User.Username)
				w.Header().Set("Retry-After", "5")
				return http.StatusServiceUnavailable, fmt.Errorf("transcode playlist not ready")
			}
			logger.Warningf("[transcode] playlist failed session=%s user=%s err=%v", id, d.User.Username, lastErr)
			return transcodeErrorStatus(lastErr), lastErr
		}
		select {
		case <-r.Context().Done():
			return http.StatusRequestTimeout, r.Context().Err()
		case <-time.After(200 * time.Millisecond):
		}
	}
}

func transcodeInitHandler(w http.ResponseWriter, r *http.Request, d *Context) (int, error) {
	mgr := transcodeManager()
	if mgr == nil {
		return http.StatusServiceUnavailable, transcoding.ErrDisabled
	}
	id := strings.TrimSpace(r.PathValue("id"))
	f, err := mgr.OpenInit(d.User.Username, id)
	if err != nil {
		logger.Infof("[transcode] init failed session=%s user=%s err=%v", id, d.User.Username, err)
		return transcodeErrorStatus(err), err
	}
	defer f.Close()
	logger.Infof("[transcode] init session=%s user=%s", id, d.User.Username)
	w.Header().Set("Content-Type", "video/iso.segment")
	w.Header().Set("Cache-Control", "private, max-age=3600")
	_, copyErr := io.Copy(w, f)
	if copyErr != nil {
		return http.StatusInternalServerError, copyErr
	}
	return http.StatusOK, nil
}

func transcodeSegmentHandler(w http.ResponseWriter, r *http.Request, d *Context) (int, error) {
	mgr := transcodeManager()
	if mgr == nil {
		return http.StatusServiceUnavailable, transcoding.ErrDisabled
	}
	id := strings.TrimSpace(r.PathValue("id"))
	name := strings.TrimSpace(r.PathValue("name"))
	// A segment listed in the playlist is normally aligned already; retry briefly
	// so a narrow race with the aligner never surfaces as a failed request.
	f, err := openSegmentWithRetry(r.Context(), mgr, d.User.Username, id, name)
	if err != nil {
		logger.Infof("[transcode] segment failed session=%s user=%s name=%s err=%v", id, d.User.Username, name, err)
		return transcodeErrorStatus(err), err
	}
	defer f.Close()
	logger.Infof("[transcode] segment session=%s user=%s name=%s", id, d.User.Username, name)
	w.Header().Set("Content-Type", "video/iso.segment")
	w.Header().Set("Cache-Control", "private, max-age=3600")
	_, copyErr := io.Copy(w, f)
	if copyErr != nil {
		return http.StatusInternalServerError, copyErr
	}
	return http.StatusOK, nil
}

func openSegmentWithRetry(ctx context.Context, mgr *transcoding.Manager, username, sessionID, name string) (*os.File, error) {
	deadline := time.Now().Add(10 * time.Second)
	for {
		f, err := mgr.OpenSegment(username, sessionID, name)
		if err == nil || !errors.Is(err, transcoding.ErrNotReady) {
			return f, err
		}
		if time.Now().After(deadline) {
			return nil, err
		}
		select {
		case <-ctx.Done():
			return nil, ctx.Err()
		case <-time.After(100 * time.Millisecond):
		}
	}
}

func resolveTranscodeFile(d *Context, source, scopedPath string) (realPath, fileName string, status int, err error) {
	idx := indexing.GetIndex(source)
	if idx == nil {
		return "", "", http.StatusInternalServerError, fmt.Errorf("source %s is not available", source)
	}
	if !stateAccessPermitted(d, idx.Path, scopedPath) {
		return "", "", http.StatusForbidden, fmt.Errorf("access denied")
	}
	userScope, scopeErr := d.User.GetScopeForSourceName(source)
	if scopeErr != nil || userScope == "" {
		return "", "", http.StatusForbidden, fmt.Errorf("user has no access to source: %s", source)
	}
	realPath, _, err = idx.GetRealPathScoped(userScope, scopedPath)
	if err != nil {
		if errors.Is(err, liberrors.ErrPathEscapesScope) {
			return "", "", http.StatusForbidden, err
		}
		return "", "", http.StatusInternalServerError, err
	}
	fileName = filepath.Base(scopedPath)
	return realPath, fileName, http.StatusOK, nil
}

func stateAccessPermitted(d *Context, sourcePath, scopedPath string) bool {
	permUser := d.User.Username
	return state.AccessPermitted(sourcePath, utils.IndexPathFromNormalized(scopedPath, true), permUser)
}

func transcodeErrorStatus(err error) int {
	switch {
	case errors.Is(err, transcoding.ErrNotFound):
		return http.StatusNotFound
	case errors.Is(err, transcoding.ErrNotOwner):
		return http.StatusNotFound
	case errors.Is(err, transcoding.ErrUserLimit), errors.Is(err, transcoding.ErrReplaceRequired):
		return http.StatusConflict
	case errors.Is(err, transcoding.ErrGlobalLimit), errors.Is(err, transcoding.ErrUnavailable), errors.Is(err, transcoding.ErrDisabled):
		return http.StatusServiceUnavailable
	case errors.Is(err, transcoding.ErrInvalidProfile), errors.Is(err, transcoding.ErrUnsupportedMedia):
		return http.StatusUnprocessableEntity
	case errors.Is(err, transcoding.ErrNotReady):
		// A 2xx would make hls.js treat the (non-segment) error body as media.
		return http.StatusServiceUnavailable
	default:
		return http.StatusInternalServerError
	}
}

// effectiveUserLimit resolves a user's concurrent transcode limit, clamped by the
// admin-defined per-user ceiling and the global limit.
func effectiveUserLimit(userMax int) int {
	limit := userMax
	if limit < 1 {
		limit = settings.Config.UserDefaults.Account.MaxConcurrentTranscodes
	}
	if limit < 1 {
		limit = 1
	}
	if ceiling := settings.TranscodeMaxConcurrentPerUser(); ceiling > 0 && limit > ceiling {
		limit = ceiling
	}
	if global := settings.TranscodeMaxConcurrent(); global > 0 && limit > global {
		limit = global
	}
	return limit
}
