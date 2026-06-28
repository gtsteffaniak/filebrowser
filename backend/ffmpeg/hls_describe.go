package ffmpeg

import (
	"fmt"
	"strings"
)

// DescribeHLSEncodePlan summarizes the encode path and resolved HW/SW codecs for logging.
func (s *Service) DescribeHLSEncodePlan(params HLSSegmentParams) string {
	if s == nil || s.inner == nil {
		return "path=unavailable"
	}
	plan := s.inner.DescribeHLSSegmentPlan(params)
	cfg := ActiveHLSConfig().Normalized()
	if strings.Contains(plan, "throttle=off") {
		return strings.Replace(plan, "throttle=off", fmt.Sprintf("throttle=off(hls-%s)", cfg.Mode), 1)
	}
	return plan + fmt.Sprintf(" hls=%s", cfg.Mode)
}
