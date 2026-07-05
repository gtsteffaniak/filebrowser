package activity

import (
	"net/http"
	"strconv"
	"strings"
	"time"

	activitydb "github.com/gtsteffaniak/filebrowser/backend/internal/database/activity"
	"github.com/gtsteffaniak/filebrowser/backend/internal/database/users"
)

// MoveCopyItem identifies a single move/copy/rename for activity logging.
type MoveCopyItem struct {
	FromSource string
	FromPath   string
	ToPath     string
}

// BulkDeleteItem identifies a deleted path for activity logging.
type BulkDeleteItem struct {
	Source string
	Path   string
}

type activityActorPolicy int

const (
	activityActorUser activityActorPolicy = iota
	activityActorAny
	activityActorShareOwner
)

func recordEntry(entry activitydb.Entry) {
	Record(entry)
}

func finalizeActivityEntry(r *http.Request, actor *Actor, entry activitydb.Entry, policy activityActorPolicy) {
	switch policy {
	case activityActorUser:
		if actor == nil || actor.User == nil || actor.User.ID == 0 {
			return
		}
		if entry.UserID == 0 {
			entry.UserID = actor.User.ID
		}
	case activityActorAny:
		if actor == nil {
			return
		}
		if actor.User != nil && entry.UserID == 0 {
			entry.UserID = actor.User.ID
		}
	case activityActorShareOwner:
		if actor == nil || actor.Share.Hash == "" || actor.Share.UserID == 0 {
			return
		}
		entry.UserID = actor.Share.UserID
		entry.Details.ShareHash = actor.Share.Hash
		entry.Details.ShareOwnerUserID = actor.Share.UserID
	}

	if entry.CreatedAt == 0 {
		entry.CreatedAt = time.Now().Unix()
	}
	if entry.IPAddress == "" && r != nil {
		entry.IPAddress = remoteIP(r)
	}
	applyActivityAuthContext(actor, &entry)
	recordEntry(entry)
}

func applyActivityAuthContext(actor *Actor, entry *activitydb.Entry) {
	if actor == nil || actor.Token == "" {
		return
	}
	if actor.User != nil {
		if name, ok := users.TokenNameByRaw(actor.User.Tokens, actor.Token); ok {
			entry.Details.TokenName = name
			if strings.HasPrefix(name, "WEB_TOKEN") {
				entry.Details.AuthMethod = "webToken"
			} else {
				entry.Details.AuthMethod = "apiKey"
			}
			return
		}
	}
	entry.Details.AuthMethod = "webToken"
}

func RecordUser(r *http.Request, actor *Actor, entry activitydb.Entry) {
	finalizeActivityEntry(r, actor, entry, activityActorUser)
}

func RecordActor(r *http.Request, actor *Actor, entry activitydb.Entry) {
	finalizeActivityEntry(r, actor, entry, activityActorAny)
}

func RecordShareOwner(r *http.Request, actor *Actor, entry activitydb.Entry) {
	finalizeActivityEntry(r, actor, entry, activityActorShareOwner)
}

func RecordWebDAVUser(r *http.Request, user *users.User, entry activitydb.Entry) {
	if user == nil || user.ID == 0 {
		return
	}
	actor := &Actor{User: user}
	if entry.UserID == 0 {
		entry.UserID = user.ID
	}
	finalizeActivityEntry(r, actor, entry, activityActorUser)
}

func RecordDownload(r *http.Request, actor *Actor, source string, fileList []string) {
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
	if actor != nil && actor.Share.Hash != "" {
		entry.Details.ShareHash = actor.Share.Hash
		entry.Details.ShareOwnerUserID = actor.Share.UserID
	}
	RecordActor(r, actor, entry)
}

func RecordPatchItem(r *http.Request, actor *Actor, action string, item MoveCopyItem) {
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
	if actor != nil && actor.Share.Hash != "" {
		RecordShareOwner(r, actor, entry)
		return
	}
	RecordUser(r, actor, entry)
}

