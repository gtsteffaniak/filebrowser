package http

import (
	"net/http"
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
	entry.Success = entry.Status >= 200 && entry.Status < 400
	state.RecordActivity(entry)
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
	entry := activitydb.Entry{
		EventType:  eventType,
		Source:     item.FromSource,
		Path:       item.FromPath,
		TargetPath: item.ToPath,
		Status:     status,
		Details: activitydb.Details{
			Source:     item.FromSource,
			Path:       item.FromPath,
			TargetPath: item.ToPath,
		},
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

func recordUploadActivity(r *http.Request, d *requestContext, source, path string, isDir bool) {
	pathLabel := path
	if isDir {
		pathLabel = path + "/"
	}
	recordUserActivity(r, d, activitydb.Entry{
		EventType: activitydb.EventUpload,
		Source:    source,
		Path:      pathLabel,
		Details: activitydb.Details{
			Source: source,
			Path:   pathLabel,
		},
	})
}

func recordShareMutation(r *http.Request, d *requestContext, eventType activitydb.EventType, hash, sourceName, path string) {
	recordUserActivity(r, d, activitydb.Entry{
		EventType: eventType,
		Source:    sourceName,
		Path:      path,
		Details: activitydb.Details{
			Source:    sourceName,
			Path:      path,
			ShareHash: hash,
		},
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

func recordUserMutation(r *http.Request, d *requestContext, eventType activitydb.EventType, target *users.User) {
	if d == nil || d.user == nil || target == nil {
		return
	}
	recordUserActivity(r, d, activitydb.Entry{
		EventType: eventType,
		Details: activitydb.Details{
			TargetUsername: target.Username,
			Scopes:         scopesToActivityDetails(target),
		},
	})
}

func recordTokenMutation(r *http.Request, d *requestContext, eventType activitydb.EventType, tokenName string) {
	recordUserActivity(r, d, activitydb.Entry{
		EventType: eventType,
		Details: activitydb.Details{
			TokenName: tokenName,
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
