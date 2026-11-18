package gomedia

import (
	"sort"
	"sync"
	"time"
)

// shadowStats tallies every shadow comparison per operation for the regressions
// view. It lives in memory only and resets when the process restarts or the
// regressions are cleared. It is not written to the database.
var shadowStats = &shadowTally{ops: map[string]*opTally{}}

// OpStats is the tally for one operation.
type OpStats struct {
	Operation string `json:"operation"`
	// Runs counts shadow comparisons: go-media succeeded, or it missed and ffmpeg ran.
	Runs int64 `json:"runs"`
	// Succeeded counts runs where go-media returned a result.
	Succeeded int64 `json:"succeeded"`
	// Regressions counts runs that recorded a regression.
	Regressions int64 `json:"regressions"`
	// PairedRuns counts runs where both go-media and ffmpeg succeeded. Averages
	// use only these runs, so the two sides are compared on the same inputs.
	PairedRuns  int64   `json:"pairedRuns"`
	GoAvgMs     float64 `json:"goAvgMs"`
	FfmpegAvgMs float64 `json:"ffmpegAvgMs"`
}

// shadowObservation is the outcome of one shadow comparison.
type shadowObservation struct {
	goOK      bool
	goLatency time.Duration
	ffOK      bool
	ffLatency time.Duration
	regressed bool
}

type opTally struct {
	runs, succeeded, regressions, paired int64
	goSum, ffSum                         time.Duration
}

type shadowTally struct {
	mu  sync.Mutex
	ops map[string]*opTally
}

func (t *shadowTally) observe(op string, o shadowObservation) {
	t.mu.Lock()
	defer t.mu.Unlock()
	tally := t.ops[op]
	if tally == nil {
		tally = &opTally{}
		t.ops[op] = tally
	}
	tally.runs++
	if o.goOK {
		tally.succeeded++
	}
	if o.regressed {
		tally.regressions++
	}
	if o.goOK && o.ffOK {
		tally.paired++
		tally.goSum += o.goLatency
		tally.ffSum += o.ffLatency
	}
}

func (t *shadowTally) reset() {
	t.mu.Lock()
	defer t.mu.Unlock()
	t.ops = map[string]*opTally{}
}

// ShadowStats returns the current tally for every operation seen since the last
// reset, sorted by operation name.
func ShadowStats() []OpStats {
	shadowStats.mu.Lock()
	defer shadowStats.mu.Unlock()
	out := make([]OpStats, 0, len(shadowStats.ops))
	for name, tally := range shadowStats.ops {
		s := OpStats{
			Operation:   name,
			Runs:        tally.runs,
			Succeeded:   tally.succeeded,
			Regressions: tally.regressions,
			PairedRuns:  tally.paired,
		}
		if tally.paired > 0 {
			s.GoAvgMs = ms(tally.goSum) / float64(tally.paired)
			s.FfmpegAvgMs = ms(tally.ffSum) / float64(tally.paired)
		}
		out = append(out, s)
	}
	sort.Slice(out, func(i, j int) bool { return out[i].Operation < out[j].Operation })
	return out
}

// ResetShadowStats clears the tally for every operation.
func ResetShadowStats() {
	shadowStats.reset()
}

// ShadowStatsObserveForTest records one run for op. Test helper only.
func ShadowStatsObserveForTest(op string) {
	shadowStats.observe(op, shadowObservation{goOK: true})
}
