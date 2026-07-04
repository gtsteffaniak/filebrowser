package web

import (
	"encoding/csv"
	"encoding/json"
	"fmt"
	"net/http"

	"github.com/gtsteffaniak/filebrowser/backend/internal/activity"
	activitydb "github.com/gtsteffaniak/filebrowser/backend/internal/database/activity"
	"github.com/gtsteffaniak/filebrowser/backend/internal/state"
	"github.com/gtsteffaniak/filebrowser/backend/internal/utils"
)

func toActor(d *Context) *activity.Actor {
	if d == nil {
		return nil
	}
	return &activity.Actor{User: d.User, Share: d.Share, Token: d.Token}
}

// ListHandler returns paginated activity events (ungrouped).
func ListHandler(w http.ResponseWriter, r *http.Request, d *Context) (int, error) {
	filter, status, err := activity.ParseFilter(r, toActor(d))
	if err != nil {
		return status, err
	}
	activity.ClampListPaging(&filter)

	items, total, err := state.ListActivity(filter)
	if err != nil {
		return http.StatusInternalServerError, err
	}
	activity.PrepareItemsForViewer(items, d.User)
	totalPages := (total + filter.Limit - 1) / filter.Limit
	if totalPages == 0 {
		totalPages = 1
	}

	return RenderJSON(w, r, activitydb.ListResponse{
		Items:      utils.NonNilSlice(items),
		Total:      total,
		Page:       filter.Page,
		Limit:      filter.Limit,
		TotalPages: totalPages,
	})
}

// GroupedHandler returns aggregated activity buckets for charts.
func GroupedHandler(w http.ResponseWriter, r *http.Request, d *Context) (int, error) {
	filter, status, err := activity.ParseFilter(r, toActor(d))
	if err != nil {
		return status, err
	}
	if err = activity.ValidateChartParams(filter); err != nil {
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
	return RenderJSON(w, r, activitydb.GroupedResponse{Buckets: utils.NonNilSlice(buckets)})
}

// ExportHandler streams activity rows as CSV.
func ExportHandler(w http.ResponseWriter, r *http.Request, d *Context) (int, error) {
	filter, status, err := activity.ParseFilter(r, toActor(d))
	if err != nil {
		return status, err
	}
	optionalCols, err := activity.ParseExportRows(r.URL.Query().Get("rows"))
	if err != nil {
		return http.StatusBadRequest, err
	}
	filter.Page = 1
	filter.Limit = utils.Clamp(1000, 1, 500)

	w.Header().Set("Content-Type", "text/csv; charset=utf-8")
	w.Header().Set("Content-Disposition", fmt.Sprintf(
		`attachment; filename="activity-%d-%d.csv"`, filter.From, filter.To))

	cw := csv.NewWriter(w)
	includeDetails := d.User.Permissions.Admin
	_ = cw.Write(activity.ExportHeader(includeDetails, optionalCols))

	exported := 0
	for {
		items, total, err := state.ListActivity(filter)
		if err != nil {
			return http.StatusInternalServerError, err
		}
		if len(items) == 0 {
			break
		}
		activity.PrepareItemsForViewer(items, d.User)
		for _, item := range items {
			if exported >= activity.MaxExportRows {
				_ = cw.Write([]string{"", "", "", "TRUNCATED"})
				cw.Flush()
				return 0, nil
			}
			detailsJSON := ""
			if includeDetails {
				detailsJSON = string(mustJSON(item.Details))
			}
			_ = cw.Write(activity.ExportRowValues(item, optionalCols, includeDetails, detailsJSON))
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

func mustJSON(v any) []byte {
	b, err := json.Marshal(v)
	if err != nil {
		return []byte("{}")
	}
	return b
}
