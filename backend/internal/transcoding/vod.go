package transcoding

import (
	"strconv"
	"strings"

	"github.com/gtsteffaniak/filebrowser/backend/internal/ffmpeg"
)

const (
	vodSegmentLookahead     = 2
	vodSegmentWaitTimeout   = 90
	vodSegmentPollInterval  = 100 // milliseconds
	vodCompleteMarker       = "vod.complete"
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
