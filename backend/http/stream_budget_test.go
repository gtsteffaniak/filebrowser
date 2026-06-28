package http

import "testing"

func TestStreamMaxForwardSpan(t *testing.T) {
	t.Parallel()
	const size = 6_326_251_520
	const duration = 3707
	got := streamMaxForwardSpan(size, duration)
	const wantMin = 40 << 20
	const wantMax = 80 << 20
	if got < wantMin || got > wantMax {
		t.Fatalf("span = %d, want between %d and %d", got, wantMin, wantMax)
	}
}

func TestApplyStreamFetchBudgetSequential(t *testing.T) {
	t.Parallel()
	token := "test-seq"
	t.Cleanup(func() { clearStreamFetchWindow(token) })

	const size = 100 << 20
	const duration = 1000

	start, end, ok := applyStreamFetchBudget(token, size, duration, 0, (4<<20)-1, false)
	if !ok || start != 0 {
		t.Fatalf("first chunk: ok=%v start=%d end=%d", ok, start, end)
	}

	// Sequential reads should keep advancing with a rolling forward window.
	var lastEnd int64 = (4 << 20) - 1
	for i := 0; i < 20; i++ {
		nextStart := lastEnd + 1
		nextEnd := nextStart + (4 << 20) - 1
		if nextEnd >= size {
			nextEnd = size - 1
		}
		_, end, allowed := applyStreamFetchBudget(token, size, duration, nextStart, nextEnd, false)
		if !allowed {
			t.Fatalf("chunk %d: expected rolling window to allow sequential read at %d", i, nextStart)
		}
		lastEnd = end
		if lastEnd+1 >= size {
			break
		}
	}

	// Reject contiguous read-ahead at the rolling window boundary (not a seek-sized jump).
	win := getStreamFetchWindow(token, size, duration)
	win.mu.Lock()
	highWater := win.highWater
	span := win.maxSpan
	win.mu.Unlock()

	beyondWindow := highWater + span
	_, _, allowed := applyStreamFetchBudget(token, size, duration, beyondWindow, beyondWindow+(4<<20)-1, false)
	if allowed {
		t.Fatalf("expected forward window to reject read at %d (highWater=%d span=%d)", beyondWindow, highWater, span)
	}

	// Large forward jumps within seek distance open a new window instead of being rejected.
	jumpStart := lastEnd + span + (4 << 20)
	if jumpStart <= highWater+streamForwardJumpGap {
		t.Fatalf("test setup: jumpStart %d should exceed seek reset threshold %d", jumpStart, highWater+streamForwardJumpGap)
	}
	_, _, allowed = applyStreamFetchBudget(token, size, duration, jumpStart, jumpStart+(4<<20)-1, false)
	if !allowed {
		t.Fatal("expected large forward jump to open a new read window")
	}
}

func TestApplyStreamFetchBudgetSeekResetsWindow(t *testing.T) {
	t.Parallel()
	token := "test-seek"
	t.Cleanup(func() { clearStreamFetchWindow(token) })

	const size = 200 << 20
	const duration = 1000
	_, _, ok := applyStreamFetchBudget(token, size, duration, 0, (4<<20)-1, false)
	if !ok {
		t.Fatal("expected initial range")
	}

	jumpStart := int64(120 << 20)
	jumpEnd := jumpStart + (4 << 20) - 1
	start, end, ok := applyStreamFetchBudget(token, size, duration, jumpStart, jumpEnd, false)
	if !ok || start != jumpStart || end != jumpEnd {
		t.Fatalf("seek reset: ok=%v start=%d end=%d", ok, start, end)
	}
}

func TestApplyStreamFetchBudgetAllowsSuffix(t *testing.T) {
	t.Parallel()
	token := "test-suffix"
	t.Cleanup(func() { clearStreamFetchWindow(token) })

	const size = 100 << 20
	start := int64(99 << 20)
	end := size - 1
	gotStart, gotEnd, ok := applyStreamFetchBudget(token, size, 1000, start, int64(end), true)
	if !ok {
		t.Fatal("expected suffix range")
	}
	if gotEnd-gotStart+1 > maxSuffixRangeBytes {
		t.Fatalf("suffix not capped: %d bytes", gotEnd-gotStart+1)
	}
}
