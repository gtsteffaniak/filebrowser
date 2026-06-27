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
	maxSpan := streamMaxForwardSpan(size, duration)

	start, end, ok := applyStreamFetchBudget(token, size, duration, 0, (4<<20)-1, false)
	if !ok || start != 0 {
		t.Fatalf("first chunk: ok=%v start=%d end=%d", ok, start, end)
	}

	// Exhaust the forward window with sequential reads.
	var lastEnd int64 = (4 << 20) - 1
	for lastEnd+maxSpan < size {
		nextStart := lastEnd + 1
		nextEnd := nextStart + (4 << 20) - 1
		_, end, allowed := applyStreamFetchBudget(token, size, duration, nextStart, nextEnd, false)
		if !allowed {
			break
		}
		lastEnd = end
	}

	_, _, allowed := applyStreamFetchBudget(token, size, duration, lastEnd+1, lastEnd+(4<<20), false)
	if allowed {
		t.Fatal("expected forward window to reject reads beyond ~30s span")
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
