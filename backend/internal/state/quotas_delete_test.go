package state

import (
	"reflect"
	"slices"
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
	wantDeltas := map[string]int64{"cover": -60, "scope": -60, shareID: -60}
	wantResets := []string{"exact", "nested", "deeper", "missing"}
	got := PrepareAccountedDelete(principal, "delete", "/source", "/public", "share", true, 60)
	if !reflect.DeepEqual(got.Deltas, wantDeltas) {
		t.Fatalf("deltas = %v, want %v", got.Deltas, wantDeltas)
	}
	if !slicesEqual(got.Resets, wantResets) {
		t.Fatalf("resets = %v, want %v", got.Resets, wantResets)
	}
	for id, delta := range wantDeltas {
		quotaCounters[id].UsedBytes = -delta
	}
	quotaCounters["cover"].UsedBytes += 40
	for _, id := range wantResets {
		quotaCounters[id].UsedBytes = 100
	}
	if err := ApplyAccountedDelete(got); err != nil {
		t.Fatal(err)
	}
	for id, c := range quotaCounters {
		expected := int64(100)
		if _, ok := wantDeltas[id]; ok {
			expected = 0
		}
		if slices.Contains(wantResets, id) {
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
		wantResetsUnknown := []string{"exact", "nested", "deeper", "missing"}
		if len(got.Deltas) != 0 || !slicesEqual(got.Resets, wantResetsUnknown) {
			t.Fatalf("unavailable/zero ancestor: deltas=%v resets=%v", got.Deltas, got.Resets)
		}
	}
	got = PrepareAccountedDelete(principal, "delete", "/source", "/public/nested/file", "", false, 5)
	if !reflect.DeepEqual(got, DeleteAdjustments{
		Deltas: map[string]int64{"cover": -5, "exact": -5, "nested": -5, "scope": -5},
	}) {
		t.Fatalf("file adjustments: %v", got)
	}
	empty := PrepareAccountedDelete(nil, "delete", "/source", "/public", "share", true, 60)
	if len(empty.Deltas) != 0 || len(empty.Resets) != 0 {
		t.Fatalf("nil principal: %v", empty)
	}
}

func TestPrepareAccountedDeleteResetsWithoutIndexedSizes(t *testing.T) {
	oldFolders, oldSources, oldCounters := folderQuotasByID, folderQuotasBySource, quotaCounters
	folderQuotasByID = make(map[string]*quota.FolderQuota)
	folderQuotasBySource = make(map[string][]string)
	quotaCounters = make(map[string]*quotaCounterMem)
	t.Cleanup(func() {
		folderQuotasByID, folderQuotasBySource, quotaCounters = oldFolders, oldSources, oldCounters
		indexing.ClearTestIndices()
	})
	indexing.SetTestIndex("delete", "/source")
	principal := &users.User{ID: 1}
	nested := quota.FolderQuota{ID: "nested", Source: "/source", Path: "/public/nested", Meter: quota.MeterAccounted, LimitBytes: 1000}
	storeFolderQuota(nested)
	quotaCounters["nested"] = &quotaCounterMem{UsedBytes: 50, ReservedBytes: 3}
	got := PrepareAccountedDelete(principal, "delete", "/source", "/public", "", true, 0)
	if !slicesEqual(got.Resets, []string{"nested"}) {
		t.Fatalf("resets = %v, want nested reset without index sizes", got.Resets)
	}
	if err := ApplyAccountedDelete(got); err != nil {
		t.Fatal(err)
	}
	if used, reserved := GetQuotaCounterSnapshot("nested"); used != 0 || reserved != 3 {
		t.Fatalf("nested used=%d reserved=%d, want 0/3", used, reserved)
	}
}

func slicesEqual(a, b []string) bool {
	if len(a) != len(b) {
		return false
	}
	ac := slices.Clone(a)
	bc := slices.Clone(b)
	slices.Sort(ac)
	slices.Sort(bc)
	return slices.Equal(ac, bc)
}
