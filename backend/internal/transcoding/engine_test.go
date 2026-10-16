package transcoding

import (
	"strconv"
	"strings"
	"testing"

	"github.com/gtsteffaniak/filebrowser/backend/internal/ffmpeg"
	"github.com/gtsteffaniak/filebrowser/backend/pkg/settings"
)

func withTranscodeProfiles(t *testing.T) {
	t.Helper()
	original := settings.Config.Integrations.Media.Transcode.Profiles
	t.Cleanup(func() { settings.Config.Integrations.Media.Transcode.Profiles = original })
	settings.Config.Integrations.Media.Transcode.Profiles = settings.DefaultTranscodeProfiles()
}

// A profile that caps the height above the source resolution must not select a
// higher bitrate rung: the scale filter can only cap, never upscale.
func TestEncodeProfileForModeClampsToSourceHeight(t *testing.T) {
	withTranscodeProfiles(t)
	small := ffmpeg.StreamInfo{Height: 270, VideoBitrate: 0}

	if got := bitrateKbps(encodeProfileForMode(small, ProfileQuality).Bitrate.Target); got != 1500 {
		t.Fatalf("quality bitrate for 270p source = %d kbps, want the 1500 rung (no upscale)", got)
	}
	if got := bitrateKbps(encodeProfileForMode(small, ProfileBalanced).Bitrate.Target); got != 4000 {
		t.Fatalf("balanced bitrate = %d kbps, want the 4000 cap", got)
	}
	if got := bitrateKbps(encodeProfileForMode(small, ProfileDataSaver).Bitrate.Target); got != 1500 {
		t.Fatalf("data saver bitrate = %d kbps, want the 1500 cap", got)
	}
}

// Bitrate caps are ceilings, not targets: -b:v is a target average, so a source
// already below the cap must keep its own bitrate instead of being inflated.
func TestEncodeProfileForModeFloorsBitrateAtSource(t *testing.T) {
	withTranscodeProfiles(t)
	small := ffmpeg.StreamInfo{Height: 720, VideoBitrate: 800_000}
	for _, profile := range []Profile{ProfileQuality, ProfileBalanced, ProfileDataSaver} {
		if kbps := bitrateKbps(encodeProfileForMode(small, profile).Bitrate.Target); kbps != 800 {
			t.Fatalf("%s bitrate for an 800 kbps source = %d kbps, want 800", profile, kbps)
		}
	}
}

func TestEncodeProfileForModeKeepsSourceBitrate(t *testing.T) {
	withTranscodeProfiles(t)
	// 720p H.264 at ~2500 kbps sits below the balanced cap, so the source
	// bitrate wins.
	src := ffmpeg.StreamInfo{Height: 720, VideoBitrate: 2_500_000}
	got := encodeProfileForMode(src, ProfileBalanced)
	if kbps := bitrateKbps(got.Bitrate.Target); kbps != 2500 {
		t.Fatalf("bitrate = %d kbps, want the source 2500 kbps", kbps)
	}
}

func TestProfileForcesVideoTranscode(t *testing.T) {
	if profileForcesVideoTranscode(ProfileQuality) {
		t.Fatal("quality must keep the source when stream copy is possible")
	}
	for _, profile := range []Profile{ProfileBalanced, ProfileDataSaver} {
		if !profileForcesVideoTranscode(profile) {
			t.Fatalf("%s must always transcode video", profile)
		}
	}
}

func TestFMP4CopyableVideoCodec(t *testing.T) {
	for _, codec := range []string{"h264", "avc1", "hevc", "h265", "hvc1", "hev1", "vp9", "vp09.00.40.08"} {
		if !fmp4CopyableVideoCodec(codec) {
			t.Fatalf("%s should be fMP4-copyable", codec)
		}
	}
	for _, codec := range []string{"vp8", "av1", "wmv3", "mpeg4", "theora"} {
		if fmp4CopyableVideoCodec(codec) {
			t.Fatalf("%s has no usable fMP4 mapping and must transcode", codec)
		}
	}
}

func TestFMP4CopyEligibility(t *testing.T) {
	tests := []struct {
		name          string
		info          ffmpeg.StreamInfo
		mustTranscode bool
		wantRemux     bool
		wantVideoCopy bool
	}{
		{"hevc+aac remuxes", ffmpeg.StreamInfo{HasVideo: true, VideoCodec: "hevc", AudioCodec: "aac"}, false, true, false},
		{"vp9+aac remuxes", ffmpeg.StreamInfo{HasVideo: true, VideoCodec: "vp9", AudioCodec: "aac"}, false, true, false},
		{"vp9+opus copies video only", ffmpeg.StreamInfo{HasVideo: true, VideoCodec: "vp9", AudioCodec: "opus"}, false, false, true},
		{"hevc+ac3 copies video only", ffmpeg.StreamInfo{HasVideo: true, VideoCodec: "hevc", AudioCodec: "ac3"}, false, false, true},
		{"hevc without audio remuxes", ffmpeg.StreamInfo{HasVideo: true, VideoCodec: "hevc"}, false, true, false},
		{"vp8 never copies", ffmpeg.StreamInfo{HasVideo: true, VideoCodec: "vp8", AudioCodec: "aac"}, false, false, false},
		{"av1 never copies", ffmpeg.StreamInfo{HasVideo: true, VideoCodec: "av1", AudioCodec: "aac"}, false, false, false},
		{"forced transcode copies nothing", ffmpeg.StreamInfo{HasVideo: true, VideoCodec: "vp9", AudioCodec: "aac"}, true, false, false},
		{"no video copies nothing", ffmpeg.StreamInfo{AudioCodec: "aac"}, false, false, false},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			remux, videoCopy := fmp4CopyEligibility(tc.info, tc.mustTranscode)
			if remux != tc.wantRemux || videoCopy != tc.wantVideoCopy {
				t.Fatalf("got (remux=%v, videoCopy=%v), want (%v, %v)", remux, videoCopy, tc.wantRemux, tc.wantVideoCopy)
			}
		})
	}
}

func TestProfileRequiresBitrateTranscode(t *testing.T) {
	withTranscodeProfiles(t)

	// Data saver pins 1500 kbps: a higher-bitrate or unknown-bitrate source must
	// be re-encoded, since stream copy ignores the cap.
	high := ffmpeg.StreamInfo{Height: 720, VideoBitrate: 4_000_000}
	if !profileRequiresBitrateTranscode(ProfileDataSaver, high) {
		t.Fatal("a source above the cap must force a transcode")
	}
	unknown := ffmpeg.StreamInfo{Height: 480, VideoBitrate: 0}
	if !profileRequiresBitrateTranscode(ProfileDataSaver, unknown) {
		t.Fatal("an unknown source bitrate must force a transcode to honor the cap")
	}
	low := ffmpeg.StreamInfo{Height: 720, VideoBitrate: 1_000_000}
	if profileRequiresBitrateTranscode(ProfileDataSaver, low) {
		t.Fatal("a source below the cap does not need a re-encode")
	}
	// Quality pins no bitrate, so it never forces a transcode for the bitrate's sake.
	if profileRequiresBitrateTranscode(ProfileQuality, high) {
		t.Fatal("quality has no bitrate cap and must not force a transcode")
	}
}

func bitrateKbps(target string) int {
	trimmed := strings.TrimSuffix(target, "k")
	n, err := strconv.Atoi(trimmed)
	if err != nil {
		return -1
	}
	return n
}

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
