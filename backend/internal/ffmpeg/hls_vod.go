package ffmpeg

import (
	"context"

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

// HLSVODSegmentReady reports whether a segment file is a complete fMP4 fragment.
func HLSVODSegmentReady(path string) (bool, error) {
	return ops.HLSVODSegmentReady(path)
}

// StartHLSVODSegmentJob launches ffmpeg writing numbered fMP4 segments (Model A).
func (s *Service) StartHLSVODSegmentJob(ctx context.Context, opts HLSVODJobOptions) (*HLSVODSegmentJob, error) {
	if s == nil || s.inner == nil {
		return nil, errUnavailable()
	}
	return s.inner.StartHLSVODSegmentJob(ctx, opts)
}
