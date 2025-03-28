package http

import (
	"net/http"

	"github.com/gtsteffaniak/filebrowser/backend/adapters/fs/cache"
	"github.com/gtsteffaniak/filebrowser/backend/indexing"
	"github.com/shirou/gopsutil/v3/disk"
)

func getJobHandler(w http.ResponseWriter, r *http.Request, d *requestContext) (int, error) {
	cacheKey := "usageCache"
	_, ok := cache.DiskUsage.Get(cacheKey).(bool)
	if ok {
		return renderJSON(w, r, indexing.GetIndexesInfo())
	}
	for _, source := range config.Server.Sources {
		usage, err := disk.UsageWithContext(r.Context(), source.Path)
		if err != nil {
			return errToStatus(err), err
		}
		latestUsage := indexing.DiskUsage{
			Total: usage.Total,
			Used:  usage.Used,
		}
		indexing.SetUsage(source.Name, latestUsage)
	}
	cache.DiskUsage.Set(cacheKey, true)

	return renderJSON(w, r, indexing.GetIndexesInfo())
}
