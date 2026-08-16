package state

import (
	"fmt"
	"strings"
	"sync"
	"time"

	"github.com/google/uuid"
	"github.com/gtsteffaniak/filebrowser/backend/internal/database/quota"
	"github.com/gtsteffaniak/filebrowser/backend/internal/database/users"
	"github.com/gtsteffaniak/filebrowser/backend/pkg/indexing"
	"github.com/gtsteffaniak/filebrowser/backend/pkg/settings"
	"github.com/gtsteffaniak/go-logger/logger"
)

var (
	quotasMux            sync.RWMutex
	folderQuotasByID     map[string]*quota.FolderQuota
	folderQuotasBySource map[string][]string // source path -> quota ids
	quotaCounters        map[string]*quotaCounterMem
	reservationsBySession map[string][]quotaReservation
	pendingIndexDelta    map[string]int64 // quota_id -> bytes not yet in index
	quotaFlusher         *quotaCounterFlusher
)

type quotaCounterMem struct {
	UsedBytes     int64
	ReservedBytes int64
	Dirty         bool
}

type quotaReservation struct {
	QuotaID string
	Bytes   int64
	Meter   string
	Kind    string
}

func initQuotaMaps() {
	folderQuotasByID = make(map[string]*quota.FolderQuota)
	folderQuotasBySource = make(map[string][]string)
	quotaCounters = make(map[string]*quotaCounterMem)
	reservationsBySession = make(map[string][]quotaReservation)
	pendingIndexDelta = make(map[string]int64)
}

// InitQuotas loads quota rows and starts the counter flusher.
func InitQuotas(cfg settings.Database) error {
	initQuotaMaps()
	if sqlDb == nil {
		return nil
	}

	rows, err := sqlDb.GetAllFolderQuotas()
	if err != nil {
		return fmt.Errorf("load folder quotas: %w", err)
	}
	for _, q := range rows {
		storeFolderQuota(q)
	}

	startQuotaFlusher(cfg.Quotas)

	counters, err := sqlDb.GetAllQuotaCounters()
	if err != nil {
		return fmt.Errorf("load quota counters: %w", err)
	}
	for _, c := range counters {
		quotaCounters[c.QuotaID] = &quotaCounterMem{
			UsedBytes:     c.UsedBytes,
			ReservedBytes: 0,
		}
		if c.ReservedBytes > 0 {
			quotaCounters[c.QuotaID].Dirty = true
			if quotaFlusher != nil {
				quotaFlusher.markDirty(c.QuotaID)
			}
		}
	}

	return nil
}

func storeFolderQuota(q quota.FolderQuota) {
	copyQ := q
	folderQuotasByID[q.ID] = &copyQ
	ids := folderQuotasBySource[q.Source]
	if !containsString(ids, q.ID) {
		folderQuotasBySource[q.Source] = append(ids, q.ID)
	}
}

func containsString(ss []string, s string) bool {
	for _, v := range ss {
		if v == s {
			return true
		}
	}
	return false
}

// StopQuotaFlusher flushes dirty counters and stops the background loop.
func StopQuotaFlusher() {
	if quotaFlusher != nil {
		quotaFlusher.Stop()
	}
}

// GetFolderQuotaByPath returns a folder quota for exact source+path (first match).
// GetFolderQuotaByID returns a copy of a folder quota by id.
func GetFolderQuotaByID(id string) (*quota.FolderQuota, error) {
	quotasMux.RLock()
	defer quotasMux.RUnlock()
	q, ok := folderQuotasByID[id]
	if !ok {
		return nil, fmt.Errorf("quota not found")
	}
	copyQ := *q
	return &copyQ, nil
}

func GetFolderQuotaByPath(source, path string) (*quota.FolderQuota, error) {
	return GetFolderQuotaByPathAndUser(source, path, 0)
}

// GetFolderQuotaByPathAndUser returns a folder quota for exact source, path, and user binding (0 = all users).
func GetFolderQuotaByPathAndUser(source, path string, userID uint64) (*quota.FolderQuota, error) {
	path = normalizeQuotaPath(path)
	quotasMux.RLock()
	defer quotasMux.RUnlock()
	for _, id := range folderQuotasBySource[source] {
		q := folderQuotasByID[id]
		if q != nil && normalizeQuotaPath(q.Path) == path && q.UserID == userID {
			copyQ := *q
			return &copyQ, nil
		}
	}
	return nil, fmt.Errorf("quota not found")
}

