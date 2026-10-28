package transcoding

import (
	"strconv"
	"strings"
	"testing"

	"github.com/gtsteffaniak/filebrowser/backend/internal/ffmpeg"
	"github.com/gtsteffaniak/filebrowser/backend/pkg/settings"
	"github.com/gtsteffaniak/go-ffmpeg/capabilities"
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

func TestHLSPipelineSelection(t *testing.T) {
	mkvAC3 := ffmpeg.BuildHLSSegmentBuildInput(ffmpeg.StreamInfo{
		HasVideo: true, VideoCodec: "h264", AudioCodec: "ac3", FormatName: "matroska,webm",
	}, ffmpeg.HLSPipelineOptions{})
	if !mkvAC3.VideoCopy || mkvAC3.Remux {
		t.Fatalf("mkv h264+ac3 should video-copy with AAC audio transcode: remux=%v videoCopy=%v", mkvAC3.Remux, mkvAC3.VideoCopy)
	}
	mp4AC3 := ffmpeg.BuildHLSSegmentBuildInput(ffmpeg.StreamInfo{
		HasVideo: true, VideoCodec: "h264", AudioCodec: "ac3", FormatName: "mov,mp4,m4a,3gp,3g2",
	}, ffmpeg.HLSPipelineOptions{})
	if mp4AC3.Remux || !mp4AC3.VideoCopy {
		t.Fatalf("mp4 h264+ac3 = remux %v videoCopy %v", mp4AC3.Remux, mp4AC3.VideoCopy)
	}
	aac := ffmpeg.BuildHLSSegmentBuildInput(ffmpeg.StreamInfo{
		HasVideo: true, VideoCodec: "h264", AudioCodec: "aac", FormatName: "matroska,webm",
	}, ffmpeg.HLSPipelineOptions{})
	if !aac.Remux || aac.VideoCopy {
		t.Fatalf("h264+aac should remux: remux=%v videoCopy=%v", aac.Remux, aac.VideoCopy)
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

func TestVODSeekParams(t *testing.T) {
	t.Parallel()
	plan := hlsPlan{
		vodList: ffmpeg.BuildVODSegmentList(30.5, 4.0, 0, nil),
	}

	tests := []struct {
		name         string
		requested    float64
		wantIndex    int
		wantStartSec float64
	}{
		{name: "zero", requested: 0, wantIndex: 0, wantStartSec: 0},
		{name: "within first segment", requested: 3.44, wantIndex: 0, wantStartSec: 0},
		{name: "segment boundary", requested: 4, wantIndex: 1, wantStartSec: 4},
		{name: "mid segment", requested: 5.5, wantIndex: 1, wantStartSec: 4},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()
			idx, start := vodSeekParams(plan, tc.requested)
			if idx != tc.wantIndex || start != tc.wantStartSec {
				t.Fatalf("vodSeekParams(%v) = (%d, %v), want (%d, %v)",
					tc.requested, idx, start, tc.wantIndex, tc.wantStartSec)
			}
		})
	}
}

func TestTranscodePlanUsesFixedSegmentGrid(t *testing.T) {
	t.Parallel()
	const segDur = 4.0
	duration := 30.5

	// Sparse WMV-like keyframes that do not match the output grid.
	keyframeList := ffmpeg.BuildVODSegmentList(duration, segDur, 0, []float64{0, 3.909, 7.818, 11.727})
	fixedList := ffmpeg.BuildVODSegmentList(duration, segDur, 0, nil)

	if len(keyframeList.Durations) == len(fixedList.Durations) {
		same := true
		for i := range fixedList.Durations {
			if keyframeList.Durations[i] != fixedList.Durations[i] {
				same = false
				break
			}
		}
		if same {
			t.Fatal("expected keyframe-based durations to differ from fixed grid for sparse keyframes")
		}
	}

	lastFixed := fixedList.Durations[len(fixedList.Durations)-1]
	if lastFixed <= 0 || lastFixed > segDur {
		t.Fatalf("tail segment duration %v out of range", lastFixed)
	}
	for i, dur := range fixedList.Durations[:len(fixedList.Durations)-1] {
		if dur != segDur {
			t.Fatalf("fixed segment %d duration = %v, want %v", i, dur, segDur)
		}
	}
}

// Containers with a non-zero format start_time (e.g. some MKV/MP4 muxes) carry
// keyframe times on a shifted container timeline. Segment planning must
// rebase them onto the 0-based media timeline the HLS output covers.
func TestTranscodePlanNormalizesContainerStartTime(t *testing.T) {
	t.Parallel()
	const segDur = 4.0
	const startTime = 1.4

	// Same media as the zero-offset case: 30.5s of content whose format
	// duration includes the 1.4s pre-roll offset.
	containerDuration := 30.5 + startTime
	containerKeyframes := []float64{startTime, startTime + 3.909, startTime + 7.818, startTime + 11.727}

	sanitized, mediaDuration := ffmpeg.PrepareHLSMediaTimeline(containerKeyframes, containerDuration, startTime)
	if mediaDuration < 30.4 || mediaDuration > 30.6 {
		t.Fatalf("media duration = %v, want ~30.5", mediaDuration)
	}
	offsetList := ffmpeg.BuildVODSegmentList(mediaDuration, segDur, 0, sanitized)

	zeroOffsetList := ffmpeg.BuildVODSegmentList(30.5, segDur, 0, []float64{0, 3.909, 7.818, 11.727})

	if len(offsetList.Durations) != len(zeroOffsetList.Durations) {
		t.Fatalf("normalized timeline has %d segments, want %d", len(offsetList.Durations), len(zeroOffsetList.Durations))
	}
	for i := range zeroOffsetList.Durations {
		if offsetList.Durations[i] != zeroOffsetList.Durations[i] {
			t.Fatalf("segment %d duration = %v, want %v", i, offsetList.Durations[i], zeroOffsetList.Durations[i])
		}
	}

	// The lib also normalizes internally when startTimeSec is passed directly.
	libList := ffmpeg.BuildVODSegmentList(containerDuration, segDur, startTime, containerKeyframes)
	if len(libList.Durations) != len(zeroOffsetList.Durations) {
		t.Fatalf("lib-normalized timeline has %d segments, want %d", len(libList.Durations), len(zeroOffsetList.Durations))
	}
}

// The encoder backend preference resolves profile override, then the
// transcode-level default; hardware backends are capability-guarded so an
// unavailable one degrades to auto instead of erroring the job.
func TestVODMaxSegmentDuration(t *testing.T) {
	list := ffmpeg.VODSegmentList{Durations: []float64{4, 50.9, 984}}
	if got := vodMaxSegmentDuration(list); got != 984 {
		t.Fatalf("max duration = %v, want 984", got)
	}
	if maxCopySegmentDurationSec(4) != 12 {
		t.Fatalf("copy cap = %v, want 12", maxCopySegmentDurationSec(4))
	}
}

func TestPreferredEncoderAccel(t *testing.T) {
	preferredEncoderAccel := func(profile Profile) capabilities.AccelType {
		accel, _ := preferredEncoderSelection(profile)
		return accel
	}
	withTranscodeProfiles(t)
	origBackend := settings.Config.Integrations.Media.Transcode.EncoderBackend
	t.Cleanup(func() { settings.Config.Integrations.Media.Transcode.EncoderBackend = origBackend })

	origHW := settings.Config.Integrations.Media.HardwareAcceleration
	t.Cleanup(func() { settings.Config.Integrations.Media.HardwareAcceleration = origHW })
	settings.Config.Integrations.Media.HardwareAcceleration = true

	settings.Config.Integrations.Media.Transcode.EncoderBackend = ""
	if got := preferredEncoderAccel(ProfileBalanced); got != "" {
		t.Fatalf("auto backend = %q, want \"\"", got)
	}

	// Toggling hardware acceleration off takes effect immediately (no re-detect).
	settings.Config.Integrations.Media.HardwareAcceleration = false
	if got := preferredEncoderAccel(ProfileBalanced); got != capabilities.AccelNone {
		t.Fatalf("hardware off = %q, want software", got)
	}
	settings.Config.Integrations.Media.HardwareAcceleration = true

	settings.Config.Integrations.Media.Transcode.EncoderBackend = "cpu"
	if got := preferredEncoderAccel(ProfileBalanced); got != capabilities.AccelNone {
		t.Fatalf("cpu backend = %q, want %q", got, capabilities.AccelNone)
	}
	// A per-preset override beats the transcode-level default.
	profiles := settings.DefaultTranscodeProfiles()
	profiles.DataSaver.EncoderBackend = "cpu"
	settings.Config.Integrations.Media.Transcode.Profiles = profiles
	if got := preferredEncoderAccel(ProfileDataSaver); got != capabilities.AccelNone {
		t.Fatalf("preset-override backend = %q, want %q", got, capabilities.AccelNone)
	}

	// No ffmpeg service in tests -> no capabilities -> hardware backends must
	// degrade to auto rather than resolving (ResolveEncoder would error).
	settings.Config.Integrations.Media.Transcode.EncoderBackend = "nvenc"
	if got := preferredEncoderAccel(ProfileBalanced); got != "" {
		t.Fatalf("unavailable nvenc = %q, want auto \"\"", got)
	}
}

// Keyframes handed to buildTimeline are already normalized to the 0-based media
// timeline, so a non-zero container start_time must not shift them again.
func TestBuildTimelineDoesNotRenormalizeKeyframes(t *testing.T) {
	plan := hlsPlan{
		info:       ffmpeg.StreamInfo{Duration: 41.4, StartTime: 1.4},
		segmentSec: 4,
	}
	plan.buildTimeline([]float64{0, 4.1, 8.2, 12.3, 16.4, 20.5, 24.6, 28.7, 32.8, 36.9})
	want := []float64{0, 4.1, 8.2, 12.3, 16.4, 20.5, 24.6, 28.7, 32.8, 36.9}
	if len(plan.vodList.Starts) != len(want) {
		t.Fatalf("starts = %v, want %v", plan.vodList.Starts, want)
	}
	for i, s := range plan.vodList.Starts {
		if s < want[i]-0.001 || s > want[i]+0.001 {
			t.Fatalf("starts = %v, want %v", plan.vodList.Starts, want)
		}
	}
}

func TestCopyJobStopIndexWindowsTheJob(t *testing.T) {
	if got := copyJobStopIndex(14, 200); got != 14+copyLookaheadSegments {
		t.Fatalf("stop = %d, want %d", got, 14+copyLookaheadSegments)
	}
	if got := copyJobStopIndex(190, 200); got != 0 {
		t.Fatalf("a window reaching the end must be unbounded, got %d", got)
	}
}
