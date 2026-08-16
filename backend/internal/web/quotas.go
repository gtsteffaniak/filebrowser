package web

import (
	"encoding/json"
	"fmt"
	"net/http"
	"strings"

	"github.com/gtsteffaniak/filebrowser/backend/internal/activity"
	"github.com/gtsteffaniak/filebrowser/backend/internal/database/quota"
	"github.com/gtsteffaniak/filebrowser/backend/internal/database/users"
	"github.com/gtsteffaniak/filebrowser/backend/internal/state"
	"github.com/gtsteffaniak/filebrowser/backend/internal/utils"
	"github.com/gtsteffaniak/filebrowser/backend/pkg/settings"
	"github.com/gtsteffaniak/go-logger/logger"
)

type folderQuotaResponse struct {
	Source            string `json:"source"`
	Path              string `json:"path"`
	LimitBytes        int64  `json:"limitBytes"`
	UsedBytes         int64  `json:"usedBytes,omitempty"`
	ReservedBytes     int64  `json:"reservedBytes,omitempty"`
	Meter             string `json:"meter,omitempty"`
	ConfiguredMeter   string `json:"configuredMeter,omitempty"`
	EffectiveMeter    string `json:"effectiveMeter,omitempty"`
	MeasurementStatus string `json:"measurementStatus,omitempty"`
}

type scopeQuotaResponse struct {
	QuotaKind         string `json:"quotaKind"`
	LimitBytes        int64  `json:"limitBytes"`
	UsedBytes         int64  `json:"usedBytes"`
	ReservedBytes     int64  `json:"reservedBytes"`
	Meter             string `json:"meter,omitempty"`
	ConfiguredMeter   string `json:"configuredMeter,omitempty"`
	EffectiveMeter    string `json:"effectiveMeter,omitempty"`
	MeasurementStatus string `json:"measurementStatus,omitempty"`
}

func quotaClientPathFromIndex(userScope, indexPath string) string {
	indexPath = strings.TrimSuffix(indexPath, "/")
	userScope = strings.TrimRight(userScope, "/")
	if indexPath == "" || indexPath == "/" {
		return "/"
	}
	if userScope == "" || userScope == "/" {
		return indexPath
	}
	if strings.HasPrefix(indexPath, userScope) {
		rest := strings.TrimPrefix(indexPath, userScope)
		if rest == "" {
			return "/"
		}
		return rest
	}
	return indexPath
}

func quotasGetHandler(w http.ResponseWriter, r *http.Request, d *Context) (int, error) {
	sourceName := r.URL.Query().Get("source")
	path := r.URL.Query().Get("path")
	if sourceName == "" {
		return http.StatusBadRequest, fmt.Errorf("source is required")
	}
	sourceInfo, ok := settings.Config.Server.NameToSource[sourceName]
	if !ok {
		return http.StatusBadRequest, fmt.Errorf("invalid source")
	}
	if path != "" {
		clean, err := utils.SanitizePath(path)
		if err != nil {
			return http.StatusBadRequest, err
		}
		path = clean
	}
	userscope, err := d.User.GetScopeForSourceName(sourceName)
	if err != nil {
		return http.StatusForbidden, err
	}
	fullPath := utils.JoinScopedIndexPath(userscope, path)
	logger.Debugf("quota GET: source=%s queryPath=%q userScope=%q fullIndexPath=%q", sourceName, path, userscope, fullPath)
	if path != "" {
		userID, err := resolveFolderQuotaUserID(r.URL.Query().Get("username"), d.User)
		if err != nil {
			return ErrToStatus(err), err
		}
		q, err := state.GetFolderQuotaByPathAndUser(sourceInfo.Path, fullPath, userID)
		if err != nil {
			preview := state.FolderQuotaUsagePreview(sourceName, fullPath)
			return RenderJSON(w, r, []folderQuotaResponse{folderUsagePreviewResponse(sourceName, path, preview)})
		}
		return RenderJSON(w, r, []folderQuotaResponse{folderQuotaToResponse(*q, sourceName, path)})
	}
	rows := state.ListFolderQuotasForSource(sourceInfo.Path)
	out := make([]folderQuotaResponse, 0, len(rows))
	for _, q := range rows {
		displayPath := quotaClientPathFromIndex(userscope, q.Path)
		out = append(out, folderQuotaToResponse(q, sourceName, displayPath))
	}
	return RenderJSON(w, r, out)
}