// ListFolderQuotasForSource returns folder quotas for a source.
func ListFolderQuotasForSource(source string) []quota.FolderQuota {
	quotasMux.RLock()
	defer quotasMux.RUnlock()
	ids := folderQuotasBySource[source]
	out := make([]quota.FolderQuota, 0, len(ids))
	for _, id := range ids {
		if q := folderQuotasByID[id]; q != nil {
			out = append(out, *q)
		}
	}
	return out
}

// CreateFolderQuota persists and caches a folder quota.
func CreateFolderQuota(source, path string, userID uint64, limitBytes int64, meter string) (*quota.FolderQuota, error) {
	path = normalizeQuotaPath(path)
	if limitBytes <= 0 {
		return nil, fmt.Errorf("limit must be positive")
	}
	meter = strings.TrimSpace(meter)
	if meter == "" {
		meter = quota.MeterIndexSize
	}
	sourceName := sourceNameForPath(source)
	if sourceName != "" {
		if err := quota.ValidateConfiguredMeter(sourceName, meter); err != nil {
			return nil, err
		}
	}
	q := quota.FolderQuota{
		ID:         uuid.New().String(),
		Source:     source,
		Path:       path,
		UserID:     userID,
		LimitBytes: limitBytes,
		Meter:      meter,
	}

	quotasMux.Lock()
	defer quotasMux.Unlock()

	if err := sqlDb.SaveFolderQuota(&q); err != nil {
		return nil, err
	}
	storeFolderQuota(q)
	if err := ensureQuotaCounterMem(q.ID); err != nil {
		return nil, err
	}
	copyQ := q
	return &copyQ, nil
}

// UpdateFolderQuota updates limit and optional meter; userID is applied only when userIDPatch is non-nil.
func UpdateFolderQuota(id string, limitBytes int64, meter string, userIDPatch *uint64) (*quota.FolderQuota, error) {
	quotasMux.Lock()
	defer quotasMux.Unlock()

	q, ok := folderQuotasByID[id]
	if !ok {
		return nil, fmt.Errorf("quota not found")
	}
	if limitBytes > 0 {
		q.LimitBytes = limitBytes
	}
	if userIDPatch != nil {
		q.UserID = *userIDPatch
	}
	meter = strings.TrimSpace(meter)
	if meter != "" {
		sourceName := sourceNameForPath(q.Source)
		if sourceName != "" {
			if err := quota.ValidateConfiguredMeter(sourceName, meter); err != nil {
				return nil, err
			}
		}
		q.Meter = meter
	}
	if err := sqlDb.SaveFolderQuota(q); err != nil {
		return nil, err
	}
	copyQ := *q
	return &copyQ, nil
}

func sourceNameForPath(sourcePath string) string {
	if src, ok := settings.Config.Server.SourceMap[sourcePath]; ok {
		return src.Name
	}
	return ""
}

// DeleteFolderQuota removes a folder quota and its counter.
func DeleteFolderQuota(id string) error {
	quotasMux.Lock()
	defer quotasMux.Unlock()

	q, ok := folderQuotasByID[id]
	if !ok {
		return fmt.Errorf("quota not found")
	}
	if err := sqlDb.DeleteFolderQuota(id); err != nil {
		return err
	}
	delete(folderQuotasByID, id)
	ids := folderQuotasBySource[q.Source]
	for i, v := range ids {
		if v == id {
			folderQuotasBySource[q.Source] = append(ids[:i], ids[i+1:]...)
			break
		}
	}
	delete(quotaCounters, id)
	delete(pendingIndexDelta, id)
	return nil
}

func ensureQuotaCounterMem(quotaID string) error {
	if _, ok := quotaCounters[quotaID]; ok {
		return nil
	}
	if err := sqlDb.EnsureQuotaCounter(quotaID); err != nil {
		return err
	}
	quotaCounters[quotaID] = &quotaCounterMem{}
	return nil
}

// EnsureShareQuotaCounter creates counter for share quota.
func EnsureShareQuotaCounter(hash string) error {
	quotasMux.Lock()
	defer quotasMux.Unlock()
	return ensureQuotaCounterMem(quota.ShareQuotaID(hash))
}

// DeleteShareQuotaCounter removes share quota counter.
func DeleteShareQuotaCounter(hash string) error {
	quotasMux.Lock()
	defer quotasMux.Unlock()
	id := quota.ShareQuotaID(hash)
	delete(quotaCounters, id)
	delete(pendingIndexDelta, id)
	return sqlDb.DeleteQuotaCounter(id)
}

