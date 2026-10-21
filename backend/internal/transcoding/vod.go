package transcoding

import (
	"strconv"
	"strings"

	"github.com/gtsteffaniak/filebrowser/backend/internal/ffmpeg"
)

const (
	vodSegmentWaitTimeout  = 90
	vodSegmentPollInterval = 100 // milliseconds
	vodCompleteMarker      = "vod.complete"
)

// vodSeekParams maps a requested playlist time to the ffmpeg job anchor.
func vodSeekParams(plan hlsPlan, requestedSec float64) (startIndex int, playlistStartSec float64) {
	if requestedSec <= 0 || plan.vodList.Count() == 0 {
		return 0, 0
	}
	return ffmpeg.SegmentIndexForTime(requestedSec, plan.vodList.Starts)
}

func parseSegmentIndex(name string) (int, error) {
	base := strings.TrimSuffix(strings.TrimSpace(name), ".m4s")
	if base == "" {
		return 0, ErrInvalidSegment
	}
	n, err := strconv.Atoi(base)
	if err != nil || n < 0 {
		return 0, ErrInvalidSegment
	}
	return n, nil
}

func vodPlaylistStartSec(plan hlsPlan, index int) float64 {
	if index < 0 || index >= plan.vodList.Count() {
		return 0
	}
	return plan.vodList.Starts[index]
}

// planUsesExplicitCopy reports whether the plan runs copy/remux jobs with the
// aligned -f segment muxer (SegmentList passed to the job).
func planUsesExplicitCopy(plan hlsPlan) bool {
	return (plan.remux || plan.videoCopy) && plan.vodList.Count() > 0
}

// vodDecisionConfig returns the Jellyfin-style wait/restart thresholds for a plan.
func vodDecisionConfig(plan hlsPlan) ffmpeg.HLSVODDecisionConfig {
	return ffmpeg.HLSVODDecisionConfig{
		SegmentSec:   plan.segmentSec,
		GapMediaSec:  ffmpeg.DefaultHLSVODForwardGapSec,
		SegmentCount: plan.vodList.Count(),
	}
}
