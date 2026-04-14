package indexing

import (
	"testing"
	"time"
)

func TestAlignCeilMinute(t *testing.T) {
	loc := time.UTC
	base := time.Date(2026, 4, 14, 12, 34, 0, 0, loc)
	if got := alignCeilMinute(base); !got.Equal(base) {
		t.Fatalf("on boundary: got %v want %v", got, base)
	}
	withSec := time.Date(2026, 4, 14, 12, 34, 1, 0, loc)
	want := time.Date(2026, 4, 14, 12, 35, 0, 0, loc)
	if got := alignCeilMinute(withSec); !got.Equal(want) {
		t.Fatalf("ceil: got %v want %v", got, want)
	}
}

func TestComputeNextAlignedRunMinuteTier(t *testing.T) {
	loc := time.UTC
	last := time.Date(2026, 4, 14, 12, 0, 0, 0, loc)
	ref := time.Date(2026, 4, 14, 12, 0, 30, 0, loc)
	interval := 10 * time.Minute
	got := computeNextAlignedRun(last, ref, interval)
	// minNext = 12:10, after ref 12:00:30 ->12:10, align minute -> 12:10
	want := time.Date(2026, 4, 14, 12, 10, 0, 0, loc)
	if !got.Equal(want) {
		t.Fatalf("got %v want %v", got, want)
	}
}

func TestComputeNextAlignedRunHourTierUses5MinGrid(t *testing.T) {
	loc := time.UTC
	last := time.Date(2026, 4, 14, 12, 0, 0, 0, loc)
	ref := time.Date(2026, 4, 14, 12, 3, 0, 0, loc)
	interval := 1 * time.Hour
	got := computeNextAlignedRun(last, ref, interval)
	if got.Minute()%5 != 0 || got.Second() != 0 {
		t.Fatalf("expected 5-minute grid, got %v", got)
	}
	if got.Before(ref) {
		t.Fatalf("got %v before ref %v", got, ref)
	}
}