// RebuildShareQuotaUsage sets share counter used bytes from indexed size at indexPath.
func RebuildShareQuotaUsage(hash, sourceName, indexPath string) error {
	size, ok := IndexedPathBytes(sourceName, indexPath, true)
	if !ok {
		size = 0
	}
	id := quota.ShareQuotaID(hash)
	quotasMux.Lock()
	defer quotasMux.Unlock()
	if err := ensureQuotaCounterMemLocked(id); err != nil {
		return err
	}
	c := quotaCounters[id]
	c.UsedBytes = size
	c.ReservedBytes = 0
	c.Dirty = true
	if quotaFlusher != nil {
		quotaFlusher.markDirty(id)
	}
	signalQuotaFlush()
	return nil
}

// GetQuotaCounterSnapshot returns used/reserved for a quota_id.
func GetQuotaCounterSnapshot(quotaID string) (used, reserved int64) {
	quotasMux.RLock()
	defer quotasMux.RUnlock()
	if c := quotaCounters[quotaID]; c != nil {
		return c.UsedBytes, c.ReservedBytes
	}
	return 0, 0
}

// GetShareQuotaUsage returns used and reserved bytes for a share.
func GetShareQuotaUsage(hash string) (used, reserved int64) {
	return GetQuotaCounterSnapshot(quota.ShareQuotaID(hash))
}

// FolderQuotaSnapshot builds a snapshot for a folder quota.
func FolderQuotaSnapshot(q quota.FolderQuota, sourceName string) quota.Snapshot {
	configured := configuredFolderMeter(q)
	effective := quota.EffectiveMeter(configured, sourceName, q.Path)
	used, reserved := folderUsageForMeter(effective, sourceName, q.Path, q.ID)
	status := quota.MeasurementStatus(configured, effective)
	logger.Debugf("quota snapshot: id=%s source=%s indexPath=%q configured=%s effective=%s used=%d reserved=%d status=%s",
		q.ID, sourceName, q.Path, configured, effective, used, reserved, status)
	return quota.Snapshot{
		QuotaID:           q.ID,
		Kind:              "folder",
		LimitBytes:        q.LimitBytes,
		UsedBytes:         used,
		ReservedBytes:     reserved,
		Meter:             effective,
		ConfiguredMeter:   configured,
		EffectiveMeter:    effective,
		MeasurementStatus: status,
	}
}

// FolderQuotaUsagePreview returns usage for a path without a persisted quota row.
func FolderQuotaUsagePreview(sourceName, indexPath string) quota.Snapshot {
	configured := quota.MeterIndexSize
	effective := quota.EffectiveMeter(configured, sourceName, indexPath)
	used, reserved := folderUsageForMeter(effective, sourceName, indexPath, "")
	status := quota.MeasurementStatus(configured, effective)
	logger.Debugf("quota usage preview: source=%s indexPath=%q effective=%s used=%d status=%s",
		sourceName, indexPath, effective, used, status)
	return quota.Snapshot{
		UsedBytes:         used,
		ReservedBytes:     reserved,
		Meter:             effective,
		ConfiguredMeter:   configured,
		EffectiveMeter:    effective,
		MeasurementStatus: status,
	}
}

func configuredFolderMeter(q quota.FolderQuota) string {
	if q.Meter == "" {
		return quota.MeterIndexSize
	}
	return q.Meter
}

func folderUsageForMeter(meter, sourceName, indexPath, quotaID string) (used, reserved int64) {
	if meter == quota.MeterAccounted {
		return GetQuotaCounterSnapshot(quotaID)
	}
	used, _ = indexFolderUsage(sourceName, indexPath)
	used += getPendingIndexDelta(quotaID)
	return used, getCounterReserved(quotaID)
}

// ScopeQuotaSnapshot builds a snapshot for a user scope quota.
func ScopeQuotaSnapshot(user *users.User, sourceName string, sq *users.ScopeQuota) quota.Snapshot {
	if sq == nil || sq.LimitBytes <= 0 {
		return quota.Snapshot{}
	}
	configured := sq.Meter
	if configured == "" {
		configured = quota.MeterIndexScope
	}
	scopePath := scopeIndexPath(user, sourceName)
	effective := quota.EffectiveMeter(configured, sourceName, scopePath)
	used, reserved := scopeUsageForMeter(effective, user, sourceName, scopePath, sq.ID)
	return quota.Snapshot{
		QuotaID:           sq.ID,
		Kind:              "scope",
		LimitBytes:        sq.LimitBytes,
		UsedBytes:         used,
		ReservedBytes:     reserved,
		Meter:             effective,
		ConfiguredMeter:   configured,
		EffectiveMeter:    effective,
		MeasurementStatus: quota.MeasurementStatus(configured, effective),
	}
}

