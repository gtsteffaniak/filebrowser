package web

import (
	"encoding/json"
	"fmt"
	"net/http"
	"strings"

	"github.com/gtsteffaniak/filebrowser/backend/internal/activity"
	"github.com/gtsteffaniak/filebrowser/backend/internal/database/quota"
	"github.com/gtsteffaniak/filebrowser/backend/internal/state"
	"github.com/gtsteffaniak/filebrowser/backend/internal/utils"
	"github.com/gtsteffaniak/filebrowser/backend/pkg/settings"
	"github.com/gtsteffaniak/go-logger/logger"
)

type folderQuotaResponse struct {
	ID                string `json:"id"`
	Source            string `json:"source"`
	Path              string `json:"path"`
	UserID            uint64 `json:"userId,omitempty"`
	LimitBytes        int64  `json:"limitBytes"`
	UsedBytes         int64  `json:"usedBytes,omitempty"`
	ReservedBytes     int64  `json:"reservedBytes,omitempty"`
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
		q, err := state.GetFolderQuotaByPath(sourceInfo.Path, fullPath)
		if err != nil {
			preview := state.FolderQuotaUsagePreview(sourceName, fullPath)
			return RenderJSON(w, r, []folderQuotaResponse{folderUsagePreviewResponse(sourceName, path, preview)})
		}
		return RenderJSON(w, r, folderQuotaToResponse(*q, sourceName))
	}
	rows := state.ListFolderQuotasForSource(sourceInfo.Path)
	out := make([]folderQuotaResponse, 0, len(rows))
	for _, q := range rows {
		out = append(out, folderQuotaToResponse(q, sourceName))
	}
	return RenderJSON(w, r, out)
}

func quotasPostHandler(w http.ResponseWriter, r *http.Request, d *Context) (int, error) {
	var body struct {
		Source     string `json:"source"`
		Path       string `json:"path"`
		UserID     uint64 `json:"userId"`
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
	userscope, err := d.User.GetScopeForSourceName(body.Source)
	if err != nil {
		return http.StatusForbidden, err
	}
	fullPath := utils.JoinScopedIndexPath(userscope, clean)
	q, err := state.CreateFolderQuota(sourceInfo.Path, fullPath, body.UserID, body.LimitBytes, body.Meter)
	if err != nil {
		return http.StatusBadRequest, err
	}
	activity.RecordQuotaCreate(r, toActor(d), body.Source, clean, activity.QuotaFolderCreateChanges(*q))
	return RenderJSON(w, r, folderQuotaToResponse(*q, body.Source))
}

func quotasPatchHandler(w http.ResponseWriter, r *http.Request, d *Context) (int, error) {
	id := r.PathValue("id")
	if id == "" {
		return http.StatusBadRequest, fmt.Errorf("id is required")
	}
	var body struct {
		UserID     uint64 `json:"userId"`
		LimitBytes int64  `json:"limitBytes"`
		Meter      string `json:"meter"`
	}
	if err := decodeJSON(r, &body); err != nil {
		return http.StatusBadRequest, err
	}
	before, err := state.GetFolderQuotaByID(id)
	if err != nil {
		return http.StatusNotFound, err
	}
	q, err := state.UpdateFolderQuota(id, body.UserID, body.LimitBytes, body.Meter)
	if err != nil {
		return http.StatusNotFound, err
	}
	sourceName := q.Source
	if src, ok := settings.Config.Server.SourceMap[q.Source]; ok {
		sourceName = src.Name
	}
	userscope, err := d.User.GetScopeForSourceName(sourceName)
	if err != nil {
		return http.StatusForbidden, err
	}
	displayPath := quotaClientPathFromIndex(userscope, q.Path)
	changes := activity.QuotaFolderUpdateChanges(*before, *q)
	if len(changes) > 0 {
		activity.RecordQuotaUpdate(r, toActor(d), sourceName, displayPath, changes)
	}
	return RenderJSON(w, r, folderQuotaToResponse(*q, sourceName))
}

func quotasDeleteHandler(w http.ResponseWriter, r *http.Request, d *Context) (int, error) {
	id := r.PathValue("id")
	if id == "" {
		return http.StatusBadRequest, fmt.Errorf("id is required")
	}
	before, err := state.GetFolderQuotaByID(id)
	if err != nil {
		return http.StatusNotFound, err
	}
	sourceName := before.Source
	if src, ok := settings.Config.Server.SourceMap[before.Source]; ok {
		sourceName = src.Name
	}
	userscope, err := d.User.GetScopeForSourceName(sourceName)
	if err != nil {
		return http.StatusForbidden, err
	}
	displayPath := quotaClientPathFromIndex(userscope, before.Path)
	if err := state.DeleteFolderQuota(id); err != nil {
		return http.StatusNotFound, err
	}
	activity.RecordQuotaDelete(r, toActor(d), sourceName, displayPath, activity.QuotaFolderDeleteChanges(*before))
	return http.StatusNoContent, nil
}

func userQuotaSnapshotHandler(w http.ResponseWriter, r *http.Request, d *Context) (int, error) {
	sourceName := r.URL.Query().Get("source")
	if sourceName == "" {
		return http.StatusBadRequest, fmt.Errorf("source is required")
	}
	username := r.PathValue("username")
	target := d.User
	if username != "" && username != d.User.Username {
		if !d.User.Permissions.Admin {
			return http.StatusForbidden, fmt.Errorf("admin required")
		}
		u, err := state.GetUserByUsername(username)
		if err != nil {
			return http.StatusNotFound, err
		}
		target = &u
	}
	for _, bs := range target.BackendScopes {
		sourceInfo, ok := settings.Config.Server.NameToSource[sourceName]
		if !ok {
			return http.StatusBadRequest, fmt.Errorf("invalid source")
		}
		if bs.Path != sourceInfo.Path || bs.Quota == nil {
			continue
		}
		return RenderJSON(w, r, state.ScopeQuotaSnapshot(target, sourceName, bs.Quota))
	}
	return http.StatusNotFound, fmt.Errorf("no scope quota for source")
}

func decodeJSON(r *http.Request, dest interface{}) error {
	return json.NewDecoder(r.Body).Decode(dest)
}

func folderQuotaToResponse(q quota.FolderQuota, sourceName string) folderQuotaResponse {
	snap := state.FolderQuotaSnapshot(q, sourceName)
	return folderQuotaResponse{
		ID:                q.ID,
		Source:            sourceName,
		Path:              q.Path,
		UserID:            q.UserID,
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