func quotasPostHandler(w http.ResponseWriter, r *http.Request, d *Context) (int, error) {
	var body struct {
		Source     string `json:"source"`
		Path       string `json:"path"`
		Username   string `json:"username"`
		LimitBytes int64  `json:"limitBytes"`
		Meter      string `json:"meter"`
	}
	if err := decodeJSON(r, &body); err != nil {
		return http.StatusBadRequest, err
	}
	sourceInfo, ok := settings.Config.Server.NameToSource[body.Source]
	if !ok {
		return http.StatusBadRequest, fmt.Errorf("invalid source")
	}
	clean, err := utils.SanitizePath(body.Path)
	if err != nil {
		return http.StatusBadRequest, err
	}
	userID, err := resolveFolderQuotaUserID(body.Username, d.User)
	if err != nil {
		return ErrToStatus(err), err
	}
	userscope, err := d.User.GetScopeForSourceName(body.Source)
	if err != nil {
		return http.StatusForbidden, err
	}
	fullPath := utils.JoinScopedIndexPath(userscope, clean)
	q, err := state.CreateFolderQuota(sourceInfo.Path, fullPath, userID, body.LimitBytes, body.Meter)
	if err != nil {
		return http.StatusBadRequest, err
	}
	activity.RecordQuotaCreate(r, toActor(d), body.Source, clean, activity.QuotaFolderCreateChanges(*q))
	return RenderJSON(w, r, []folderQuotaResponse{folderQuotaToResponse(*q, body.Source, clean)})
}

func quotasPatchHandler(w http.ResponseWriter, r *http.Request, d *Context) (int, error) {
	var raw map[string]json.RawMessage
	if err := json.NewDecoder(r.Body).Decode(&raw); err != nil {
		return http.StatusBadRequest, err
	}
	sourceName, err := patchStringField(raw, "source")
	if err != nil {
		return http.StatusBadRequest, err
	}
	pathVal, err := patchStringField(raw, "path")
	if err != nil {
		return http.StatusBadRequest, err
	}
	if sourceName == "" || pathVal == "" {
		return http.StatusBadRequest, fmt.Errorf("source and path are required")
	}
	sourceInfo, ok := settings.Config.Server.NameToSource[sourceName]
	if !ok {
		return http.StatusBadRequest, fmt.Errorf("invalid source")
	}
	clean, err := utils.SanitizePath(pathVal)
	if err != nil {
		return http.StatusBadRequest, err
	}
	var lookupUserID uint64
	if _, hasUsername := raw["username"]; hasUsername {
		username, uerr := patchStringField(raw, "username")
		if uerr != nil {
			return http.StatusBadRequest, uerr
		}
		lookupUserID, err = resolveFolderQuotaUserID(username, d.User)
		if err != nil {
			return ErrToStatus(err), err
		}
	}
	userscope, err := d.User.GetScopeForSourceName(sourceName)
	if err != nil {
		return http.StatusForbidden, err
	}
	fullPath := utils.JoinScopedIndexPath(userscope, clean)
	before, err := state.GetFolderQuotaByPathAndUser(sourceInfo.Path, fullPath, lookupUserID)
	if err != nil {
		return http.StatusNotFound, err
	}
	var limitBytes int64
	if v, ok := raw["limitBytes"]; ok {
		if err := json.Unmarshal(v, &limitBytes); err != nil {
			return http.StatusBadRequest, err
		}
	}
	var meter string
	if v, ok := raw["meter"]; ok {
		if err := json.Unmarshal(v, &meter); err != nil {
			return http.StatusBadRequest, err
		}
	}
	var userIDPatch *uint64
	if _, hasUsername := raw["username"]; hasUsername {
		username, uerr := patchStringField(raw, "username")
		if uerr != nil {
			return http.StatusBadRequest, uerr
		}
		uid, uerr := resolveFolderQuotaUserID(username, d.User)
		if uerr != nil {
			return ErrToStatus(uerr), uerr
		}
		userIDPatch = &uid
	}
	q, err := state.UpdateFolderQuota(before.ID, limitBytes, meter, userIDPatch)
	if err != nil {
		return http.StatusNotFound, err
	}
	changes := activity.QuotaFolderUpdateChanges(*before, *q)
	if len(changes) > 0 {
		activity.RecordQuotaUpdate(r, toActor(d), sourceName, clean, changes)
	}
	return RenderJSON(w, r, []folderQuotaResponse{folderQuotaToResponse(*q, sourceName, clean)})
}

