package ffmpeg

import (
	"testing"
	"time"
)

func TestDefaultOnDemandHLSConfig(t *testing.T) {
	cfg := DefaultOnDemandHLSConfig()
	if cfg.Mode != HLSModeOnDemand {
		t.Fatalf("mode = %q, want on-demand", cfg.Mode)
	}
	if cfg.SegmentDurationSec != 4.0 {
		t.Fatalf("segment duration = %v, want 4", cfg.SegmentDurationSec)
	}
	if cfg.WarmPlaylistSegments != 3 {
		t.Fatalf("warm segments = %d, want 3", cfg.WarmPlaylistSegments)
	}
	if cfg.PlayerBufferSegments != 3 {
		t.Fatalf("buffer segments = %d, want 3", cfg.PlayerBufferSegments)
	}
	if cfg.PlayerBufferAheadSec() != 12 {
		t.Fatalf("buffer ahead = %v, want 12", cfg.PlayerBufferAheadSec())
	}
}

func TestHLSConfigNormalized(t *testing.T) {
	cfg := HLSConfig{SegmentDurationSec: 6, PlayerBufferSegments: 2}.Normalized()
	if cfg.SegmentDurationSec != 6 {
		t.Fatalf("segment duration = %v", cfg.SegmentDurationSec)
	}
	if cfg.PlayerBufferAheadSec() != 12 {
		t.Fatalf("buffer ahead = %v, want 12", cfg.PlayerBufferAheadSec())
	}
	if cfg.SegmentEncodeTimeout != 25*time.Second {
		t.Fatalf("encode timeout = %v", cfg.SegmentEncodeTimeout)
	}
}

func TestHLSConfigModeNormalized(t *testing.T) {
	for _, mode := range []HLSMode{"", HLSModeOnDemand, HLSModeDiskCache, HLSModeLongSegment, "bogus"} {
		cfg := HLSConfig{Mode: mode}.Normalized()
		if cfg.Mode != HLSModeOnDemand {
			t.Fatalf("mode %q normalized to %q, want on-demand", mode, cfg.Mode)
		}
	}
}

func TestPlaylistConfigComment(t *testing.T) {
	comment := DefaultOnDemandHLSConfig().PlaylistConfigComment()
	want := "#EXT-X-FB-CONFIG:mode=on-demand;seg=4;buffer=3"
	if comment != want {
		t.Fatalf("comment = %q, want %q", comment, want)
	}
}
