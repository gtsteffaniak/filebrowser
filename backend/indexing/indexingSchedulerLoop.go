package indexing

import (
	"sort"
	"time"

	"github.com/gtsteffaniak/go-logger/logger"
)

// alignCeilMinute rounds t up to the next whole minute (seconds and ns zeroed).
func alignCeilMinute(t time.Time) time.Time {
	if t.Nanosecond() == 0 && t.Second() == 0 {
		return t.Truncate(time.Minute)
	}
	return t.Truncate(time.Minute).Add(time.Minute)
}

// alignCeil5Min rounds t up to the next 5-minute boundary since Unix epoch.
func alignCeil5Min(t time.Time) time.Time {
	const step = int64(5 * 60)
	epoch := t.Unix()
	if epoch%step == 0 && t.Nanosecond() == 0 {
		return t.UTC().Truncate(time.Second).In(t.Location())
	}
	aligned := ((epoch + step - 1) / step) * step
	return time.Unix(aligned, 0).In(t.Location())
}

// computeNextAlignedRun returns the next scheduled wake time after a scan completes.
// interval is the pre-update tier interval (same semantics as legacy nextSleepTime).
func computeNextAlignedRun(lastScanned, ref time.Time, interval time.Duration) time.Time {
	if interval < time.Minute {
		interval = time.Minute
	}
	minNext := lastScanned.Add(interval)
	if minNext.Before(ref) {
		minNext = ref
	}
	if interval >= time.Hour {
		return alignCeil5Min(minNext)
	}
	return alignCeilMinute(minNext)
}

func (idx *Index) restoreScannerNextRuns() {
	if !idx.useAdaptiveScheduling() {
		return
	}
	idx.mu.RLock()
	n := len(idx.scanners)
	idx.mu.RUnlock()
	now := time.Now()
	for _, s := range idx.scanners {
		s.withStatsLock(func() {
			if s.lastScanned.IsZero() {
				s.nextRun = time.Time{}
				return
			}
			d := scanSchedule[s.currentSchedule] + s.smartModifier
			if d < time.Minute {
				d = time.Minute
			}
			s.nextRun = computeNextAlignedRun(s.lastScanned, now, d)
		})
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
	idx.mu.Lock()
	defer idx.mu.Unlock()
	for path, s := range idx.scanners {
		if path == "/" {
			continue
		}
		if !s.directoryExists() {
			delete(idx.scanners, path)
			logger.Debugf("Scheduler removed scanner [%s]: path gone", path)
		}
	}
}

// computeNextGlobalWake returns the earliest time any scanner needs to run, or ref if something is due now.
func (idx *Index) computeNextGlobalWake(ref time.Time) time.Time {
	idx.mu.RLock()
	defer idx.mu.RUnlock()
	var minNext time.Time
	for _, s := range idx.scanners {
		if s.nextRun.IsZero() || !s.nextRun.After(ref) {
			return ref
		}
		if minNext.IsZero() || s.nextRun.Before(minNext) {
			minNext = s.nextRun
		}
	}
	if minNext.IsZero() {
		return ref.Add(time.Minute)
	}
	return minNext
}

func (idx *Index) needsAdaptiveBootstrap() bool {
	idx.mu.RLock()
	defer idx.mu.RUnlock()
	for _, s := range idx.scanners {
		if s.lastScanned.IsZero() {
			return true
		}
	}
	return false
}

type dueScanner struct {
	path string
	s    *Scanner
}

func (idx *Index) pickNextDueScanner(now time.Time) *Scanner {
	idx.mu.RLock()
	candidates := make([]dueScanner, 0, len(idx.scanners))
	for path, s := range idx.scanners {
		if s.nextRun.IsZero() || !s.nextRun.After(now) {
			candidates = append(candidates, dueScanner{path: path, s: s})
		}
	}
	idx.mu.RUnlock()
	if len(candidates) == 0 {
		return nil
	}
	sort.Slice(candidates, func(i, j int) bool {
		ti, tj := candidates[i].s.nextRun, candidates[j].s.nextRun
		pi, pj := candidates[i].path, candidates[j].path
		if ti.IsZero() && tj.IsZero() {
			return pi < pj
		}
		if ti.IsZero() {
			return true
		}
		if tj.IsZero() {
			return false
		}
		if !ti.Equal(tj) {
			return ti.Before(tj)
		}
		return pi < pj
	})
	return candidates[0].s
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
		logger.Debugf("[%s] scheduler: adaptive tick bootstrap (serial pass all scanners)", idx.Name)
		idx.runSerialScanPass()
		return
	}
	now := time.Now()
	s := idx.pickNextDueScanner(now)
	if s == nil {
		idx.mu.RLock()
		n := len(idx.scanners)
		idx.mu.RUnlock()
		logger.Debugf("[%s] scheduler: adaptive tick no due scanner (now=%v scanners=%d)", idx.Name, now.UTC().Format(time.RFC3339), n)
		return
	}
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
		next := idx.computeNextGlobalWake(now)
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
