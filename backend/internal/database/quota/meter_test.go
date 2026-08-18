package quota_test

import (
	"testing"

	"github.com/gtsteffaniak/filebrowser/backend/internal/database/quota"
	"github.com/gtsteffaniak/filebrowser/backend/pkg/indexing"
	"github.com/gtsteffaniak/filebrowser/backend/pkg/settings"
)

func setupMeterTestSource(t *testing.T, name, path string, indexingDisabled bool) {
	t.Helper()
	prevSourceMap := settings.Config.Server.SourceMap
	prevNameToSource := settings.Config.Server.NameToSource
	t.Cleanup(func() {
		indexing.ClearTestIndices()
		settings.Config.Server.SourceMap = prevSourceMap
		settings.Config.Server.NameToSource = prevNameToSource
	})
	indexing.SetTestIndex(name, path)
	idx := indexing.GetIndex(name)
	if idx == nil {
		t.Fatal("expected test index")
	}
	idx.Config.ResolvedRules.IndexingDisabled = indexingDisabled
	settings.Config.Server.SourceMap = map[string]*settings.Source{
		path: {Path: path, Name: name},
	}
	settings.Config.Server.NameToSource = map[string]*settings.Source{
		name: settings.Config.Server.SourceMap[path],
	}
}

func TestValidateConfiguredMeter_IndexingDisabled(t *testing.T) {
	root := t.TempDir()
	name := "meter-src"
	setupMeterTestSource(t, name, root, true)

	if err := quota.ValidateConfiguredMeter(name, quota.MeterIndexSize); err == nil {
		t.Fatal("expected error for index_size when indexing disabled")
	}
	if err := quota.ValidateConfiguredMeter(name, quota.MeterIndexScope); err == nil {
		t.Fatal("expected error for index_scope when indexing disabled")
	}
	if err := quota.ValidateConfiguredMeter(name, quota.MeterAccounted); err != nil {
		t.Fatalf("accounted should be allowed: %v", err)
	}
}

func TestEffectiveMeter_IndexingDisabledForcesAccounted(t *testing.T) {
	root := t.TempDir()
	name := "meter-src2"
	setupMeterTestSource(t, name, root, true)

	got := quota.EffectiveMeter(quota.MeterIndexSize, name, "/")
	if got != quota.MeterAccounted {
		t.Fatalf("expected accounted, got %q", got)
	}
}

func TestEffectiveMeter_ExplicitAccountedWhenIndexOn(t *testing.T) {
	root := t.TempDir()
	name := "meter-src3"
	setupMeterTestSource(t, name, root, false)
	idx := indexing.GetIndex(name)
	idx.SetFolderSize("/", 1024)

	got := quota.EffectiveMeter(quota.MeterAccounted, name, "/")
	if got != quota.MeterAccounted {
		t.Fatalf("expected accounted, got %q", got)
	}
}

func TestEffectiveMeter_IndexWhenSizeAvailable(t *testing.T) {
	root := t.TempDir()
	name := "meter-src4"
	setupMeterTestSource(t, name, root, false)
	idx := indexing.GetIndex(name)
	idx.SetFolderSize("/", 2048)

	got := quota.EffectiveMeter(quota.MeterIndexSize, name, "/")
	if got != quota.MeterIndexSize {
		t.Fatalf("expected index_size, got %q", got)
	}
	status := quota.MeasurementStatus(quota.MeterIndexSize, got)
	if status != quota.MeasurementStatusReady {
		t.Fatalf("expected ready, got %q", status)
	}
}

func TestEffectiveMeter_IndexPathTrailingSlash(t *testing.T) {
	root := t.TempDir()
	name := "meter-src6"
	setupMeterTestSource(t, name, root, false)
	idx := indexing.GetIndex(name)
	idx.SetFolderSize("/projects/", 4096)

	got := quota.EffectiveMeter(quota.MeterIndexSize, name, "/projects")
	if got != quota.MeterIndexSize {
		t.Fatalf("expected index_size for path without trailing slash, got %q", got)
	}
}
func TestEffectiveMeter_RuntimeFallback(t *testing.T) {
	root := t.TempDir()
	name := "meter-src5"
	setupMeterTestSource(t, name, root, false)

	got := quota.EffectiveMeter(quota.MeterIndexScope, name, "/missing")
	if got != quota.MeterAccounted {
		t.Fatalf("expected accounted fallback, got %q", got)
	}
	status := quota.MeasurementStatus(quota.MeterIndexScope, got)
	if status != quota.MeasurementStatusAccountedFallback {
		t.Fatalf("expected accounted_fallback, got %q", status)
	}
}
