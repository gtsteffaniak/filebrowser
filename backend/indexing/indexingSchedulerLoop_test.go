package indexing

import (
	"testing"
	"time"
)

func TestAlignCeilToFiveMinutes(t *testing.T) {
	loc := time.UTC
	base := time.Date(2026, 4, 14, 12, 35, 0, 0, loc)
	step := 5 * time.Minute
	if got := alignCeilToInterval(base, step); !got.Equal(base) {
		t.Fatalf("on boundary: got %v want %v", got, base)
	}
	withSec := time.Date(2026, 4, 14, 12, 35, 1, 0, loc)
	want := time.Date(2026, 4, 14, 12, 40, 0, 0, loc)
	if got := alignCeilToInterval(withSec, step); !got.Equal(want) {
		t.Fatalf("ceil: got %v want %v", got, want)
	}
}

func TestComputeNextSlotTimeTenMinuteGrid(t *testing.T) {
	loc := time.UTC
	last := time.Date(2026, 4, 14, 12, 0, 0, 0, loc)
	ref := time.Date(2026, 4, 14, 12, 0, 30, 0, loc)
	interval := 10 * time.Minute
	got := computeNextSlotTime(last, ref, interval)
	want := time.Date(2026, 4, 14, 12, 10, 0, 0, loc)
	if !got.Equal(want) {
		t.Fatalf("got %v want %v", got, want)
	}
}

func TestScheduleTierBoundsForComplexity(t *testing.T) {
	n := len(scanScheduleTiers) - 1
	tests := []struct {
		c       uint
		min, max int
	}{
		{0, 0, n},
		{1, 0, 4},
		{3, 0, n},
		{4, 1, n},
		{6, 2, n},
		{8, 3, n},
		{10, 4, n},
	}
	for _, tt := range tests {
		min, max := scheduleTierBoundsForComplexity(tt.c)
		if min != tt.min || max != tt.max {
			t.Fatalf("complexity %d: got min=%d max=%d want min=%d max=%d", tt.c, min, max, tt.min, tt.max)
		}
	}
}

func TestComputeNextSlotTimeHourUsesHourGrid(t *testing.T) {
	loc := time.UTC
	last := time.Date(2026, 4, 14, 12, 0, 0, 0, loc)
	ref := time.Date(2026, 4, 14, 12, 3, 0, 0, loc)
	interval := 1 * time.Hour
	got := computeNextSlotTime(last, ref, interval)
	step := int64(interval / time.Second)
	if got.Unix()%step != 0 {
		t.Fatalf("expected multiple of %v, got %v", interval, got)
	}
	if got.Before(ref) {
		t.Fatalf("got %v before ref %v", got, ref)
	}
}