func RecordDelete(r *http.Request, actor *Actor, source, path string) {
	RecordUser(r, actor, activitydb.Entry{
		EventType: activitydb.EventDelete,
		Source:    source,
		Path:      path,
		Details: activitydb.Details{
			Source: source,
			Path:   path,
		},
	})
}

func RecordBulkDelete(r *http.Request, actor *Actor, succeeded []BulkDeleteItem) {
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
	RecordUser(r, actor, entry)
}

func RecordUpload(r *http.Request, actor *Actor, source, path string, isDir bool) {
	pathLabel := path
	if isDir {
		pathLabel = path + "/"
	}
	details := activitydb.Details{
		Source: source,
		Path:   pathLabel,
	}
	RecordUser(r, actor, activitydb.Entry{
		EventType: activitydb.EventUpload,
		Source:    source,
		Path:      pathLabel,
		Details:   details,
	})
}

func RecordShareMutation(r *http.Request, actor *Actor, eventType activitydb.EventType, hash, sourceName, path string, changes []activitydb.FieldChange) {
	details := activitydb.Details{
		Source:    sourceName,
		Path:      path,
		ShareHash: hash,
		Changes:   changes,
	}
	RecordUser(r, actor, activitydb.Entry{
		EventType: eventType,
		Source:    sourceName,
		Path:      path,
		Details:   details,
	})
}

func RecordArchive(r *http.Request, actor *Actor, eventType activitydb.EventType, source, path string, paths []string) {
	details := activitydb.Details{
		Source:    source,
		Path:      path,
		FileCount: len(paths),
		Paths:     append([]string(nil), paths...),
	}
	details.CapPaths()
	RecordUser(r, actor, activitydb.Entry{
		EventType: eventType,
		Source:    source,
		Path:      path,
		Details:   details,
	})
}

func RecordAccessCreate(r *http.Request, actor *Actor, source, path string, changes []activitydb.FieldChange) {
	RecordUser(r, actor, activitydb.Entry{
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

func RecordAccessUpdate(r *http.Request, actor *Actor, source, path string, changes []activitydb.FieldChange) {
	RecordUser(r, actor, activitydb.Entry{
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

func RecordAccessDelete(r *http.Request, actor *Actor, source, path string, changes []activitydb.FieldChange) {
	RecordUser(r, actor, activitydb.Entry{
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

func AccessRuleCreateChanges(allow bool, ruleCategory, value string) []activitydb.FieldChange {
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

func AccessRuleDeleteChanges(ruleType, ruleCategory, value string, cascade bool, count int) []activitydb.FieldChange {
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

func RecordUserMutation(r *http.Request, actor *Actor, eventType activitydb.EventType, target *users.User, changes []activitydb.FieldChange) {
	if actor == nil || actor.User == nil || target == nil {
		return
	}
	details := activitydb.Details{
		TargetUsername: target.Username,
		Changes:        changes,
	}
	if eventType == activitydb.EventUserCreate {
		details.Scopes = scopesToActivityDetails(target)
	}
	RecordUser(r, actor, activitydb.Entry{
		EventType: eventType,
		Details:   details,
	})
}

func RecordTokenMutation(r *http.Request, actor *Actor, eventType activitydb.EventType, affectedTokenName string) {
	RecordUser(r, actor, activitydb.Entry{
		EventType: eventType,
		Details: activitydb.Details{
			AffectedTokenName: affectedTokenName,
		},
	})
}

func RecordAuth(r *http.Request, user *users.User, eventType activitydb.EventType, details activitydb.Details) {
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
		entry.IPAddress = remoteIP(r)
	}
	recordEntry(entry)
}

func RecordLogin(r *http.Request, user *users.User) {
	if user == nil {
		return
	}
	RecordAuth(r, user, activitydb.EventLogin, activitydb.Details{
		LoginMethod: string(user.LoginMethod),
	})
}

func RecordTool(r *http.Request, actor *Actor, eventType activitydb.EventType, details activitydb.Details) {
	RecordUser(r, actor, activitydb.Entry{
		EventType: eventType,
		Details:   details,
	})
}
