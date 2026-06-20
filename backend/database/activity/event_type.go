package activity

import "fmt"

// EventType identifies a semantic user activity for audit logging.
type EventType string

const (
	EventDownload      EventType = "download"
	EventMove          EventType = "move"
	EventCopy          EventType = "copy"
	EventRename        EventType = "rename"
	EventUpload        EventType = "upload"
	EventDelete        EventType = "delete"
	EventBulkDelete    EventType = "bulkDelete"
	EventArchive       EventType = "archive"
	EventUnarchive     EventType = "unarchive"
	EventShareCreate   EventType = "shareCreate"
	EventShareUpdate   EventType = "shareUpdate"
	EventShareDelete   EventType = "shareDelete"
	// EventShareDownload is deprecated; share downloads are recorded as EventDownload with details.shareHash.
	EventShareDownload EventType = "shareDownload"
	EventUserCreate    EventType = "userCreate"
	EventUserUpdate    EventType = "userUpdate"
	EventAccessUpdate  EventType = "accessUpdate"
	EventLogin             EventType = "login"
	EventLogout            EventType = "logout"
	EventSignup            EventType = "signup"
	EventPasskeyRegister   EventType = "passkeyRegister"
	EventPasskeyDelete     EventType = "passkeyDelete"
	EventTokenCreate       EventType = "tokenCreate"
	EventTokenDelete       EventType = "tokenDelete"
	EventDuplicateFinder   EventType = "duplicateFinder"
)

// AllEventTypes lists every defined event type for validation and UI filters.
var AllEventTypes = []EventType{
	EventDownload,
	EventMove,
	EventCopy,
	EventRename,
	EventUpload,
	EventDelete,
	EventBulkDelete,
	EventArchive,
	EventUnarchive,
	EventShareCreate,
	EventShareUpdate,
	EventShareDelete,
	EventUserCreate,
	EventUserUpdate,
	EventAccessUpdate,
	EventLogin,
	EventLogout,
	EventSignup,
	EventPasskeyRegister,
	EventPasskeyDelete,
	EventTokenCreate,
	EventTokenDelete,
	EventDuplicateFinder,
}

// FileEventTypes are file and path operations (scope=files).
var FileEventTypes = []EventType{
	EventDownload,
	EventMove,
	EventCopy,
	EventRename,
	EventUpload,
	EventDelete,
	EventBulkDelete,
	EventArchive,
	EventUnarchive,
	EventAccessUpdate,
}

// ShareEventTypes are share lifecycle events (scope=shares). Share downloads are
// EventDownload rows with a non-empty details.shareHash.
var ShareEventTypes = []EventType{
	EventShareCreate,
	EventShareUpdate,
	EventShareDelete,
}

// ShareScopeEventTypes are valid explicit event-type filters when scope=shares.
var ShareScopeEventTypes = append(append([]EventType{}, ShareEventTypes...), EventDownload)

// ResolveScopeEventTypes returns the effective event-type filter for a scope.
func ResolveScopeEventTypes(scope string, explicit []EventType) ([]EventType, error) {
	var allowed []EventType
	switch scope {
	case "", "all":
		if len(explicit) == 0 {
			return nil, nil
		}
		return explicit, nil
	case "files":
		allowed = FileEventTypes
	case "shares":
		if len(explicit) == 0 {
			return nil, nil
		}
		allowed = ShareScopeEventTypes
	default:
		return nil, fmt.Errorf("scope must be all, files, or shares")
	}
	if len(explicit) == 0 {
		return allowed, nil
	}
	return intersectEventTypes(explicit, allowed), nil
}

func intersectEventTypes(requested, allowed []EventType) []EventType {
	allowedSet := make(map[EventType]struct{}, len(allowed))
	for _, et := range allowed {
		allowedSet[et] = struct{}{}
	}
	var out []EventType
	for _, et := range requested {
		if _, ok := allowedSet[et]; ok {
			out = append(out, et)
		}
	}
	return out
}

// Valid reports whether e is a known event type constant.
func (e EventType) Valid() bool {
	switch e {
	case EventDownload, EventMove, EventCopy, EventRename,
		EventUpload, EventDelete, EventBulkDelete,
		EventArchive, EventUnarchive,
		EventShareCreate, EventShareUpdate, EventShareDelete, EventShareDownload,
		EventUserCreate, EventUserUpdate, EventAccessUpdate,
		EventLogin, EventLogout, EventSignup,
		EventPasskeyRegister, EventPasskeyDelete,
		EventTokenCreate, EventTokenDelete,
		EventDuplicateFinder:
		return true
	default:
		return false
	}
}

// String returns the wire/database representation.
func (e EventType) String() string {
	return string(e)
}

// EventTypeFromAction maps a resource PATCH action string to an event type.
func EventTypeFromAction(action string) (EventType, bool) {
	switch action {
	case "copy":
		return EventCopy, true
	case "move":
		return EventMove, true
	case "rename":
		return EventRename, true
	default:
		return "", false
	}
}
