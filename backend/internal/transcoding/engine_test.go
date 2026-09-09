package transcoding

import (
	"testing"

	"github.com/gtsteffaniak/filebrowser/backend/internal/ffmpeg"
)

func TestContinuousSeekParams(t *testing.T) {
	t.Parallel()
	const seg = 4.0

	tests := []struct {
		name       string
		startSec   float64
		wantIndex  int
		wantFFmpeg float64
	}{
		{name: "zero", startSec: 0, wantIndex: 0, wantFFmpeg: 0},
		{name: "within first segment", startSec: 3.44, wantIndex: 0, wantFFmpeg: 0},
		{name: "segment boundary", startSec: 4, wantIndex: 1, wantFFmpeg: 4},
		{name: "mid segment", startSec: 5.5, wantIndex: 1, wantFFmpeg: 4},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()
			idx, ffStart := continuousSeekParams(tc.startSec, seg)
			if idx != tc.wantIndex || ffStart != tc.wantFFmpeg {
				t.Fatalf("continuousSeekParams(%v, %v) = (%d, %v), want (%d, %v)",
					tc.startSec, seg, idx, ffStart, tc.wantIndex, tc.wantFFmpeg)
			}
		})
	}
}

func TestTranscodePlanUsesFixedSegmentGrid(t *testing.T) {
	t.Parallel()
	const segDur = 4.0
	duration := 30.5

	// Sparse WMV-like keyframes that do not match the output grid.
	keyframes := []float64{0, 3.909, 7.818, 11.727}
	keyframes = ffmpeg.SanitizeHLSKeyframes(keyframes, duration)
	_, keyframeDurs := ffmpeg.BuildHLSSegmentTimeline(duration, keyframes, segDur)

	_, fixedDurs := ffmpeg.BuildHLSSegmentTimeline(duration, nil, segDur)

	if len(keyframeDurs) == len(fixedDurs) {
		same := true
		for i := range fixedDurs {
			if keyframeDurs[i] != fixedDurs[i] {
				same = false
				break
			}
		}
		if same {
			t.Fatal("expected keyframe-based durations to differ from fixed grid for sparse keyframes")
		}
	}

	lastFixed := fixedDurs[len(fixedDurs)-1]
	if lastFixed <= 0 || lastFixed > segDur {
		t.Fatalf("tail segment duration %v out of range", lastFixed)
	}
	for i, dur := range fixedDurs[:len(fixedDurs)-1] {
		if dur != segDur {
			t.Fatalf("fixed segment %d duration = %v, want %v", i, dur, segDur)
		}
	}
}