func scopeUsageForMeter(meter string, user *users.User, sourceName, scopePath, quotaID string) (used, reserved int64) {
	if meter == quota.MeterAccounted {
		return GetQuotaCounterSnapshot(quotaID)
	}
	used, _ = indexFolderUsage(sourceName, scopePath)
	used += getPendingIndexDelta(quotaID)
	return used, getCounterReserved(quotaID)
}

func scopeIndexPath(user *users.User, sourceName string) string {
	sourceInfo, ok := settings.Config.Server.NameToSource[sourceName]
	if !ok {
		return "/"
	}
	scopePath, err := user.GetScopeForSourcePath(sourceInfo.Path)
	if err != nil {
		return "/"
	}
	return scopePath
}

func getCounterReserved(quotaID string) int64 {
	quotasMux.RLock()
	defer quotasMux.RUnlock()
	if c := quotaCounters[quotaID]; c != nil {
		return c.ReservedBytes
	}
	return 0
}

func getPendingIndexDelta(quotaID string) int64 {
	quotasMux.RLock()
	defer quotasMux.RUnlock()
	return pendingIndexDelta[quotaID]
}

func indexFolderUsage(sourceName, indexPath string) (int64, int64) {
	idx := indexing.GetIndex(sourceName)
	if idx == nil {
		logger.Debugf("quota usage: index missing for source=%s indexPath=%q", sourceName, indexPath)
		return 0, 0
	}
	key := folderSizeLookupKeyForQuota(indexPath)
	size, ok := idx.GetFolderSizeForIndexPath(indexPath)
	if !ok {
		logger.Debugf("quota usage: folder size unavailable source=%s indexPath=%q lookupKey=%q", sourceName, indexPath, key)
		return 0, 0
	}
	logger.Debugf("quota usage: source=%s indexPath=%q lookupKey=%q usedBytes=%d", sourceName, indexPath, key, size)
	return int64(size), 0
}

func folderSizeLookupKeyForQuota(path string) string {
	path = normalizeQuotaPath(path)
	if path == "/" {
		return "/"
	}
	return path + "/"
}

