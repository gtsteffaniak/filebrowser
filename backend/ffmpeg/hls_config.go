package ffmpeg

import (
	"net/http"
	"strconv"
	"sync"
	"time"

	goffmpeg "github.com/gtsteffaniak/go-ffmpeg"
)

// HLSMode identifies how HLS media is produced and served.
// Only HLSModeOnDemand is implemented today; other modes reserve extension points.
type HLSMode string

const (
	// HLSModeOnDemand encodes each fMP4 segment on first request (test-ffmpeg default).
	HLSModeOnDemand HLSMode = "on-demand"
	// HLSModeDiskCache serves pre-transcoded segments from disk (future Netflix-style path).
	HLSModeDiskCache HLSMode = "disk-cache"
	// HLSModeLongSegment uses longer independent segments to amortize ffmpeg startup (future).
	HLSModeLongSegment HLSMode = "long-segment"
)

const (
	DefaultHLSSegmentDurationSec = goffmpeg.DefaultHLSSegmentDurationSec
	defaultHLSWarmSegments       = 3
	defaultHLSPlayerBufferSegs   = 3
	defaultHLSDefaultGOP         = 120
)

// HLSConfig holds delivery tuning shared by backend encode and frontend playback.
type HLSConfig struct {
	Mode HLSMode `json:"mode"`

	// SegmentDurationSec is the target fMP4 segment length (on-demand grid).
	SegmentDurationSec float64 `json:"segmentDurationSec"`

	// WarmPlaylistSegments pre-encodes N segments after playlist load (seg 0 sync, 1..N async).
	WarmPlaylistSegments int `json:"warmPlaylistSegments"`

	// PlayerBufferSegments is how many segments hls.js should prefetch ahead of playhead.
	PlayerBufferSegments int `json:"playerBufferSegments"`

	// SegmentEncodeTimeout limits a single segment ffmpeg run.
	SegmentEncodeTimeout time.Duration `json:"-"`

	// KeyframeProbeTimeout limits keyframe probing for stream-copy paths.
	KeyframeProbeTimeout time.Duration `json:"-"`

	// DefaultGOP is used when fps probe fails (typically fps * segment duration).
	DefaultGOP int `json:"defaultGop"`
}

var (
	hlsConfigMu sync.RWMutex
	activeHLS   = DefaultOnDemandHLSConfig()
)

// DefaultOnDemandHLSConfig returns settings validated against the test-ffmpeg harness.
func DefaultOnDemandHLSConfig() HLSConfig {
	return HLSConfig{
		Mode:                 HLSModeOnDemand,
		SegmentDurationSec:   DefaultHLSSegmentDurationSec,
		WarmPlaylistSegments: defaultHLSWarmSegments,
		PlayerBufferSegments: defaultHLSPlayerBufferSegs,
		SegmentEncodeTimeout: 25 * time.Second,
		KeyframeProbeTimeout: 10 * time.Second,
		DefaultGOP:           defaultHLSDefaultGOP,
	}
}

// ActiveHLSConfig returns the HLS delivery configuration in use.
func ActiveHLSConfig() HLSConfig {
	hlsConfigMu.RLock()
	defer hlsConfigMu.RUnlock()
	return activeHLS
}

// SetActiveHLSConfig replaces the active config (tests and future settings integration).
func SetActiveHLSConfig(cfg HLSConfig) {
	cfg = cfg.Normalized()
	hlsConfigMu.Lock()
	activeHLS = cfg
	hlsConfigMu.Unlock()
}

// Normalized returns cfg with default fields filled in.
func (c HLSConfig) Normalized() HLSConfig {
	return c.withDefaults()
}

func (c HLSConfig) withDefaults() HLSConfig {
	out := c
	switch out.Mode {
	case "", HLSModeOnDemand:
		out.Mode = HLSModeOnDemand
	default:
		out.Mode = HLSModeOnDemand
	}
	if out.SegmentDurationSec <= 0 {
		out.SegmentDurationSec = DefaultHLSSegmentDurationSec
	}
	if out.WarmPlaylistSegments <= 0 {
		out.WarmPlaylistSegments = defaultHLSWarmSegments
	}
	if out.PlayerBufferSegments <= 0 {
		out.PlayerBufferSegments = defaultHLSPlayerBufferSegs
	}
	if out.SegmentEncodeTimeout <= 0 {
		out.SegmentEncodeTimeout = 25 * time.Second
	}
	if out.KeyframeProbeTimeout <= 0 {
		out.KeyframeProbeTimeout = 10 * time.Second
	}
	if out.DefaultGOP <= 0 {
		out.DefaultGOP = defaultHLSDefaultGOP
	}
	return out
}

// SegmentDurationSec returns the active segment duration in seconds.
func SegmentDurationSec() float64 {
	return ActiveHLSConfig().SegmentDurationSec
}

// PlayerBufferAheadSec returns recommended hls.js max buffer (segmentDuration * bufferSegments).
func (c HLSConfig) PlayerBufferAheadSec() float64 {
	return c.SegmentDurationSec * float64(c.PlayerBufferSegments)
}

const (
	hlsHeaderMode           = "X-HLS-Mode"
	hlsHeaderSegmentDur     = "X-HLS-Segment-Duration-Sec"
	hlsHeaderBufferSegments = "X-HLS-Player-Buffer-Segments"
)

// WriteHLSConfigHeaders exposes delivery parameters to the frontend player.
func WriteHLSConfigHeaders(w http.ResponseWriter, cfg HLSConfig) {
	if w == nil {
		return
	}
	cfg = cfg.Normalized()
	w.Header().Set(hlsHeaderMode, string(cfg.Mode))
	w.Header().Set(hlsHeaderSegmentDur, strconv.FormatFloat(cfg.SegmentDurationSec, 'f', -1, 64))
	w.Header().Set(hlsHeaderBufferSegments, strconv.Itoa(cfg.PlayerBufferSegments))
}

// PlaylistConfigComment returns an #EXT-X comment mirroring WriteHLSConfigHeaders (playlist fallback).
func (c HLSConfig) PlaylistConfigComment() string {
	c = c.withDefaults()
	return "#EXT-X-FB-CONFIG:mode=" + string(c.Mode) +
		";seg=" + strconv.FormatFloat(c.SegmentDurationSec, 'f', -1, 64) +
		";buffer=" + strconv.Itoa(c.PlayerBufferSegments)
}
