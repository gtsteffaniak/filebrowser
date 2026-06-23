package http

import (
	"net/http"
	"strings"
	"time"

	activitydb "github.com/gtsteffaniak/filebrowser/backend/database/activity"
	"github.com/gtsteffaniak/filebrowser/backend/database/users"
	"github.com/gtsteffaniak/filebrowser/backend/state"
)

type activityActorPolicy int

const (
	activityActorUser activityActorPolicy = iota
	activityActorAny
	activityActorShareOwner
)

func finalizeActivityEntry(r *http.Request, d *requestContext, entry activitydb.Entry, policy activityActorPolicy) {
	switch policy {
	case activityActorUser:
		if d == nil || d.user == nil || d.user.ID == 0 {
			return
		}
		if entry.UserID == 0 {
			entry.UserID = d.user.ID
		}
	case activityActorAny:
		if d == nil {
			return
		}
		if d.user != nil && entry.UserID == 0 {
			entry.UserID = d.user.ID
		}
	case activityActorShareOwner:
		if d == nil || d.share.Hash == "" || d.share.UserID == 0 {
			return
		}
		entry.UserID = d.share.UserID
		entry.Details.ShareHash = d.share.Hash
		entry.Details.ShareOwnerUserID = d.share.UserID
	}

	if entry.CreatedAt == 0 {
		entry.CreatedAt = time.Now().Unix()
	}
	if entry.IPAddress == "" && r != nil {
		entry.IPAddress = getRemoteIP(r)
	}
	if entry.Status == 0 {
		entry.Status = http.StatusOK
	}
	applyActivityAuthContext(d, &entry)
	entry.Success = entry.Status >= 200 && entry.Status < 400
	state.RecordActivity(entry)
}

func applyActivityAuthContext(d *requestContext, entry *activitydb.Entry) {
	if d == nil || d.token == "" {
		return
	}
	if d.user != nil {
		if name, ok := state.TokenNameForRawToken(d.user, d.token); ok {
			entry.Details.TokenName = name
			if strings.HasPrefix(name, "WEB_TOKEN") {
				entry.Details.AuthMethod = "webToken"
			} else {
				entry.Details.AuthMethod = "apiKey"
			}
			return
		}
	}
	// Ephemeral browser session JWT (login/renew); not persisted in user.Tokens.
	entry.Details.AuthMethod = "webToken"
}

func recordUserActivity(r *http.Request, d *requestContext, entry activitydb.Entry) {
	finalizeActivityEntry(r, d, entry, activityActorUser)
}

func recordActorActivity(r *http.Request, d *requestContext, entry activitydb.Entry) {
	finalizeActivityEntry(r, d, entry, activityActorAny)
}

func recordShareOwnerActivity(r *http.Request, d *requestContext, entry activitydb.Entry) {
	finalizeActivityEntry(r, d, entry, activityActorShareOwner)
}

func recordWebDAVUserActivity(r *http.Request, user *users.User, entry activitydb.Entry) {
	if user == nil || user.ID == 0 {
		return
	}
	d := &requestContext{user: user}
	if entry.UserID == 0 {
		entry.UserID = user.ID
	}
	finalizeActivityEntry(r, d, entry, activityActorUser)
}

func recordDownloadActivity(r *http.Request, d *requestContext, source string, fileList []string, status int) {
	if status == 0 {
		status = http.StatusOK
	}
	details := activitydb.Details{
		Source:    source,
		FileCount: len(fileList),
		Paths:     append([]string(nil), fileList...),
	}
	details.CapPaths()
	if len(fileList) == 1 {
		details.Path = fileList[0]
	}
	entry := activitydb.Entry{
		EventType: activitydb.EventDownload,
		Source:    source,
		Status:    status,
		Details:   details,
	}
	if len(fileList) == 1 {
		entry.Path = fileList[0]
	}
	if d != nil && d.share.Hash != "" {
		entry.Details.ShareHash = d.share.Hash
		entry.Details.ShareOwnerUserID = d.share.UserID
	}
	recordActorActivity(r, d, entry)
}

func recordPatchItemActivity(r *http.Request, d *requestContext, action string, item MoveCopyItem, status int) {
	eventType, ok := activitydb.EventTypeFromAction(action)
	if !ok {
		return
	}
	if status == 0 {
		status = http.StatusOK
	}
	details := activitydb.Details{
		Source:     item.FromSource,
		Path:       item.FromPath,
		TargetPath: item.ToPath,
	}
	if status >= 400 && item.Message != "" {
		details.Error = item.Message
	}
	entry := activitydb.Entry{
		EventType:  eventType,
		Source:     item.FromSource,
		Path:       item.FromPath,
		TargetPath: item.ToPath,
		Status:     status,
		Details:    details,
	}
	if d.share.Hash != "" {
		recordShareOwnerActivity(r, d, entry)
		return
	}
	recordUserActivity(r, d, entry)
}

func recordDeleteActivity(r *http.Request, d *requestContext, source, path string) {
	recordUserActivity(r, d, activitydb.Entry{
		EventType: activitydb.EventDelete,
		Source:    source,
		Path:      path,
		Details: activitydb.Details{
			Source: source,
			Path:   path,
		},
	})
}