func normalizeQuotaPath(path string) string {
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

func indexPathCovers(root, target string) bool {
	root = normalizeQuotaPath(root)
	target = normalizeQuotaPath(target)
	if root == "/" {
		return true
	}
	return target == root || strings.HasPrefix(target, root+"/")
}

// ApplicableFolderQuotas returns folder quotas covering destIndexPath for principal.
func ApplicableFolderQuotas(sourcePath, destIndexPath string, principalUserID uint64) []quota.FolderQuota {
	destIndexPath = normalizeQuotaPath(destIndexPath)
	quotasMux.RLock()
	defer quotasMux.RUnlock()
	var out []quota.FolderQuota
	for _, id := range folderQuotasBySource[sourcePath] {
		q := folderQuotasByID[id]
		if q == nil || q.LimitBytes <= 0 {
			continue
		}
		if q.UserID != 0 && q.UserID != principalUserID {
			continue
		}
		if !indexPathCovers(q.Path, destIndexPath) {
			continue
		}
		out = append(out, *q)
	}
	return out
}

// ReserveQuota increases reserved bytes for applicable quotas.
func ReserveQuota(sessionID string, checks []QuotaReserveCheck) error {
	if sessionID == "" || len(checks) == 0 {
		return nil
	}
	quotasMux.Lock()
	defer quotasMux.Unlock()

	for _, chk := range checks {
		if chk.LimitBytes <= 0 || chk.DeltaBytes <= 0 {
			continue
		}
		reserved := sumSessionReservedLocked(chk.QuotaID)
		if chk.UsedBytes+reserved+chk.DeltaBytes > chk.LimitBytes {
			return fmt.Errorf("quota exceeded for %s", chk.QuotaID)
		}
		if chk.Meter == quota.MeterAccounted || chk.Kind == "share" {
			if err := ensureQuotaCounterMemLocked(chk.QuotaID); err != nil {
				return err
			}
			quotaCounters[chk.QuotaID].ReservedBytes += chk.DeltaBytes
		}
		reservationsBySession[sessionID] = append(reservationsBySession[sessionID], quotaReservation{
			QuotaID: chk.QuotaID,
			Bytes:   chk.DeltaBytes,
			Meter:   chk.Meter,
			Kind:    chk.Kind,
		})
	}
	return nil
}

func sumSessionReservedLocked(quotaID string) int64 {
	var total int64
	for _, rs := range reservationsBySession {
		for _, r := range rs {
			if r.QuotaID == quotaID {
				total += r.Bytes
			}
		}
	}
	return total
}

// SessionReservedForQuota returns bytes reserved across active sessions for a quota id.
func SessionReservedForQuota(quotaID string) int64 {
	quotasMux.RLock()
	defer quotasMux.RUnlock()
	return sumSessionReservedLocked(quotaID)
}

func ensureQuotaCounterMemLocked(quotaID string) error {
	if _, ok := quotaCounters[quotaID]; ok {
		return nil
	}
	if err := sqlDb.EnsureQuotaCounter(quotaID); err != nil {
		return err
	}
	quotaCounters[quotaID] = &quotaCounterMem{}
	return nil
}

// CommitQuota applies committed byte delta and releases reservation for session.
func CommitQuota(sessionID string, deltaBytes int64) error {
	quotasMux.Lock()
	defer quotasMux.Unlock()

	rs := reservationsBySession[sessionID]
	delete(reservationsBySession, sessionID)

	for _, r := range rs {
		isAccounted := r.Meter == quota.MeterAccounted || r.Kind == "share"
		commitBytes := r.Bytes
		if commitBytes <= 0 && deltaBytes > 0 {
			commitBytes = deltaBytes
		}
		if isAccounted {
			if err := ensureQuotaCounterMemLocked(r.QuotaID); err != nil {
				return err
			}
			c := quotaCounters[r.QuotaID]
			if c.ReservedBytes >= r.Bytes {
				c.ReservedBytes -= r.Bytes
			} else {
				c.ReservedBytes = 0
			}
			c.UsedBytes += commitBytes
			c.Dirty = true
			if quotaFlusher != nil {
				quotaFlusher.markDirty(r.QuotaID)
			}
		} else {
			pendingIndexDelta[r.QuotaID] += commitBytes
		}
	}
	signalQuotaFlush()
	return nil
}

// ReleaseQuota releases reservations without committing usage.
func ReleaseQuota(sessionID string) {
	quotasMux.Lock()
	defer quotasMux.Unlock()
	releaseSessionReservationsLocked(sessionID)
}

func releaseSessionReservationsLocked(sessionID string) {
	rs, ok := reservationsBySession[sessionID]
	if !ok {
		return
	}
	delete(reservationsBySession, sessionID)
	for _, r := range rs {
		if c := quotaCounters[r.QuotaID]; c != nil {
			if c.ReservedBytes >= r.Bytes {
				c.ReservedBytes -= r.Bytes
			} else {
				c.ReservedBytes = 0
			}
		}
	}
}

// ForceCommitSessionQuota commits reserved usage for a session; retries once on failure.
func ForceCommitSessionQuota(sessionID string, deltaBytes int64) error {
	if err := CommitQuota(sessionID, deltaBytes); err != nil {
		return CommitQuota(sessionID, deltaBytes)
	}
	return nil
}

// ApplyAccountedUsageDelta adjusts accounted and share quota counters for a path (negative delta frees space).
func ApplyAccountedUsageDelta(principal *users.User, sourceName, sourcePath, indexPath, shareHash string, delta int64) error {
	if delta == 0 || principal == nil {
		return nil
	}
	principalID := principal.ID
	quotasMux.Lock()
	defer quotasMux.Unlock()

	for _, fq := range applicableFolderQuotasLocked(sourcePath, indexPath, principalID) {
		configured := configuredFolderMeter(fq)
		effective := quota.EffectiveMeter(configured, sourceName, fq.Path)
		if effective != quota.MeterAccounted {
			continue
		}
		if err := applyCounterDeltaLocked(fq.ID, delta); err != nil {
			return err
		}
	}

	for _, bs := range principal.BackendScopes {
		if bs.Path != sourcePath || bs.Quota == nil || bs.Quota.LimitBytes <= 0 {
			continue
		}
		sq := bs.Quota
		configured := sq.Meter
		if configured == "" {
			configured = quota.MeterIndexScope
		}
		scopePath := scopeIndexPath(principal, sourceName)
		effective := quota.EffectiveMeter(configured, sourceName, scopePath)
		if effective != quota.MeterAccounted {
			continue
		}
		if err := applyCounterDeltaLocked(sq.ID, delta); err != nil {
			return err
		}
	}

	if shareHash != "" {
		id := quota.ShareQuotaID(shareHash)
		if err := applyCounterDeltaLocked(id, delta); err != nil {
			return err
		}
	}

	signalQuotaFlush()
	return nil
}

func applicableFolderQuotasLocked(sourcePath, destIndexPath string, principalUserID uint64) []quota.FolderQuota {
	destIndexPath = normalizeQuotaPath(destIndexPath)
	var out []quota.FolderQuota
	for _, id := range folderQuotasBySource[sourcePath] {
		q := folderQuotasByID[id]
		if q == nil || q.LimitBytes <= 0 {
			continue
		}
		if q.UserID != 0 && q.UserID != principalUserID {
			continue
		}
		if !indexPathCovers(q.Path, destIndexPath) {
			continue
		}
		out = append(out, *q)
	}
	return out
}

func applyCounterDeltaLocked(quotaID string, delta int64) error {
	if err := ensureQuotaCounterMemLocked(quotaID); err != nil {
		return err
	}
	c := quotaCounters[quotaID]
	c.UsedBytes += delta
	if c.UsedBytes < 0 {
		c.UsedBytes = 0
	}
	c.Dirty = true
	if quotaFlusher != nil {
		quotaFlusher.markDirty(quotaID)
	}
	return nil
}

type QuotaReserveCheck struct {
	QuotaID    string
	Kind       string
	Meter      string
	LimitBytes int64
	UsedBytes  int64
	DeltaBytes int64
}

// ClearPendingIndexDelta clears overlay after index refresh.
func ClearPendingIndexDelta(quotaID string, delta int64) {
	quotasMux.Lock()
	defer quotasMux.Unlock()
	if pendingIndexDelta[quotaID] <= delta {
		delete(pendingIndexDelta, quotaID)
	} else {
		pendingIndexDelta[quotaID] -= delta
	}
}

// BuildReserveChecks builds reserve checks for an upload destination.
func BuildReserveChecks(principal *users.User, sourceName, sourcePath, destIndexPath string, shareHash string, shareLimit int64, delta int64) []QuotaReserveCheck {
	principalID := principal.ID
	checks := []QuotaReserveCheck{}

	for _, fq := range ApplicableFolderQuotas(sourcePath, destIndexPath, principalID) {
		configured := configuredFolderMeter(fq)
		effective := quota.EffectiveMeter(configured, sourceName, fq.Path)
		used, _ := folderUsageForMeter(effective, sourceName, fq.Path, fq.ID)
		checks = append(checks, QuotaReserveCheck{
			QuotaID:    fq.ID,
			Kind:       "folder",
			Meter:      effective,
			LimitBytes: fq.LimitBytes,
			UsedBytes:  used,
			DeltaBytes: delta,
		})
	}

	for _, bs := range principal.BackendScopes {
		if bs.Path != sourcePath || bs.Quota == nil || bs.Quota.LimitBytes <= 0 {
			continue
		}
		sq := bs.Quota
		configured := sq.Meter
		if configured == "" {
			configured = quota.MeterIndexScope
		}
		scopePath := scopeIndexPath(principal, sourceName)
		effective := quota.EffectiveMeter(configured, sourceName, scopePath)
		used, _ := scopeUsageForMeter(effective, principal, sourceName, scopePath, sq.ID)
		checks = append(checks, QuotaReserveCheck{
			QuotaID:    sq.ID,
			Kind:       "scope",
			Meter:      effective,
			LimitBytes: sq.LimitBytes,
			UsedBytes:  used,
			DeltaBytes: delta,
		})
	}

	if shareHash != "" && shareLimit > 0 {
		id := quota.ShareQuotaID(shareHash)
		used, _ := GetQuotaCounterSnapshot(id)
		checks = append(checks, QuotaReserveCheck{
			QuotaID:    id,
			Kind:       "share",
			Meter:      quota.MeterAccounted,
			LimitBytes: shareLimit,
			UsedBytes:  used,
			DeltaBytes: delta,
		})
	}
	return checks
}

// IndexedPathBytes returns indexed byte size for a file or directory path.
func IndexedPathBytes(sourceName, indexPath string, isDir bool) (int64, bool) {
	idx := indexing.GetIndex(sourceName)
	if idx == nil {
		return 0, false
	}
	normalized := normalizeQuotaPath(indexPath)
	if isDir {
		size, ok := idx.GetFolderSizeForIndexPath(normalized)
		return int64(size), ok
	}
	fi, err := idx.GetFileInfo(indexing.FileInfoRequest{
		IndexPath:      normalized,
		FollowSymlinks: true,
		Expand:         false,
	})
	if err != nil {
		return 0, false
	}
	return fi.Size, true
}

func copyMoveItemDelta(action, fromIndexPath string, quotaRoot string, itemSize, overwrite int64) int64 {
	if action == "move" || action == "rename" {
		if indexPathCovers(quotaRoot, fromIndexPath) {
			return 0
		}
	}
	delta := itemSize
	if overwrite > 0 {
		delta = itemSize - overwrite
	}
	if delta < 0 {
		return 0
	}
	return delta
}

// BuildCopyMoveReserveChecks builds reserve checks for copy/move into toIndexPath.
func BuildCopyMoveReserveChecks(principal *users.User, sourceName, sourcePath, fromIndexPath, toIndexPath, action string, itemSize, overwrite int64, shareHash string, shareLimit int64) []QuotaReserveCheck {
	principalID := principal.ID
	checks := []QuotaReserveCheck{}

	for _, fq := range ApplicableFolderQuotas(sourcePath, toIndexPath, principalID) {
		delta := copyMoveItemDelta(action, fromIndexPath, fq.Path, itemSize, overwrite)
		if delta <= 0 {
			continue
		}
		configured := configuredFolderMeter(fq)
		effective := quota.EffectiveMeter(configured, sourceName, fq.Path)
		used, _ := folderUsageForMeter(effective, sourceName, fq.Path, fq.ID)
		checks = append(checks, QuotaReserveCheck{
			QuotaID:    fq.ID,
			Kind:       "folder",
			Meter:      effective,
			LimitBytes: fq.LimitBytes,
			UsedBytes:  used,
			DeltaBytes: delta,
		})
	}

	for _, bs := range principal.BackendScopes {
		if bs.Path != sourcePath || bs.Quota == nil || bs.Quota.LimitBytes <= 0 {
			continue
		}
		sq := bs.Quota
		scopePath := scopeIndexPath(principal, sourceName)
		delta := copyMoveItemDelta(action, fromIndexPath, scopePath, itemSize, overwrite)
		if delta <= 0 {
			continue
		}
		configured := sq.Meter
		if configured == "" {
			configured = quota.MeterIndexScope
		}
		effective := quota.EffectiveMeter(configured, sourceName, scopePath)
		used, _ := scopeUsageForMeter(effective, principal, sourceName, scopePath, sq.ID)
		checks = append(checks, QuotaReserveCheck{
			QuotaID:    sq.ID,
			Kind:       "scope",
			Meter:      effective,
			LimitBytes: sq.LimitBytes,
			UsedBytes:  used,
			DeltaBytes: delta,
		})
	}

	if shareHash != "" && shareLimit > 0 {
		delta := itemSize
		if overwrite > 0 {
			delta = itemSize - overwrite
		}
		if delta > 0 {
			id := quota.ShareQuotaID(shareHash)
			used, _ := GetQuotaCounterSnapshot(id)
			checks = append(checks, QuotaReserveCheck{
				QuotaID:    id,
				Kind:       "share",
				Meter:      quota.MeterAccounted,
				LimitBytes: shareLimit,
				UsedBytes:  used,
				DeltaBytes: delta,
			})
		}
	}
	return checks
}

// HasApplicableQuotas reports whether any quota applies to this upload.
func HasApplicableQuotas(principal *users.User, sourcePath, destIndexPath, shareHash string, shareLimit int64) bool {
	if shareLimit > 0 {
		return true
	}
	if len(ApplicableFolderQuotas(sourcePath, destIndexPath, principal.ID)) > 0 {
		return true
	}
	for _, bs := range principal.BackendScopes {
		if bs.Path == sourcePath && bs.Quota != nil && bs.Quota.LimitBytes > 0 {
			return true
		}
	}
	return false
}

// --- flusher (mirrors activity recorder) ---

type quotaCounterFlusher struct {
	mu            sync.Mutex
	dirtyIDs      map[string]struct{}
	flushCh       chan struct{}
	stopCh        chan struct{}
	doneCh        chan struct{}
	flushInterval time.Duration
	maxBuffers    int
	stopped       bool
}

func startQuotaFlusher(cfg settings.QuotasConfig) {
	interval := cfg.FlushIntervalSeconds
	if interval <= 0 {
		interval = 10
	}
	maxBuf := cfg.FlushMaxBuffers
	if maxBuf <= 0 {
		maxBuf = 500
	}
	quotaFlusher = &quotaCounterFlusher{
		dirtyIDs:      make(map[string]struct{}),
		flushCh:       make(chan struct{}, 1),
		stopCh:        make(chan struct{}),
		doneCh:        make(chan struct{}),
		flushInterval: time.Duration(interval) * time.Second,
		maxBuffers:    maxBuf,
	}
	go quotaFlusher.loop()
}

func signalQuotaFlush() {
	if quotaFlusher == nil {
		return
	}
	quotaFlusher.mu.Lock()
	count := len(quotaFlusher.dirtyIDs)
	quotaFlusher.mu.Unlock()
	if count >= quotaFlusher.maxBuffers {
		select {
		case quotaFlusher.flushCh <- struct{}{}:
		default:
		}
	}
}

func (f *quotaCounterFlusher) markDirty(id string) {
	f.mu.Lock()
	defer f.mu.Unlock()
	if f.stopped {
		return
	}
	f.dirtyIDs[id] = struct{}{}
}

func (f *quotaCounterFlusher) loop() {
	defer close(f.doneCh)
	ticker := time.NewTicker(f.flushInterval)
	defer ticker.Stop()
	for {
		select {
		case <-f.stopCh:
			f.flush()
			return
		case <-f.flushCh:
			f.flush()
		case <-ticker.C:
			f.flush()
		}
	}
}

func (f *quotaCounterFlusher) flush() {
	f.mu.Lock()
	if len(f.dirtyIDs) == 0 {
		f.mu.Unlock()
		return
	}
	ids := make([]string, 0, len(f.dirtyIDs))
	for id := range f.dirtyIDs {
		ids = append(ids, id)
	}
	f.dirtyIDs = make(map[string]struct{})
	f.mu.Unlock()

	quotasMux.RLock()
	batch := make([]quota.Counter, 0, len(ids))
	for _, id := range ids {
		if c := quotaCounters[id]; c != nil && c.Dirty {
			batch = append(batch, quota.Counter{
				QuotaID:       id,
				UsedBytes:     c.UsedBytes,
				ReservedBytes: c.ReservedBytes,
			})
		}
	}
	quotasMux.RUnlock()

	if len(batch) == 0 || sqlDb == nil {
		return
	}
	if err := sqlDb.BulkUpsertQuotaCounters(batch); err != nil {
		logger.Warningf("quota counter flush failed: %v", err)
		quotasMux.Lock()
		for _, c := range batch {
			if mem := quotaCounters[c.QuotaID]; mem != nil {
				mem.Dirty = true
			}
			f.markDirty(c.QuotaID)
		}
		quotasMux.Unlock()
		return
	}

	quotasMux.Lock()
	for _, c := range batch {
		if mem := quotaCounters[c.QuotaID]; mem != nil {
			if mem.UsedBytes == c.UsedBytes && mem.ReservedBytes == c.ReservedBytes {
				mem.Dirty = false
			} else if quotaFlusher != nil {
				quotaFlusher.markDirty(c.QuotaID)
			}
		}
	}
	quotasMux.Unlock()
}

func (f *quotaCounterFlusher) Stop() {
	f.mu.Lock()
	if f.stopped {
		f.mu.Unlock()
		return
	}
	f.stopped = true
	f.mu.Unlock()
	close(f.stopCh)
	<-f.doneCh
}

// OnUserScopeQuotaChanged ensures counter exists when scope quota is enabled.
func OnUserScopeQuotaChanged(sq *users.ScopeQuota) error {
	if sq == nil || sq.LimitBytes <= 0 {
		return nil
	}
	if sq.ID == "" {
		sq.ID = uuid.New().String()
	}
	quotasMux.Lock()
	defer quotasMux.Unlock()
	return ensureQuotaCounterMemLocked(sq.ID)
}

// OnUserScopeQuotaRemoved deletes counter for a removed or disabled scope quota.
func OnUserScopeQuotaRemoved(quotaID string) error {
	if quotaID == "" {
		return nil
	}
	quotasMux.Lock()
	defer quotasMux.Unlock()
	delete(quotaCounters, quotaID)
	delete(pendingIndexDelta, quotaID)
	return sqlDb.DeleteQuotaCounter(quotaID)
}
