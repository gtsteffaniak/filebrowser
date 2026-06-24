package http

import (
	"net/http"
	"strconv"
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
	applyActivityAuthContext(d, &entry)
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

func recordDownloadActivity(r *http.Request, d *requestContext, source string, fileList []string) {
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

func recordPatchItemActivity(r *http.Request, d *requestContext, action string, item MoveCopyItem) {
	eventType, ok := activitydb.EventTypeFromAction(action)
	if !ok {
		return
	}
	details := activitydb.Details{
		Source:     item.FromSource,
		Path:       item.FromPath,
		TargetPath: item.ToPath,
	}
	entry := activitydb.Entry{
		EventType:  eventType,
		Source:     item.FromSource,
		Path:       item.FromPath,
		TargetPath: item.ToPath,
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
	details := activitydb.Details{
		Source: source,
		Path:   pathLabel,
	}
	recordUserActivity(r, d, activitydb.Entry{
		EventType: activitydb.EventUpload,
		Source:    source,
		Path:      pathLabel,
		Details:   details,
	})
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

func recordAccessCreate(r *http.Request, d *requestContext, source, path string, changes []activitydb.FieldChange) {
	recordUserActivity(r, d, activitydb.Entry{
		EventType: activitydb.EventAccessCreate,
		Source:    source,
		Path:      path,
		Details: activitydb.Details{
			Source:  source,
			Path:    path,
			Changes: changes,
		},
	})
}

func recordAccessUpdate(r *http.Request, d *requestContext, source, path string, changes []activitydb.FieldChange) {
	recordUserActivity(r, d, activitydb.Entry{
		EventType: activitydb.EventAccessUpdate,
		Source:    source,
		Path:      path,
		Details: activitydb.Details{
			Source:  source,
			Path:    path,
			Changes: changes,
		},
	})
}

func recordAccessDelete(r *http.Request, d *requestContext, source, path string, changes []activitydb.FieldChange) {
	recordUserActivity(r, d, activitydb.Entry{
		EventType: activitydb.EventAccessDelete,
		Source:    source,
		Path:      path,
		Details: activitydb.Details{
			Source:  source,
			Path:    path,
			Changes: changes,
		},
	})
}

func accessRuleCreateChanges(allow bool, ruleCategory, value string) []activitydb.FieldChange {
	ruleType := "deny"
	if allow {
		ruleType = "allow"
	}
	changes := []activitydb.FieldChange{
		{Field: "ruleType", To: ruleType},
		{Field: "ruleCategory", To: ruleCategory},
	}
	if value != "" {
		changes = append(changes, activitydb.FieldChange{Field: "value", To: value})
	}
	return changes
}

func accessRuleDeleteChanges(ruleType, ruleCategory, value string, cascade bool, count int) []activitydb.FieldChange {
	changes := []activitydb.FieldChange{
		{Field: "ruleType", To: ruleType},
		{Field: "ruleCategory", To: ruleCategory},
	}
	if value != "" {
		changes = append(changes, activitydb.FieldChange{Field: "value", To: value})
	}
	if cascade {
		changes = append(changes, activitydb.FieldChange{Field: "cascade", To: "true"})
		if count > 0 {
			changes = append(changes, activitydb.FieldChange{Field: "count", To: strconv.Itoa(count)})
		}
	}
	return changes
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

func recordAuthActivity(r *http.Request, user *users.User, eventType activitydb.EventType, details activitydb.Details) {
	if user == nil || user.ID == 0 {
		return
	}
	entry := activitydb.Entry{
		UserID:    user.ID,
		EventType: eventType,
		Details:   details,
		CreatedAt: time.Now().Unix(),
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
	recordAuthActivity(r, user, activitydb.EventLogin, activitydb.Details{
		LoginMethod: string(user.LoginMethod),
	})
}

func recordToolActivity(r *http.Request, d *requestContext, eventType activitydb.EventType, details activitydb.Details) {
	recordUserActivity(r, d, activitydb.Entry{
		EventType: eventType,
		Details:   details,
	})
}
