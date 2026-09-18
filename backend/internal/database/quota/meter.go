package quota

import (
	"fmt"
	"strings"

	"github.com/gtsteffaniak/filebrowser/backend/pkg/indexing"
)

// MeasurementStatusReady means enforcement uses the configured meter.
const MeasurementStatusReady = "ready"

// MeasurementStatusAccountedFallback means index meter configured but accounted counter used.
const MeasurementStatusAccountedFallback = "accounted_fallback"

// IsIndexMeter reports whether meter counts usage from the search index.
func IsIndexMeter(meter string) bool {
	return meter == MeterIndexSize || meter == MeterIndexScope
}

// IndexingEnabled reports whether folder sizes are available from the index on this source.
func IndexingEnabled(sourceName string) bool {
	idx := indexing.GetIndex(sourceName)
	if idx == nil {
		return false
	}
	return !idx.Config.ResolvedRules.IndexingDisabled
}

// IndexSizeAvailable reports whether GetFolderSize succeeds for the quota root path.
func IndexSizeAvailable(sourceName, indexPath string) bool {
	idx := indexing.GetIndex(sourceName)
	if idx == nil || idx.Config.ResolvedRules.IndexingDisabled {
		return false
	}
	path := normalizeMeterPath(indexPath)
	_, ok := idx.GetFolderSizeForIndexPath(path)
	return ok
}

// EffectiveMeter resolves runtime meter: accounted when indexing off, configured index when size known, else accounted fallback.
func EffectiveMeter(configuredMeter, sourceName, quotaRootPath string) string {
	if configuredMeter == MeterAccounted || !IsIndexMeter(configuredMeter) {
		return MeterAccounted
	}
	if !IndexingEnabled(sourceName) {
		return MeterAccounted
	}
	if IndexSizeAvailable(sourceName, quotaRootPath) {
		return configuredMeter
	}
	return MeterAccounted
}

// MeasurementStatus compares configured vs effective meter for API snapshots.
func MeasurementStatus(configuredMeter, effectiveMeter string) string {
	if IsIndexMeter(configuredMeter) && effectiveMeter == MeterAccounted {
		return MeasurementStatusAccountedFallback
	}
	return MeasurementStatusReady
}

// ValidateConfiguredMeter rejects index meters when indexing is disabled on the source.
func ValidateConfiguredMeter(sourceName, meter string) error {
	meter = strings.TrimSpace(meter)
	if meter == "" {
		return nil
	}
	if meter == MeterAccounted {
		return nil
	}
	if IsIndexMeter(meter) {
		if !IndexingEnabled(sourceName) {
			return fmt.Errorf("index meter not allowed when indexing is disabled on source %q", sourceName)
		}
		return nil
	}
	return fmt.Errorf("invalid quota meter %q", meter)
}

func normalizeMeterPath(path string) string {
	path = strings.TrimSpace(path)
	path = strings.TrimSuffix(path, "/")
	if path == "" {
		return "/"
	}
	if !strings.HasPrefix(path, "/") {
		path = "/" + path
	}
	return path
}
