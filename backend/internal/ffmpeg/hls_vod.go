package ffmpeg

import (
	"context"
	"os"

	goffmpeg "github.com/gtsteffaniak/go-ffmpeg"
	"github.com/gtsteffaniak/go-ffmpeg/ops"
)

// VODSegmentList is the segment timeline for Model A VOD playlists.
type VODSegmentList = ops.VODSegmentList

// VODPlaylistOptions configures URIs in BuildVODPlaylist.
type VODPlaylistOptions = ops.VODPlaylistOptions

// HLSVODJobOptions configures StartHLSVODSegmentJob.
type HLSVODJobOptions = ops.HLSVODJobOptions

// HLSVODSegmentJob runs a forward ffmpeg HLS encode for on-demand VOD.
type HLSVODSegmentJob = ops.HLSVODSegmentJob

// HLSVODSession coordinates one cache directory / transcode profile
// (Jellyfin DynamicHls-style): job tracking, wait/restart decisions, partial
// segment cleanup, and per-path read leases so in-flight readers survive
// segment rewrites.
type HLSVODSession = ops.HLSVODSession

// HLSVODSessionOptions configures NewHLSVODSession.
type HLSVODSessionOptions = ops.HLSVODSessionOptions

// HLSVODDecisionConfig tunes Jellyfin-style wait/restart thresholds.
type HLSVODDecisionConfig = ops.HLSVODDecisionConfig

// HLSVODJobView is a snapshot of the active encode job for segment decisions.
type HLSVODJobView = ops.HLSVODJobView

// HLSVODSegmentAction is the host/library decision for an on-demand segment request.
type HLSVODSegmentAction = ops.HLSVODSegmentAction

const (
	// HLSVODActionServe serves the segment immediately.
	HLSVODActionServe = ops.HLSVODActionServe
	// HLSVODActionWait waits for production to reach the segment.
	HLSVODActionWait = ops.HLSVODActionWait
	// HLSVODActionRestart repositions the encode job to the segment.
	HLSVODActionRestart = ops.HLSVODActionRestart
)

// DefaultHLSVODForwardGapSec is the forward wait window before restarting a
// job (Jellyfin DynamicHls default: 24s of media).
const DefaultHLSVODForwardGapSec = ops.DefaultHLSVODForwardGapSec

// NewHLSVODSession returns a session coordinating VOD output under outDir.
func NewHLSVODSession(opts HLSVODSessionOptions) *HLSVODSession {
	return ops.NewHLSVODSession(opts)
}

// DecideHLSVODSegmentAction implements the Model A serve/wait/restart rules.
func DecideHLSVODSegmentAction(cfg HLSVODDecisionConfig, job HLSVODJobView, k int, ready bool) HLSVODSegmentAction {
	return ops.DecideHLSVODSegmentAction(cfg, job, k, ready)
}

// BuildVODSegmentList plans segment boundaries (fixed grid or keyframe snap).
func BuildVODSegmentList(mediaDurationSec, segmentSec, startTimeSec float64, keyframes []float64) VODSegmentList {
	return goffmpeg.BuildVODSegmentList(mediaDurationSec, segmentSec, startTimeSec, keyframes)
}

// SegmentIndexForTime maps playlist seconds to segment index and start time.
func SegmentIndexForTime(timeSec float64, starts []float64) (index int, startSec float64) {
	return goffmpeg.SegmentIndexForTime(timeSec, starts)
}

// DefaultVODPlaylistOptions returns standard init/segment URIs.
func DefaultVODPlaylistOptions() VODPlaylistOptions {
	return goffmpeg.DefaultVODPlaylistOptions()
}

// BuildVODPlaylist renders a synthetic #EXT-X-PLAYLIST-TYPE:VOD media playlist.
func BuildVODPlaylist(list VODSegmentList, opts VODPlaylistOptions) string {
	return goffmpeg.BuildVODPlaylist(list, opts)
}

// HLSVODSegmentPath returns the absolute path to a segment under a job directory.
func HLSVODSegmentPath(jobDir string, index int) string {
	return ops.HLSVODSegmentPath(jobDir, index)
}

// HLSVODInitPath returns the absolute path to init.m4s under a job directory.
func HLSVODInitPath(outDir string) string {
	return ops.HLSVODInitPath(outDir)
}

// HLSVODSegmentReady reports whether a segment file is a complete fMP4 fragment.
func HLSVODSegmentReady(path string) (bool, error) {
	return ops.HLSVODSegmentReady(path)
}

// HLSVODOpenReadySegment opens path only when it is a publishable fMP4 media
// fragment. The caller must close the returned file.
func HLSVODOpenReadySegment(path string) (*os.File, bool, error) {
	return ops.HLSVODOpenReadySegment(path)
}

// HLSVODExplicitCopySegmentReady reports readiness for explicit-cut (-f segment)
// copy jobs: segment k is complete only when k+1 is listed in the live CSV, or
// the encode job has ended.
func HLSVODExplicitCopySegmentReady(outDir string, index int, jobRunning bool) (bool, error) {
	return ops.HLSVODExplicitCopySegmentReady(outDir, index, jobRunning)
}

// HLSVODOpenReadyExplicitCopySegment opens a publishable explicit-cut copy
// segment. The caller must close the returned file.
func HLSVODOpenReadyExplicitCopySegment(outDir string, index int, jobRunning bool) (*os.File, bool, error) {
	return ops.HLSVODOpenReadyExplicitCopySegment(outDir, index, jobRunning)
}

// StartHLSVODSegmentJob launches ffmpeg writing numbered fMP4 segments (Model A).
func (s *Service) StartHLSVODSegmentJob(ctx context.Context, opts HLSVODJobOptions) (*HLSVODSegmentJob, error) {
	if s == nil || s.inner == nil {
		return nil, errUnavailable()
	}
	return s.inner.StartHLSVODSegmentJob(ctx, opts)
}