func recordBulkDeleteActivity(r *http.Request, d *requestContext, succeeded []BulkDeleteItem) {
	if len(succeeded) == 0 {
		return
	}
	paths := make([]string, 0, len(succeeded))
	for _, item := range succeeded {
		paths = append(paths, item.Path)
	}
	details := activitydb.Details{
		FileCount: len(succeeded),
		Paths:     paths,
	}
	details.CapPaths()
	entry := activitydb.Entry{
		EventType: activitydb.EventBulkDelete,
		Status:    http.StatusOK,
		Details:   details,
	}
	if len(succeeded) == 1 {
		entry.Source = succeeded[0].Source
		entry.Path = succeeded[0].Path
		entry.Details.Source = succeeded[0].Source
		entry.Details.Path = succeeded[0].Path
	}
	recordUserActivity(r, d, entry)
}

func recordUploadActivity(r *http.Request, d *requestContext, source, path string, isDir bool, status int) {
	if status == 0 {
		status = http.StatusOK
	}
	pathLabel := path
	if isDir {
		pathLabel = path + "/"
	}
	details := activitydb.Details{
		Source: source,
		Path:   pathLabel,
	}
	if status >= 400 {
		details.Error = http.StatusText(status)
	}
	recordUserActivity(r, d, activitydb.Entry{
		EventType: activitydb.EventUpload,
		Source:    source,
		Path:      pathLabel,
		Status:    status,
		Details:   details,
	})
}

func patchFailureHTTPStatus(message string) int {
	switch {
	case strings.Contains(message, "access denied"),
		strings.Contains(message, "not allowed"),
		strings.Contains(message, "read-only"):
		return http.StatusForbidden
	case strings.Contains(message, "does not exist"),
		strings.Contains(message, "not found"),
		strings.Contains(message, "not available"):
		return http.StatusNotFound
	case strings.Contains(message, "unsupported action"),
		strings.Contains(message, "invalid"),
		strings.Contains(message, "required"):
		return http.StatusBadRequest
	default:
		return http.StatusBadRequest
	}
}

func recordShareMutation(r *http.Request, d *requestContext, eventType activitydb.EventType, hash, sourceName, path string, changes []activitydb.FieldChange) {
	details := activitydb.Details{
		Source:    sourceName,
		Path:      path,
		ShareHash: hash,
		Changes:   changes,
	}
	recordUserActivity(r, d, activitydb.Entry{
		EventType: eventType,
		Source:    sourceName,
		Path:      path,
		Details:   details,
	})
}

func recordArchiveActivity(r *http.Request, d *requestContext, eventType activitydb.EventType, source, path string, paths []string) {
	details := activitydb.Details{
		Source:    source,
		Path:      path,
		FileCount: len(paths),
		Paths:     append([]string(nil), paths...),
	}
	details.CapPaths()
	recordUserActivity(r, d, activitydb.Entry{
		EventType: eventType,
		Source:    source,
		Path:      path,
		Details:   details,
	})
}

func recordAccessUpdate(r *http.Request, d *requestContext, source, path string) {
	recordUserActivity(r, d, activitydb.Entry{
		EventType: activitydb.EventAccessUpdate,
		Source:    source,
		Path:      path,
		Details: activitydb.Details{
			Source: source,
			Path:   path,
		},
	})
}

func scopesToActivityDetails(target *users.User) []activitydb.ScopeDetail {
	if target == nil {
		return nil
	}
	frontend := target.GetFrontendScopes()
	out := make([]activitydb.ScopeDetail, 0, len(frontend))
	for _, s := range frontend {
		out = append(out, activitydb.ScopeDetail{
			Source: s.Name,
			Path:   s.Scope,
		})
	}
	return out
}

func recordUserMutation(r *http.Request, d *requestContext, eventType activitydb.EventType, target *users.User, changes []activitydb.FieldChange) {
	if d == nil || d.user == nil || target == nil {
		return
	}
	details := activitydb.Details{
		TargetUsername: target.Username,
		Changes:        changes,
	}
	if eventType == activitydb.EventUserCreate {
		details.Scopes = scopesToActivityDetails(target)
	}
	recordUserActivity(r, d, activitydb.Entry{
		EventType: eventType,
		Details:   details,
	})
}

func recordTokenMutation(r *http.Request, d *requestContext, eventType activitydb.EventType, affectedTokenName string) {
	recordUserActivity(r, d, activitydb.Entry{
		EventType: eventType,
		Details: activitydb.Details{
			AffectedTokenName: affectedTokenName,
		},
	})
}

func recordAuthActivity(r *http.Request, user *users.User, eventType activitydb.EventType, status int, details activitydb.Details) {
	if user == nil || user.ID == 0 {
		return
	}
	if status == 0 {
		status = http.StatusOK
	}
	entry := activitydb.Entry{
		UserID:    user.ID,
		EventType: eventType,
		Status:    status,
		Details:   details,
		CreatedAt: time.Now().Unix(),
		Success:   status >= 200 && status < 400,
	}
	if r != nil {
		entry.IPAddress = getRemoteIP(r)
	}
	state.RecordActivity(entry)
}

// recordLoginActivity logs a successful initial authentication (not token refresh).
func recordLoginActivity(r *http.Request, user *users.User) {
	if user == nil {
		return
	}
	recordAuthActivity(r, user, activitydb.EventLogin, http.StatusOK, activitydb.Details{
		LoginMethod: string(user.LoginMethod),
	})
}

func recordToolActivity(r *http.Request, d *requestContext, eventType activitydb.EventType, details activitydb.Details) {
	recordUserActivity(r, d, activitydb.Entry{
		EventType: eventType,
		Details:   details,
	})
}
