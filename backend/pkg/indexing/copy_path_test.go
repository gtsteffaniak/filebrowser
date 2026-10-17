package indexing

import (
	"encoding/json"
	"strings"
	"testing"

	"github.com/gtsteffaniak/filebrowser/backend/internal/utils"
	"github.com/gtsteffaniak/filebrowser/backend/pkg/settings"
)

func TestGetIndexInfoCopyPathPrivacy(t *testing.T) {
	const sourceName = "copy-path-privacy-test"
	const hostPath = "/private/host/source"
	cacheKey := "usageCache-" + sourceName
	// Cache the disk probe so this test needs no filesystem access or index DB.
	utils.DiskUsageCache.Set(cacheKey, true)
	indexesMutex.Lock()
	previous := indexes
	idx := &Index{Source: settings.Source{Name: sourceName, Path: hostPath}}
	indexes = map[string]*Index{sourceName: idx}
	indexesMutex.Unlock()
	t.Cleanup(func() {
		indexesMutex.Lock()
		indexes = previous
		indexesMutex.Unlock()
		utils.DiskUsageCache.Delete(cacheKey)
	})

	for _, tt := range []struct {
		name   string
		config settings.SourceConfig
	}{
		{name: "legacy"},
		{name: "POSIX", config: settings.SourceConfig{CopyPathPrefix: "/home/rt"}},
		{name: "Windows", config: settings.SourceConfig{CopyPathWindows: true}},
		{name: "combined", config: settings.SourceConfig{CopyPathPrefix: "/D/work", CopyPathWindows: true}},
	} {
		t.Run(tt.name, func(t *testing.T) {
			idx.Config = tt.config
			info, err := GetIndexInfo(sourceName, false)
			if err != nil {
				t.Fatal(err)
			}
			if info.CopyPathPrefix != tt.config.CopyPathPrefix || info.CopyPathWindows != tt.config.CopyPathWindows {
				t.Fatalf("clipboard mapping lost in API/SSE source info: %+v", info)
			}
			// Both the settings endpoint and SSE serialize this client-facing DTO.
			payload, err := json.Marshal(map[string]ReducedIndex{sourceName: info})
			if err != nil {
				t.Fatal(err)
			}
			if strings.Contains(string(payload), hostPath) || strings.Contains(string(payload), `"path"`) {
				t.Fatalf("host path leaked in source info: %s", payload)
			}
			if tt.config.CopyPathPrefix == "" && strings.Contains(string(payload), `"copyPathPrefix"`) {
				t.Fatalf("unset prefix must be omitted: %s", payload)
			}
			if !tt.config.CopyPathWindows && strings.Contains(string(payload), `"copyPathWindows"`) {
				t.Fatalf("disabled Windows mapping must be omitted: %s", payload)
			}
		})
	}
}