func patchStringField(raw map[string]json.RawMessage, key string) (string, error) {
	v, ok := raw[key]
	if !ok {
		return "", nil
	}
	var s string
	if err := json.Unmarshal(v, &s); err != nil {
		return "", err
	}
	return s, nil
}

func quotasDeleteHandler(w http.ResponseWriter, r *http.Request, d *Context) (int, error) {
	sourceName := r.URL.Query().Get("source")
	path := r.URL.Query().Get("path")
	if sourceName == "" || path == "" {
		return http.StatusBadRequest, fmt.Errorf("source and path are required")
	}
	sourceInfo, ok := settings.Config.Server.NameToSource[sourceName]
	if !ok {
		return http.StatusBadRequest, fmt.Errorf("invalid source")
	}
	clean, err := utils.SanitizePath(path)
	if err != nil {
		return http.StatusBadRequest, err
	}
	userID, err := resolveFolderQuotaUserID(r.URL.Query().Get("username"), d.User)
	if err != nil {
		return ErrToStatus(err), err
	}
	userscope, err := d.User.GetScopeForSourceName(sourceName)
	if err != nil {
		return http.StatusForbidden, err
	}
	fullPath := utils.JoinScopedIndexPath(userscope, clean)
	before, err := state.GetFolderQuotaByPathAndUser(sourceInfo.Path, fullPath, userID)
	if err != nil {
		return http.StatusNotFound, err
	}
	if err := state.DeleteFolderQuota(before.ID); err != nil {
		return http.StatusNotFound, err
	}
	activity.RecordQuotaDelete(r, toActor(d), sourceName, clean, activity.QuotaFolderDeleteChanges(*before))
	return http.StatusNoContent, nil
}

func resolveFolderQuotaUserID(username string, actor *users.User) (uint64, error) {
	username = strings.TrimSpace(username)
	if username == "" {
		return 0, nil
	}
	if username != actor.Username && !actor.Permissions.Admin {
		return 0, fmt.Errorf("admin required")
	}
	u, err := state.GetUserByUsername(username)
	if err != nil {
		return 0, err
	}
	return u.ID, nil
}

func scopeQuotaToResponse(snap quota.Snapshot) scopeQuotaResponse {
	return scopeQuotaResponse{
		QuotaKind:         snap.Kind,
		LimitBytes:        snap.LimitBytes,
		UsedBytes:         snap.UsedBytes,
		ReservedBytes:     snap.ReservedBytes,
		Meter:             snap.Meter,
		ConfiguredMeter:   snap.ConfiguredMeter,
		EffectiveMeter:    snap.EffectiveMeter,
		MeasurementStatus: snap.MeasurementStatus,
	}
}

func decodeJSON(r *http.Request, dest interface{}) error {
	return json.NewDecoder(r.Body).Decode(dest)
}

func folderQuotaToResponse(q quota.FolderQuota, sourceName, clientPath string) folderQuotaResponse {
	snap := state.FolderQuotaSnapshot(q, sourceName)
	path := clientPath
	if path == "" {
		path = q.Path
	}
	return folderQuotaResponse{
		Source:            sourceName,
		Path:              path,
		LimitBytes:        q.LimitBytes,
		UsedBytes:         snap.UsedBytes,
		ReservedBytes:     snap.ReservedBytes,
		Meter:             snap.Meter,
		ConfiguredMeter:   snap.ConfiguredMeter,
		EffectiveMeter:    snap.EffectiveMeter,
		MeasurementStatus: snap.MeasurementStatus,
	}
}

func folderUsagePreviewResponse(sourceName, displayPath string, snap quota.Snapshot) folderQuotaResponse {
	return folderQuotaResponse{
		Source:            sourceName,
		Path:              displayPath,
		UsedBytes:         snap.UsedBytes,
		ReservedBytes:     snap.ReservedBytes,
		Meter:             snap.Meter,
		ConfiguredMeter:   snap.ConfiguredMeter,
		EffectiveMeter:    snap.EffectiveMeter,
		MeasurementStatus: snap.MeasurementStatus,
	}
}
