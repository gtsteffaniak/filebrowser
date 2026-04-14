package indexing

import (
	"sort"
	"time"

	"github.com/gtsteffaniak/filebrowser/backend/common/utils"
	"github.com/gtsteffaniak/go-logger/logger"
)

// alignCeilToInterval rounds t up (UTC) to the next boundary where Unix time is a multiple of interval.
func alignCeilToInterval(t time.Time, interval time.Duration) time.Time {
	if interval < time.Minute {
		interval = time.Minute
	}
	stepNs := interval.Nanoseconds()
	total := t.UTC().UnixNano()
	aligned := (total + stepNs - 1) / stepNs * stepNs
	return time.Unix(0, aligned).UTC()
}

// computeNextSlotTime returns the next scheduled slot after a scan completes.
// interval must be one of scanScheduleTiers; alignment uses that interval as the grid step.
func computeNextSlotTime(lastScanned, ref time.Time, interval time.Duration) time.Time {
	minStep := scanScheduleTiers[0]
	if interval < minStep {
		interval = minStep
	}
	minNext := lastScanned.Add(interval)
	if minNext.Before(ref) {
		minNext = ref
	}
	return alignCeilToInterval(minNext, interval)
}

func (idx *Index) removeScannerFromSlotLocked(s *Scanner) {
	sec := s.calendarSlotSec
	if sec == 0 {
		return
	}
	if idx.scheduleSlots == nil {
		s.calendarSlotSec = 0
		return
	}
	lst := idx.scheduleSlots[sec]
	out := lst[:0]
	for _, x := range lst {
		if x != s {
			out = append(out, x)
		}
	}
	if len(out) == 0 {
		delete(idx.scheduleSlots, sec)
	} else {
		idx.scheduleSlots[sec] = out
	}
	s.calendarSlotSec = 0
}

// registerScannerNextRun removes the scanner from any previous slot and adds it at the given UTC time.
func (idx *Index) registerScannerNextRun(s *Scanner, when time.Time) {
	if !idx.useAdaptiveScheduling() {
		return
	}
	when = when.UTC().Truncate(time.Second)
	sec := when.Unix()
	idx.scheduleSlotsMu.Lock()
	defer idx.scheduleSlotsMu.Unlock()
	if idx.scheduleSlots == nil {
		idx.scheduleSlots = make(map[int64][]*Scanner)
	}
	idx.removeScannerFromSlotLocked(s)
	idx.scheduleSlots[sec] = append(idx.scheduleSlots[sec], s)
	s.calendarSlotSec = sec
}

func (idx *Index) takeDueScannerBatch(now time.Time) []*Scanner {
	idx.scheduleSlotsMu.Lock()
	defer idx.scheduleSlotsMu.Unlock()
	if idx.scheduleSlots == nil {
		return nil
	}
	var dueSec int64 = -1
	nowSec := now.Unix()
	for sec := range idx.scheduleSlots {
		if sec <= nowSec {
			if dueSec < 0 || sec < dueSec {
				dueSec = sec
			}
		}
	}
	if dueSec < 0 {
		return nil
	}
	lst := idx.scheduleSlots[dueSec]
	delete(idx.scheduleSlots, dueSec)
	for _, s := range lst {
		s.calendarSlotSec = 0
	}
	return lst
}

func (idx *Index) earliestSlotWake(now time.Time) time.Time {
	idx.scheduleSlotsMu.Lock()
	defer idx.scheduleSlotsMu.Unlock()
	if len(idx.scheduleSlots) == 0 {
		return now.Add(time.Minute)
	}
	nowSec := now.Unix()
	var earliest int64 = -1
	for sec := range idx.scheduleSlots {
		if sec <= nowSec {
			return now
		}
		if earliest < 0 || sec < earliest {
			earliest = sec
		}
	}
	if earliest < 0 {
		return now.Add(time.Minute)
	}
	return time.Unix(earliest, 0).UTC()
}

func (idx *Index) restoreScannerNextRuns() {
	if !idx.useAdaptiveScheduling() {
		return
	}
	idx.scheduleSlotsMu.Lock()
	idx.scheduleSlots = make(map[int64][]*Scanner)
	idx.scheduleSlotsMu.Unlock()

	idx.mu.RLock()
	n := len(idx.scanners)
	idx.mu.RUnlock()
	now := time.Now()
	for _, s := range idx.scanners {
		var next time.Time
		s.withStatsLock(func() {
			if s.lastScanned.IsZero() {
				s.nextRun = time.Time{}
				return
			}
			tier := utils.Clamp(s.currentSchedule, 0, len(scanScheduleTiers)-1)
			d := scanScheduleDuration(tier)
			next = computeNextSlotTime(s.lastScanned, now, d)
			s.nextRun = next
		})
		if !next.IsZero() {
			idx.registerScannerNextRun(s, next)
		}
	}
	logger.Debugf("[%s] scheduler: restored nextRun for %d scanners (adaptive)", idx.Name, n)
}

func sortedScannerPaths(idx *Index) []string {
	idx.mu.RLock()
	defer idx.mu.RUnlock()
	paths := make([]string, 0, len(idx.scanners))
	for p := range idx.scanners {
		paths = append(paths, p)
	}
	sort.Strings(paths)
	return paths
}

func (idx *Index) pruneGoneChildScanners() {
	idx.mu.RLock()
	var gone []string
	for path, s := range idx.scanners {
		if path == "/" {
			continue
		}
		if !s.directoryExists() {
			gone = append(gone, path)
		}
	}
	idx.mu.RUnlock()

	for _, path := range gone {
		idx.mu.RLock()
		s, ok := idx.scanners[path]
		idx.mu.RUnlock()
		if !ok {
			continue
		}
		idx.scheduleSlotsMu.Lock()
		idx.removeScannerFromSlotLocked(s)
		idx.scheduleSlotsMu.Unlock()
		idx.mu.Lock()
		delete(idx.scanners, path)
		idx.mu.Unlock()
		logger.Debugf("Scheduler removed scanner [%s]: path gone", path)
	}
}

