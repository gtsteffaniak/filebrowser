package quota

// Meter names for scope and folder quotas.
const (
	MeterIndexScope = "index_scope"
	MeterIndexSize  = "index_size"
	MeterAccounted  = "accounted"
)

// FolderQuota is a path-bound storage cap with configurable meter.
type FolderQuota struct {
	ID         string `json:"id"`
	Source     string `json:"source"` // backend source path
	Path       string `json:"path"`   // index path within source
	UserID     uint64 `json:"userId,omitempty"` // 0 = all users
	LimitBytes int64  `json:"limitBytes"`
	Meter      string `json:"meter,omitempty"` // index_size | index_scope (indexed size) | accounted (tracked usage)
}

// Counter holds accounted usage for a quota_id.
type Counter struct {
	QuotaID       string `json:"quotaId"`
	UsedBytes     int64  `json:"usedBytes"`
	ReservedBytes int64  `json:"reservedBytes"`
}

// Snapshot is a read model for API and admission checks.
type Snapshot struct {
	QuotaID           string `json:"quotaId"`
	Kind              string `json:"quotaKind"` // folder, scope, share
	LimitBytes        int64  `json:"limitBytes"`
	UsedBytes         int64  `json:"usedBytes"`
	ReservedBytes     int64  `json:"reservedBytes"`
	Meter             string `json:"meter,omitempty"` // effective meter (backward compatible)
	ConfiguredMeter   string `json:"configuredMeter,omitempty"`
	EffectiveMeter    string `json:"effectiveMeter,omitempty"`
	MeasurementStatus string `json:"measurementStatus,omitempty"` // ready, accounted_fallback
}

// ShareQuotaID returns the counter key for a share quota.
func ShareQuotaID(hash string) string {
	return "share:" + hash
}
