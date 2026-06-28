package http

import (
	"sync"
)

const (
	streamAheadSeconds    = 30
	maxSuffixRangeBytes   = 16 << 20 // MKV/WebM index reads at EOF
	streamSeekResetGap    = 8 << 20  // treat as seek when range start jumps back this far
	streamForwardJumpGap  = 4 << 20  // discontiguous forward jump opens a new window
)

type streamFetchWindow struct {
	mu        sync.Mutex
	anchor    int64
	highWater int64
	maxSpan   int64
}

var streamFetchWindows sync.Map

func streamMaxForwardSpan(fileSize int64, durationSec int) int64 {
	if fileSize <= 0 {
		return 8 << 20
	}
	if durationSec <= 0 {
		// ~2 Mbps fallback for 30s when duration unknown
		return 8 << 20
	}
	span := int64(float64(fileSize) * float64(streamAheadSeconds) / float64(durationSec))
	const minSpan = 4 << 20
	const maxSpan = 80 << 20
	if span < minSpan {
		return minSpan
	}
	if span > maxSpan {
		return maxSpan
	}
	return span
}

func getStreamFetchWindow(token string, fileSize int64, durationSec int) *streamFetchWindow {
	maxSpan := streamMaxForwardSpan(fileSize, durationSec)
	if existing, ok := streamFetchWindows.Load(token); ok {
		if win, ok := existing.(*streamFetchWindow); ok {
			return win
		}
	}
	w := &streamFetchWindow{maxSpan: maxSpan}
	actual, _ := streamFetchWindows.LoadOrStore(token, w)
	win, ok := actual.(*streamFetchWindow)
	if !ok {
		return w
	}
	win.mu.Lock()
	if win.maxSpan == 0 {
		win.maxSpan = maxSpan
	}
	win.mu.Unlock()
	return win
}

func clearStreamFetchWindow(token string) {
	if token != "" {
		streamFetchWindows.Delete(token)
	}
}

// applyStreamFetchBudget limits sequential forward byte reads to ~30s of media.
// Suffix ranges (e.g. MKV cues/index at EOF) are allowed separately.
func applyStreamFetchBudget(token string, fileSize int64, durationSec int, start, end int64, isSuffix bool) (int64, int64, bool) {
	if token == "" {
		return start, end, true
	}

	if isSuffix {
		if end-start+1 > maxSuffixRangeBytes {
			end = start + maxSuffixRangeBytes - 1
		}
		return start, end, true
	}

	win := getStreamFetchWindow(token, fileSize, durationSec)
	win.mu.Lock()
	defer win.mu.Unlock()

	if win.maxSpan <= 0 {
		win.maxSpan = streamMaxForwardSpan(fileSize, durationSec)
	}

	if win.highWater == 0 && win.anchor == 0 {
		win.anchor = start
		win.highWater = start
	}

	if start+streamSeekResetGap < win.anchor {
		win.anchor = start
		win.highWater = start
	} else if start > win.highWater+streamForwardJumpGap {
		win.anchor = start
		win.highWater = start
	}

	// Rolling read-ahead cap from the farthest granted byte; anchor is only for seek detection.
	windowEnd := win.highWater + win.maxSpan
	if start >= windowEnd {
		return start, end, false
	}
	if end >= windowEnd {
		end = windowEnd - 1
	}
	if end < start {
		return start, end, false
	}

	if end+1 > win.highWater {
		win.highWater = end + 1
	}
	return start, end, true
}
