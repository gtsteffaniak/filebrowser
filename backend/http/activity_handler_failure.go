package http

import (
	"net/http"
	"strings"

	activitydb "github.com/gtsteffaniak/filebrowser/backend/database/activity"
)

// invokeHandler runs a terminal API handler and records handler-level failures.
// Middleware errors returned before the handler is invoked are not logged here.
func invokeHandler(fn handleFunc, w http.ResponseWriter, r *http.Request, d *requestContext) (int, error) {
	status, err := fn(w, r, d)
	if err != nil {
		if d != nil && d.handlerFailureRecorded {
			return status, err
		}
		if d != nil {
			d.handlerFailureRecorded = true
		}
		recordHandlerFailureActivity(r, d, status, err)
	}
	return status, err
}

func shouldRecordHandlerFailure(r *http.Request, status int) bool {
	if r == nil || status < 400 {
		return false
	}
	path := r.URL.Path
	switch {
	case strings.Contains(path, "/tools/activity"):
		return false
	case path == "/health" || strings.HasSuffix(path, "/health"):
		return false
	case strings.Contains(path, "/events"):
		return false
	default:
		return true
	}
}

func inferActivityEventTypeFromRequest(r *http.Request) (activitydb.EventType, bool) {
	if r == nil {
		return "", false
	}
	path := strings.TrimSuffix(r.URL.Path, "/")
	method := r.Method

	switch {
	case strings.Contains(path, "duplicateFinder"):
		return activitydb.EventDuplicateFinder, true
	case path == "/users" || strings.HasSuffix(path, "/users"):
		switch method {
		case http.MethodPost:
			return activitydb.EventUserCreate, true
		case http.MethodPut, http.MethodPatch:
			return activitydb.EventUserUpdate, true
		case http.MethodDelete:
			return activitydb.EventUserDelete, true
		}
	case strings.Contains(path, "/auth/token"):
		switch method {
		case http.MethodPost:
			return activitydb.EventTokenCreate, true
		case http.MethodDelete:
			return activitydb.EventTokenDelete, true
		}
	case strings.Contains(path, "/auth/login"):
		return activitydb.EventLogin, true
	case strings.Contains(path, "/auth/logout"):
		return activitydb.EventLogout, true
	case strings.Contains(path, "/auth/signup"):
		return activitydb.EventSignup, true
	case strings.Contains(path, "/auth/passkey"):
		switch method {
		case http.MethodPost:
			return activitydb.EventPasskeyRegister, true
		case http.MethodDelete:
			return activitydb.EventPasskeyDelete, true
		}
	case strings.Contains(path, "/share"):
		switch method {
		case http.MethodPost:
			return activitydb.EventShareCreate, true
		case http.MethodPatch, http.MethodPut:
			return activitydb.EventShareUpdate, true
		case http.MethodDelete:
			return activitydb.EventShareDelete, true
		}
	case strings.Contains(path, "/access"):
		return activitydb.EventAccessUpdate, true
	case strings.Contains(path, "/resources"):
		return inferResourceFailureEventType(method, path)
	}
	return "", false
}

func inferResourceFailureEventType(method, path string) (activitydb.EventType, bool) {
	switch method {
	case http.MethodDelete:
		if strings.Contains(path, "bulk") {
			return activitydb.EventBulkDelete, true
		}
		return activitydb.EventDelete, true
	case http.MethodPost:
		if strings.Contains(path, "unarchive") {
			return activitydb.EventUnarchive, true
		}
		if strings.Contains(path, "archive") {
			return activitydb.EventArchive, true
		}
		return activitydb.EventUpload, true
	case http.MethodPut:
		return activitydb.EventUpload, true
	case http.MethodPatch:
		return activitydb.EventMove, true
	case http.MethodGet:
		if strings.Contains(path, "download") || path == "/raw" || strings.HasSuffix(path, "/raw") {
			return activitydb.EventDownload, true
		}
	}
	return "", false
}

func recordHandlerFailureActivity(r *http.Request, d *requestContext, status int, err error) {
	if d == nil || d.user == nil || d.user.ID == 0 || err == nil {
		return
	}
	if !shouldRecordHandlerFailure(r, status) {
		return
	}
	if status == 0 {
		status = http.StatusInternalServerError
	}

	eventType, ok := inferActivityEventTypeFromRequest(r)
	if !ok {
		return
	}

	details := activitydb.Details{
		Method:      r.Method,
		RequestPath: requestActivityPath(r),
		Error:       err.Error(),
	}
	if username := strings.TrimSpace(r.URL.Query().Get("username")); username != "" {
		details.TargetUsername = username
	}

	recordUserActivity(r, d, activitydb.Entry{
		EventType: eventType,
		Status:    status,
		Details:   details,
	})
}

func requestActivityPath(r *http.Request) string {
	if r == nil {
		return ""
	}
	path := r.URL.Path
	if r.URL.RawQuery != "" {
		return path + "?" + r.URL.RawQuery
	}
	return path
}
