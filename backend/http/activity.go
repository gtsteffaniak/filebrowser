package http

import (
	"encoding/csv"
	"encoding/json"
	"fmt"
	"net/http"
	"strconv"
	"strings"
	"time"

	"github.com/gtsteffaniak/filebrowser/backend/common/utils"
	activitydb "github.com/gtsteffaniak/filebrowser/backend/database/activity"
	"github.com/gtsteffaniak/filebrowser/backend/database/users"
	"github.com/gtsteffaniak/filebrowser/backend/state"
)

const (
	activityMaxExportRows      = 100000
	activityMaxChartDays       = 90
	activityMaxMinuteRangeSecs = 2 * 86400 // 48 hours
)

// clampActivityListPaging normalizes page/limit before database queries.
func clampActivityListPaging(filter *activitydb.QueryFilter) {
	limit := filter.Limit
	if limit <= 0 {
		limit = 100
	}
	filter.Limit = utils.Clamp(limit, 1, 500)
	if filter.Page < 1 {
		filter.Page = 1
	}
}

func parseActivityFilter(r *http.Request, d *requestContext) (activitydb.QueryFilter, int, error) {
	if status, err := enforceActivityScope(r, d); err != nil {
		return activitydb.QueryFilter{}, status, err
	}
	if status, err := enforceActivityPathGlobFilter(r, d); err != nil {
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

	if d.user.Permissions.Admin {
		if username := strings.TrimSpace(q.Get("username")); username != "" {
			if username == users.AnonymousUserName {
				filter.UserID = 0
				filter.UserFilter = true
			} else {
				u, err := state.GetUserByUsername(username)
				if err != nil {
					return activitydb.QueryFilter{}, http.StatusBadRequest, fmt.Errorf("user not found: %s", username)
				}
				filter.UserID = u.ID
				filter.UserFilter = true
			}
		}
	} else {
		filter.UserID = d.user.ID
		filter.UserFilter = true
	}

	scope := strings.TrimSpace(q.Get("scope"))
	if scope == "" {
		scope = "all"
	}
	filter.Scope = scope

	if status, err := resolveActivityShareAccess(d, &filter); err != nil {
		return activitydb.QueryFilter{}, status, err
	}

	resolvedTypes, err := activitydb.ResolveScopeEventTypes(scope, filter.EventTypes)
	if err != nil {
		return activitydb.QueryFilter{}, http.StatusBadRequest, err
	}
	filter.EventTypes = resolvedTypes

	if status, err := resolveActivityPathFilters(d, &filter); err != nil {
		return activitydb.QueryFilter{}, status, err
	}

	return filter, 0, nil
}

// enforceActivityScope rejects non-admin attempts to scope activity to another user.
// Must run before any filter parsing so user-scoping params cannot influence queries.
func enforceActivityScope(r *http.Request, d *requestContext) (int, error) {
	if d == nil || d.user == nil || d.user.ID == 0 {
		return http.StatusUnauthorized, fmt.Errorf("authentication required")
	}
	if d.user.Permissions.Admin {
		return 0, nil
	}

	q := r.URL.Query()
	if userIDParam := strings.TrimSpace(q.Get("userId")); userIDParam != "" {
		requestedID, err := strconv.ParseUint(userIDParam, 10, 64)
		if err != nil || requestedID != d.user.ID {
			return http.StatusForbidden, fmt.Errorf("forbidden: cannot query activity for another user")
		}
	}
	if username := strings.TrimSpace(q.Get("username")); username != "" && username != d.user.Username {
		return http.StatusForbidden, fmt.Errorf("forbidden: cannot query activity for another user")
	}
	return 0, nil
}

func redactActivityItems(items []activitydb.FrontendEntry, admin bool) []activitydb.FrontendEntry {
	if admin {
		return items
	}
	for i := range items {
		items[i].Details = activitydb.FrontendDetails{}
	}
	return items
}

func validateActivityChartParams(filter activitydb.QueryFilter) error {
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
	if interval == "minute" && rangeSecs > activityMaxMinuteRangeSecs {
		return fmt.Errorf("minute interval supports at most 48 hours; use hour or day for longer ranges")
	}
	if interval != "none" && rangeSecs > int64(activityMaxChartDays)*86400 {
		return fmt.Errorf("time range exceeds %d days for chart queries", activityMaxChartDays)
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

// activityListHandler returns paginated activity events (ungrouped).
//
// @Summary List activity events
// @Description Returns individual activity log rows, newest first. Details are included for admins only.
// @Tags Tools
// @Produce json
// @Param from query int false "Start unix timestamp (default: 7 days ago)"
// @Param to query int false "End unix timestamp (default: now)"
// @Param scope query string false "Event category: all, files, or shares (default: all)"
// @Param eventType query string false "Filter by event type (comma-separated)"
// @Param username query string false "Filter by username (admin only)"
// @Param source query string false "Filter by source name"
// @Param path query string false "Path prefix filter"
// @Param pathGlob query string false "Path glob filter (admin only)"
// @Param shareHash query string false "Share hash filter (owned shares for non-admins)"
// @Param page query int false "Page number (default: 1)"
// @Param limit query int false "Page size, max 500 (default: 100)"
// @Success 200 {object} activitydb.ListResponse
// @Failure 400 {object} map[string]string
// @Failure 403 {object} map[string]string
// @Router /api/tools/activity [get]
func activityListHandler(w http.ResponseWriter, r *http.Request, d *requestContext) (int, error) {
	filter, status, err := parseActivityFilter(r, d)
	if err != nil {
		return status, err
	}
	clampActivityListPaging(&filter)

	items, total, err := state.ListActivity(filter)
	if err != nil {
		return http.StatusInternalServerError, err
	}
	items = redactActivityItems(items, d.user.Permissions.Admin)
	totalPages := (total + filter.Limit - 1) / filter.Limit
	if totalPages == 0 {
		totalPages = 1
	}

	return renderJSON(w, r, activitydb.ListResponse{
		Items:      items,
		Total:      total,
		Page:       filter.Page,
		Limit:      filter.Limit,
		TotalPages: totalPages,
	})
}

// activityGroupedHandler returns aggregated activity buckets for charts.
//
// @Summary Grouped activity statistics
// @Description Returns time-bucketed activity counts for charts. Use interval and splitBy to control grouping.
// @Tags Tools
// @Produce json
// @Param from query int false "Start unix timestamp (default: 7 days ago)"
// @Param to query int false "End unix timestamp (default: now)"
// @Param scope query string false "Event category: all, files, or shares (default: all)"
// @Param eventType query string false "Filter by event type (comma-separated)"
// @Param username query string false "Filter by username (admin only)"
// @Param source query string false "Filter by source name"
// @Param path query string false "Path prefix filter"
// @Param pathGlob query string false "Path glob filter (admin only)"
// @Param shareHash query string false "Share hash filter (owned shares for non-admins)"
// @Param interval query string false "Time bucket: minute, hour, day, or none (default: hour)"
// @Param splitBy query string false "Series dimension: eventType, user, or none (default: eventType)"
// @Success 200 {object} activitydb.StatsResponse
// @Failure 400 {object} map[string]string
// @Failure 403 {object} map[string]string
// @Router /api/tools/activity/grouped [get]
func activityGroupedHandler(w http.ResponseWriter, r *http.Request, d *requestContext) (int, error) {
	filter, status, err := parseActivityFilter(r, d)
	if err != nil {
		return status, err
	}
	if err = validateActivityChartParams(filter); err != nil {
		return http.StatusBadRequest, err
	}
	if filter.Interval == "" && filter.GroupBy == "" {
		filter.Interval = "hour"
	}
	if filter.SplitBy == "" {
		filter.SplitBy = "eventType"
	}

	buckets, err := state.ListActivityStats(filter)
	if err != nil {
		return http.StatusInternalServerError, err
	}
	return renderJSON(w, r, activitydb.GroupedResponse{Buckets: buckets})
}

// activityExportHandler streams activity rows as CSV.
//
// @Summary Export activity as CSV
// @Description Streams matching activity rows as CSV. Details column is populated for admins only.
// @Tags Tools
// @Produce text/csv
// @Param from query int false "Start unix timestamp (default: 7 days ago)"
// @Param to query int false "End unix timestamp (default: now)"
// @Param scope query string false "Event category: all, files, or shares (default: all)"
// @Param eventType query string false "Filter by event type (comma-separated)"
// @Param username query string false "Filter by username (admin only)"
// @Param source query string false "Filter by source name"
// @Param path query string false "Path prefix filter"
// @Param pathGlob query string false "Path glob filter (admin only)"
// @Param shareHash query string false "Share hash filter (owned shares for non-admins)"
// @Success 200 {string} string "CSV file"
// @Failure 400 {object} map[string]string
// @Failure 403 {object} map[string]string
// @Router /api/tools/activity/export [get]
func activityExportHandler(w http.ResponseWriter, r *http.Request, d *requestContext) (int, error) {
	filter, status, err := parseActivityFilter(r, d)
	if err != nil {
		return status, err
	}
	filter.Page = 1
	filter.Limit = utils.Clamp(1000, 1, 500)

	w.Header().Set("Content-Type", "text/csv; charset=utf-8")
	w.Header().Set("Content-Disposition", fmt.Sprintf(
		`attachment; filename="activity-%d-%d.csv"`, filter.From, filter.To))

	cw := csv.NewWriter(w)
	_ = cw.Write([]string{"id", "createdAt", "username", "eventType", "ipAddress", "status", "details"})

	exported := 0
	for {
		items, total, err := state.ListActivity(filter)
		if err != nil {
			return http.StatusInternalServerError, err
		}
		if len(items) == 0 {
			break
		}
		for _, item := range items {
			if exported >= activityMaxExportRows {
				_ = cw.Write([]string{"", "", "", "TRUNCATED", "", "", ""})
				cw.Flush()
				return 0, nil
			}
			item = redactActivityItems([]activitydb.FrontendEntry{item}, d.user.Permissions.Admin)[0]
			detailsJSON, _ := json.Marshal(item.Details)
			_ = cw.Write([]string{
				strconv.FormatInt(item.ID, 10),
				strconv.FormatInt(item.CreatedAt, 10),
				item.Username,
				string(item.EventType),
				item.IPAddress,
				strconv.Itoa(item.Status),
				string(detailsJSON),
			})
			exported++
		}
		if filter.Page*filter.Limit >= total {
			break
		}
		filter.Page++
	}
	cw.Flush()
	return 0, nil
}
