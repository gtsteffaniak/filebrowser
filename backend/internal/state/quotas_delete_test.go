package state

import (
	"reflect"
	"testing"

	"github.com/gtsteffaniak/filebrowser/backend/internal/database/quota"
	"github.com/gtsteffaniak/filebrowser/backend/internal/database/users"
	"github.com/gtsteffaniak/filebrowser/backend/pkg/indexing"
)

func TestPrepareAccountedDelete(t *testing.T) {
	oldFolders, oldSources, oldCounters := folderQuotasByID, folderQuotasBySource, quotaCounters
	folderQuotasByID = make(map[string]*quota.FolderQuota)
	folderQuotasBySource = make(map[string][]string)
	quotaCounters = make(map[string]*quotaCounterMem)
	t.Cleanup(func() {
		folderQuotasByID, folderQuotasBySource, quotaCounters = oldFolders, oldSources, oldCounters
		indexing.ClearTestIndices()
	})
	indexing.SetTestIndex("delete", "/source")
	idx := indexing.GetIndex("delete")
	idx.SetFolderSize("/public/", 60)
	idx.SetFolderSize("/public/nested/", 50)
	idx.SetFolderSize("/public/nested/deeper/", 30)
	principal := &users.User{ID: 1, BackendScopes: []users.BackendScope{{Path: "/source", Scope: "/", Quota: &users.ScopeQuota{ID: "scope", LimitBytes: 1000, Meter: quota.MeterAccounted}}}}
	for _, q := range []quota.FolderQuota{
		{ID: "cover", Source: "/source", Path: "/", Meter: quota.MeterAccounted},
		{ID: "exact", Source: "/source", Path: "/public", Meter: quota.MeterAccounted},
		{ID: "nested", Source: "/source", Path: "/public/nested", Meter: quota.MeterAccounted, UserID: 1},
		{ID: "deeper", Source: "/source", Path: "/public/nested/deeper", Meter: quota.MeterAccounted},
		{ID: "missing", Source: "/source", Path: "/public/missing", Meter: quota.MeterAccounted},
		{ID: "sibling", Source: "/source", Path: "/sibling", Meter: quota.MeterAccounted},
		{ID: "prefix", Source: "/source", Path: "/public-other", Meter: quota.MeterAccounted},
		{ID: "other-user", Source: "/source", Path: "/public/nested", Meter: quota.MeterAccounted, UserID: 2},
		{ID: "other-source", Source: "/other", Path: "/public/nested", Meter: quota.MeterAccounted},
		{ID: "index", Source: "/source", Path: "/public/nested", Meter: quota.MeterIndexSize},
	} {
		q.LimitBytes = 1000
		storeFolderQuota(q)
		quotaCounters[q.ID] = &quotaCounterMem{UsedBytes: 100, ReservedBytes: 7}
	}
	shareID := quota.ShareQuotaID("share")
	quotaCounters["scope"] = &quotaCounterMem{UsedBytes: 100, ReservedBytes: 7}
	quotaCounters[shareID] = &quotaCounterMem{UsedBytes: 100, ReservedBytes: 7}
	want := map[string]int64{"cover": -60, "exact": -60, "nested": -50, "deeper": -30, "scope": -60, shareID: -60}
	got := PrepareAccountedDelete(principal, "delete", "/source", "/public", "share", true, 60)
	if !reflect.DeepEqual(got, want) {
		t.Fatalf("adjustments = %v, want %v", got, want)
	}
	for id, delta := range want {
		quotaCounters[id].UsedBytes = -delta
	}
	quotaCounters["cover"].UsedBytes += 40
	if err := ApplyAccountedDelete(got); err != nil {
		t.Fatal(err)
	}
	for id, c := range quotaCounters {
		expected := int64(100)
		if _, ok := want[id]; ok {
			expected = 0
		}
		if id == "cover" {
			expected = 40
		}
		if c.UsedBytes != expected || c.ReservedBytes != 7 {
			t.Errorf("%s: used=%d reserved=%d, want %d/7", id, c.UsedBytes, c.ReservedBytes, expected)
		}
	}
	for _, size := range []int64{0, -1} {
		got = PrepareAccountedDelete(principal, "delete", "/source", "/public", "", true, size)
		if !reflect.DeepEqual(got, map[string]int64{"nested": -50, "deeper": -30}) {
			t.Fatalf("unavailable/zero ancestor: %v", got)
		}
	}
	got = PrepareAccountedDelete(principal, "delete", "/source", "/public/nested/file", "", false, 5)
	if !reflect.DeepEqual(got, map[string]int64{"cover": -5, "exact": -5, "nested": -5, "scope": -5}) {
		t.Fatalf("file adjustments: %v", got)
	}
	if got := PrepareAccountedDelete(nil, "delete", "/source", "/public", "share", true, 60); len(got) != 0 {
		t.Fatalf("nil principal: %v", got)
	}
}
