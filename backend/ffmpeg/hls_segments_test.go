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

func TestBuildHLSSegmentTimelineKeyframesMergeToTarget(t *testing.T) {
	t.Parallel()
	kf := []float64{0, 2, 4, 6, 8, 10}
	starts, durs := BuildHLSSegmentTimeline(12, kf)
	if len(starts) != 3 {
		t.Fatalf("starts len = %d, want 3 (~4s merged segments)", len(starts))
	}
	if starts[0] != 0 || durs[0] != 4 {
		t.Fatalf("segment 0 = %.3f/%.3f, want 0/4", starts[0], durs[0])
	}
	if starts[1] != 4 || durs[1] != 4 {
		t.Fatalf("segment 1 = %.3f/%.3f, want 4/4", starts[1], durs[1])
	}
	if starts[2] != 8 || durs[2] != 4 {
		t.Fatalf("segment 2 = %.3f/%.3f, want 8/4", starts[2], durs[2])
	}
}

func TestSanitizeHLSKeyframesRejectsDenseProbes(t *testing.T) {
	t.Parallel()
	dense := make([]float64, 3000)
	for i := range dense {
		dense[i] = float64(i) / 30.0
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

func TestKeyframeSeekBefore(t *testing.T) {
	t.Parallel()
	kf := []float64{0, 5, 10, 15}
	if got := KeyframeSeekBefore(kf, 12); got != 10 {
		t.Fatalf("KeyframeSeekBefore(12) = %v, want 10", got)
	}
	if got := KeyframeSeekBefore(kf, 3); got != 0 {
		t.Fatalf("KeyframeSeekBefore(3) = %v, want 0", got)
	}
	if got := KeyframeSeekBefore(nil, 5); got != 0 {
		t.Fatalf("KeyframeSeekBefore(nil) = %v, want 0", got)
	}
}

func TestBuildHLSSegmentOptionsDelegates(t *testing.T) {
	t.Parallel()
	SetActiveHLSConfig(DefaultOnDemandHLSConfig())
	starts := []float64{0, 4, 8}
	durs := []float64{4, 4, 4}
	params := HLSSegmentParams{VideoCopy: true, GOP: 120}
	opts := BuildHLSSegmentOptions("/media/file.mkv", 2, params, starts, durs, true, []float64{0, 4, 8})
	if opts.Input.URL != "/media/file.mkv" {
		t.Fatalf("Input.URL = %q", opts.Input.URL)
	}
	if !opts.VideoCopy {
		t.Fatal("expected VideoCopy from params")
	}
	if opts.MediaTimelineSec != 8 {
		t.Fatalf("MediaTimelineSec = %v, want 8", opts.MediaTimelineSec)
	}
	if opts.DurationSec != 4 {
		t.Fatalf("DurationSec = %v, want 4", opts.DurationSec)
	}
}

func TestSanitizeHLSKeyframesAllowsFrequentIFrames(t *testing.T) {
	t.Parallel()
	// ~0.5 keyframes/sec over long duration (typical for digitized tape).
	kf := make([]float64, 0, 3500)
	for t := 0.0; t < 6800; t += 1.9 {
		kf = append(kf, t)
	}
	got := SanitizeHLSKeyframes(kf, 6805)
	if len(got) == 0 {
		t.Fatal("expected frequent but valid keyframes to be kept")
	}
}
