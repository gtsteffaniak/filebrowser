package ffmpeg

import "testing"

func TestBuildHLSSegmentTimelineMergesTinyTail(t *testing.T) {
	t.Parallel()
	starts, durs := BuildHLSSegmentTimeline(32.1, nil)
	if len(starts) != 8 {
		t.Fatalf("starts len = %d, want 8 (tiny tail merged)", len(starts))
	}
	if starts[7] != 28 || durs[7] < 4.0 || durs[7] > 4.2 {
		t.Fatalf("last segment = %.3f/%.3f, want start 28 and dur ~4.1", starts[7], durs[7])
	}
}

func TestBuildHLSSegmentTimelineFixedGrid(t *testing.T) {
	t.Parallel()
	starts, durs := BuildHLSSegmentTimeline(10, nil)
	if len(starts) != 3 {
		t.Fatalf("starts len = %d, want 3", len(starts))
	}
	if starts[0] != 0 || durs[0] != 4 || starts[2] != 8 || durs[2] != 2 {
		t.Fatalf("unexpected timeline: starts=%v durs=%v", starts, durs)
	}
}

func TestBuildHLSSegmentTimelineKeyframes(t *testing.T) {
	t.Parallel()
	starts, durs := BuildHLSSegmentTimeline(12, []float64{0, 5, 10})
	if len(starts) != 3 {
		t.Fatalf("starts len = %d, want 3", len(starts))
	}
	if starts[1] != 5 || durs[1] != 5 {
		t.Fatalf("segment 1 = %.3f/%.3f, want 5/5", starts[1], durs[1])
	}
	if starts[2] != 10 || durs[2] != 2 {
		t.Fatalf("segment 2 = %.3f/%.3f, want 10/2", starts[2], durs[2])
	}
}

func TestSanitizeHLSKeyframesRejectsDenseProbes(t *testing.T) {
	t.Parallel()
	dense := make([]float64, 200)
	for i := range dense {
		dense[i] = float64(i) * 0.4
	}
	if got := SanitizeHLSKeyframes(dense, 100); got != nil {
		t.Fatalf("expected nil for dense corrupt keyframes, got len=%d", len(got))
	}
}

func TestSanitizeHLSKeyframesAcceptsSparse(t *testing.T) {
	t.Parallel()
	got := SanitizeHLSKeyframes([]float64{0, 5, 10, 15}, 20)
	if len(got) != 4 {
		t.Fatalf("got len=%d, want 4", len(got))
	}
}

func TestSanitizeHLSKeyframesAllowsLongGOP(t *testing.T) {
	t.Parallel()
	got := SanitizeHLSKeyframes([]float64{0, 5.005, 27.945, 32.908}, 60)
	if len(got) != 4 {
		t.Fatalf("got len=%d, want 4", len(got))
	}
}