func (idx *Index) needsAdaptiveBootstrap() bool {
	idx.mu.RLock()
	defer idx.mu.RUnlock()
	if idx.schedulerStartupPassPending {
		return true
	}
	for _, s := range idx.scanners {
		if s.lastScanned.IsZero() {
			return true
		}
	}
	return false
}

func (idx *Index) runSerialScanPass() {
	idx.pruneGoneChildScanners()
	paths := sortedScannerPaths(idx)
	idx.mu.Lock()
	idx.schedulerBatch++
	batchDepth := idx.schedulerBatch
	idx.mu.Unlock()
	logger.Debugf("[%s] scheduler: serialPass start paths=%d batchDepth=%d adaptive=%v",
		idx.Name, len(paths), batchDepth, idx.useAdaptiveScheduling())

	defer func() {
		idx.mu.Lock()
		idx.schedulerBatch--
		idx.mu.Unlock()
		if err := idx.PostScan(); err != nil {
			logger.Errorf("PostScan after serial pass: %v", err)
		}
		logger.Debugf("[%s] scheduler: serialPass done", idx.Name)
	}()

	for _, p := range paths {
		idx.mu.RLock()
		s, ok := idx.scanners[p]
		idx.mu.RUnlock()
		if !ok {
			continue
		}
		if s.scanPath != "/" && !s.directoryExists() {
			continue
		}
		s.executeScan()
	}
}

func (idx *Index) schedulerAdaptiveTick() {
	idx.pruneGoneChildScanners()
	if idx.needsAdaptiveBootstrap() {
		idx.mu.Lock()
		startupPass := idx.schedulerStartupPassPending
		idx.mu.Unlock()
		if startupPass {
			logger.Debugf("[%s] scheduler: adaptive startup (serial pass all scanners)", idx.Name)
		} else {
			logger.Debugf("[%s] scheduler: adaptive tick bootstrap (serial pass all scanners)", idx.Name)
		}
		idx.runSerialScanPass()
		if startupPass {
			idx.mu.Lock()
			idx.schedulerStartupPassPending = false
			idx.mu.Unlock()
		}
		return
	}
	drainedAny := false
	for {
		batch := idx.takeDueScannerBatch(time.Now())
		if len(batch) == 0 {
			break
		}
		drainedAny = true
		sort.Slice(batch, func(i, j int) bool {
			return batch[i].scanPath < batch[j].scanPath
		})
		firstPath := batch[0].scanPath
		logger.Debugf("[%s] scheduler: adaptive slot scanners=%d first=%s", idx.Name, len(batch), firstPath)
		for _, s := range batch {
			var nextRunStr string
			s.withStatsRLock(func() {
				if s.nextRun.IsZero() {
					nextRunStr = "zero(immediate)"
				} else {
					nextRunStr = s.nextRun.UTC().Format(time.RFC3339)
				}
			})
			logger.Debugf("[%s] scheduler: adaptive run path=%s nextRun=%s", idx.Name, s.scanPath, nextRunStr)
			s.executeScan()
		}
	}
	if !drainedAny {
		idx.mu.RLock()
		n := len(idx.scanners)
		idx.mu.RUnlock()
		now := time.Now()
		logger.Debugf("[%s] scheduler: adaptive tick no due slot (now=%v scanners=%d)", idx.Name, now.UTC().Format(time.RFC3339), n)
	}
}

func (idx *Index) runIndexScheduler() {
	defer func() {
		if r := recover(); r != nil {
			logger.Errorf("[%s] index scheduler panic recovered: %v", idx.Name, r)
		}
	}()

	if !idx.useAdaptiveScheduling() {
		idx.mu.RLock()
		n := len(idx.scanners)
		idx.mu.RUnlock()
		interval := time.Duration(idx.Config.IndexingInterval) * time.Minute
		logger.Debugf("[%s] scheduler: started fixed interval=%v scanners=%d", idx.Name, interval, n)
		idx.runSerialScanPass()
		for {
			select {
			case <-idx.schedulerStop:
				logger.Debugf("[%s] scheduler: fixed loop stopped", idx.Name)
				return
			case <-time.After(interval):
				logger.Debugf("[%s] scheduler: fixed interval wake", idx.Name)
				idx.runSerialScanPass()
			}
		}
	}

	idx.mu.RLock()
	n := len(idx.scanners)
	idx.mu.RUnlock()
	logger.Debugf("[%s] scheduler: started adaptive scanners=%d", idx.Name, n)
	idx.schedulerAdaptiveTick()
	for {
		now := time.Now()
		next := idx.earliestSlotWake(now)
		d := time.Until(next)
		if d < time.Second {
			d = time.Second
		}
		logger.Debugf("[%s] scheduler: adaptive sleep until=%v (in %v)", idx.Name, next.UTC().Format(time.RFC3339), d)
		select {
		case <-idx.schedulerStop:
			logger.Debugf("[%s] scheduler: adaptive loop stopped", idx.Name)
			return
		case <-time.After(d):
			idx.schedulerAdaptiveTick()
		}
	}
}

func (idx *Index) stopScheduler() {
	idx.schedulerStopOnce.Do(func() {
		if idx.schedulerStop != nil {
			logger.Debugf("[%s] scheduler: stop channel closed", idx.Name)
			close(idx.schedulerStop)
		}
	})
}
