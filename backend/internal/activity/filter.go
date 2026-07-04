package activity

import (
	"fmt"
	"net/http"
	"strconv"
	"strings"
	"time"

	"github.com/gtsteffaniak/filebrowser/backend/internal/database/users"
	activitydb "github.com/gtsteffaniak/filebrowser/backend/internal/database/activity"
	"github.com/gtsteffaniak/filebrowser/backend/internal/utils"
)

const (
	maxExportRows      = 100000
	maxChartDays       = 90
	maxMinuteRangeSecs = 2 * 86400 // 48 hours
)

// MaxExportRows is the CSV export row cap.
const MaxExportRows = maxExportRows

// ClampListPaging normalizes page/limit before database queries.
func ClampListPaging(filter *activitydb.QueryFilter) {
	limit := filter.Limit
	if limit <= 0 {
		limit = 100
	}
	filter.Limit = utils.Clamp(limit, 1, 500)
	if filter.Page < 1 {
		filter.Page = 1
	}
}

// ParseFilter builds a query filter from HTTP params and actor context.
func ParseFilter(r *http.Request, actor *Actor) (activitydb.QueryFilter, int, error) {
	if status, err := EnforceScope(r, actor); err != nil {
		return activitydb.QueryFilter{}, status, err
	}
	if status, err := EnforcePathGlobFilter(r, actor); err != nil {
		return activitydb.QueryFilter{}, status, err
	}

	q := r.URL.Query()
	now := time.Now().Unix()

	from := parseInt64Default(q.Get("from"), now-7*86400)
	to := parseInt64Default(q.Get("to"), now)
	if to < from {
		return activitydb.QueryFilter{}, http.StatusBadRequest, fmt.Errorf("to must be >= from")
	}

	filter := activitydb.QueryFilter{
		From:       from,
		To:         to,
		Source:     strings.TrimSpace(q.Get("source")),
		PathPrefix: strings.TrimSpace(q.Get("path")),
		PathGlob:   strings.TrimSpace(q.Get("pathGlob")),
		ShareHash:  strings.TrimSpace(q.Get("shareHash")),
		Page:       parseIntDefault(q.Get("page"), 1),
		Limit:      parseIntDefault(q.Get("limit"), 100),
		Interval:   strings.TrimSpace(q.Get("interval")),
		SplitBy:    strings.TrimSpace(q.Get("splitBy")),
		GroupBy:    strings.TrimSpace(q.Get("groupBy")),
	}

	if et := q.Get("eventType"); et != "" {
		for _, part := range strings.Split(et, ",") {
			part = strings.TrimSpace(part)
			if part == "" {
				continue
			}
			ev := activitydb.EventType(part)
			if !ev.Valid() {
				return activitydb.QueryFilter{}, http.StatusBadRequest, fmt.Errorf("invalid eventType: %s", part)
			}
			filter.EventTypes = append(filter.EventTypes, ev)
		}
	}

	if actor.User.Permissions.Admin {
		if username := strings.TrimSpace(q.Get("username")); username != "" {
			if username == users.AnonymousUserName {
				filter.UserID = 0
				filter.UserFilter = true
			} else {
				uID, err := users.ResolveUsernameToID(username)
				if err != nil {
					return activitydb.QueryFilter{}, http.StatusBadRequest, fmt.Errorf("user not found: %s", username)
				}
				filter.UserID = uID
				filter.UserFilter = true
			}
		}
	} else {
		filter.UserID = actor.User.ID
		filter.UserFilter = true
	}

	scope := strings.TrimSpace(q.Get("scope"))
	if scope == "" {
		scope = "all"
	}
	filter.Scope = scope

	if status, err := ResolveShareAccess(actor, &filter); err != nil {
		return activitydb.QueryFilter{}, status, err
	}

	resolvedTypes, err := activitydb.ResolveScopeEventTypes(scope, filter.EventTypes)
	if err != nil {
		return activitydb.QueryFilter{}, http.StatusBadRequest, err
	}
	filter.EventTypes = resolvedTypes

	if status, err := ResolvePathFilters(actor, &filter); err != nil {
		return activitydb.QueryFilter{}, status, err
	}

	return filter, 0, nil
}

// EnforceScope rejects non-admin attempts to scope activity to another user.
func EnforceScope(r *http.Request, actor *Actor) (int, error) {
	if actor == nil || actor.User == nil || actor.User.ID == 0 {
		return http.StatusUnauthorized, fmt.Errorf("authentication required")
	}
	if actor.User.Permissions.Admin {
		return 0, nil
	}

	q := r.URL.Query()
	if userIDParam := strings.TrimSpace(q.Get("userId")); userIDParam != "" {
		requestedID, err := strconv.ParseUint(userIDParam, 10, 64)
		if err != nil || requestedID != actor.User.ID {
			return http.StatusForbidden, fmt.Errorf("forbidden: cannot query activity for another user")
		}
	}
	if username := strings.TrimSpace(q.Get("username")); username != "" && username != actor.User.Username {
		return http.StatusForbidden, fmt.Errorf("forbidden: cannot query activity for another user")
	}
	return 0, nil
}

// ValidateChartParams checks grouped/chart query parameters.
func ValidateChartParams(filter activitydb.QueryFilter) error {
	interval := filter.Interval
	if interval == "" {
		switch filter.GroupBy {
		case "day":
			interval = "day"
		case "none":
			interval = "none"
		case "hour", "":
			interval = "hour"
		default:
			return fmt.Errorf("groupBy must be none, hour, or day (use interval instead)")
		}
	}
	if interval != "minute" && interval != "hour" && interval != "day" && interval != "none" {
		return fmt.Errorf("interval must be minute, hour, day, or none")
	}

	splitBy := filter.SplitBy
	if splitBy == "" {
		splitBy = "eventType"
	}
	if splitBy != "eventType" && splitBy != "user" && splitBy != "none" {
		return fmt.Errorf("splitBy must be eventType, user, or none")
	}

	rangeSecs := filter.To - filter.From
	if interval == "minute" && rangeSecs > maxMinuteRangeSecs {
		return fmt.Errorf("minute interval supports at most 48 hours; use hour or day for longer ranges")
	}
	if interval != "none" && rangeSecs > int64(maxChartDays)*86400 {
		return fmt.Errorf("time range exceeds %d days for chart queries", maxChartDays)
	}
	return nil
}

func parseInt64Default(s string, def int64) int64 {
	if s == "" {
		return def
	}
	v, err := strconv.ParseInt(s, 10, 64)
	if err != nil {
		return def
	}
	return v
}

func parseIntDefault(s string, def int) int {
	if s == "" {
		return def
	}
	v, err := strconv.Atoi(s)
	if err != nil {
		return def
	}
	return v
}
