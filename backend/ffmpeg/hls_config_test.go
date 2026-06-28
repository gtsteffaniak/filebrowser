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

func TestHLSUseVideoCopy(t *testing.T) {
	eac3 := StreamInfo{HasVideo: true, VideoCodec: "h264", AudioCodec: "eac3", Height: 1080}
	if !HLSUseVideoCopy(eac3, HLSProfileQuality, 0) {
		t.Fatal("quality mode should use video copy for h264+eac3")
	}
	if HLSUseVideoCopy(eac3, HLSProfileOptimized, 0) {
		t.Fatal("optimized mode should not use video copy")
	}
	if HLSUseVideoCopy(eac3, HLSProfileDataSaver, 0) {
		t.Fatal("datasaver mode should not use video copy")
	}
	if !HLSNeedsFullVideoTranscode(eac3, HLSProfileOptimized, 0) {
		t.Fatal("optimized should require full transcode")
	}
}

func TestPlaylistConfigComment(t *testing.T) {
	comment := DefaultOnDemandHLSConfig().PlaylistConfigComment()
	want := "#EXT-X-FB-CONFIG:mode=on-demand;seg=4;buffer=3"
	if comment != want {
		t.Fatalf("comment = %q, want %q", comment, want)
	}
}
