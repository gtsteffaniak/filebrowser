package http

import (
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/gtsteffaniak/filebrowser/backend/common/settings"
	"github.com/gtsteffaniak/filebrowser/backend/database/users"
	"github.com/gtsteffaniak/filebrowser/backend/ffmpeg"
	"github.com/gtsteffaniak/go-ffmpeg/capabilities"
	"github.com/gtsteffaniak/go-ffmpeg/encode"
)

func testTranscodeUser(id uint64) *users.User {
	return &users.User{
		ID: id,
	}
}

func TestTranscodeRejectRange(t *testing.T) {
	t.Parallel()
	req := httptest.NewRequest(http.MethodGet, "/api/media/transcode", nil)
	req.Header.Set("Range", "bytes=0-")
	if !transcodeRejectRange(req) {
		t.Fatal("expected range request to be rejected")
	}
	req.Header.Del("Range")
	if transcodeRejectRange(req) {
		t.Fatal("expected non-range request to be allowed")
	}
}

func TestCanFMP4StreamCopy(t *testing.T) {
	t.Parallel()
	tests := []struct {
		name string
		info ffmpeg.StreamInfo
		want bool
	}{
		{
			name: "h264 aac",
			info: ffmpeg.StreamInfo{HasVideo: true, VideoCodec: "h264", AudioCodec: "aac"},
			want: true,
		},
		{
			name: "h264 no audio",
			info: ffmpeg.StreamInfo{HasVideo: true, VideoCodec: "h264"},
			want: true,
		},
		{
			name: "hevc",
			info: ffmpeg.StreamInfo{HasVideo: true, VideoCodec: "hevc", AudioCodec: "aac"},
			want: false,
		},
		{
			name: "h264 mp3",
			info: ffmpeg.StreamInfo{HasVideo: true, VideoCodec: "h264", AudioCodec: "mp3"},
			want: false,
		},
		{
			name: "h264 eac3",
			info: ffmpeg.StreamInfo{HasVideo: true, VideoCodec: "h264", AudioCodec: "eac3"},
			want: false,
		},
	}
	for _, tc := range tests {
		tc := tc
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()
			if got := canFMP4StreamCopy(tc.info); got != tc.want {
				t.Fatalf("canFMP4StreamCopy() = %v, want %v", got, tc.want)
			}
		})
	}
}

func TestCanH264VideoCopy(t *testing.T) {
	t.Parallel()
	tests := []struct {
		name string
		info ffmpeg.StreamInfo
		want bool
	}{
		{
			name: "h264 aac",
			info: ffmpeg.StreamInfo{HasVideo: true, VideoCodec: "h264", AudioCodec: "aac"},
			want: false,
		},
		{
			name: "h264 eac3",
			info: ffmpeg.StreamInfo{HasVideo: true, VideoCodec: "h264", AudioCodec: "eac3"},
			want: true,
		},
		{
			name: "h264 no audio",
			info: ffmpeg.StreamInfo{HasVideo: true, VideoCodec: "h264"},
			want: false,
		},
		{
			name: "hevc eac3",
			info: ffmpeg.StreamInfo{HasVideo: true, VideoCodec: "hevc", AudioCodec: "eac3"},
			want: false,
		},
	}
	for _, tc := range tests {
		tc := tc
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()
			if got := canH264VideoCopy(tc.info); got != tc.want {
				t.Fatalf("canH264VideoCopy() = %v, want %v", got, tc.want)
			}
		})
	}
}

func TestHLSUseVideoCopy(t *testing.T) {
	t.Parallel()
	eac3 := ffmpeg.StreamInfo{HasVideo: true, VideoCodec: "h264", AudioCodec: "eac3", Height: 1080}
	if !hlsUseVideoCopy(eac3, transcodeProfileQuality) {
		t.Fatal("quality mode should use video copy for h264+eac3")
	}
	if hlsUseVideoCopy(eac3, transcodeProfileOptimized) {
		t.Fatal("optimized mode should not use video copy")
	}
	if hlsUseVideoCopy(eac3, transcodeProfileDataSaver) {
		t.Fatal("datasaver mode should not use video copy")
	}
	if hlsNeedsFullVideoTranscode(eac3, transcodeProfileOptimized) != true {
		t.Fatal("optimized should require full transcode")
	}
	if hlsNeedsFullVideoTranscode(eac3, transcodeProfileDataSaver) != true {
		t.Fatal("datasaver should require full transcode")
	}
}

func TestTranscodeTargetVideoKbps(t *testing.T) {
	oldMax := settings.Config.Integrations.Media.Transcode.MaxResolution
	settings.Config.Integrations.Media.Transcode.MaxResolution = 1080
	t.Cleanup(func() {
		settings.Config.Integrations.Media.Transcode.MaxResolution = oldMax
	})

	tests := []struct {
		name string
		info ffmpeg.StreamInfo
		want int
	}{
		{
			name: "1080p baseline",
			info: ffmpeg.StreamInfo{Height: 1080},
			want: 5000,
		},
		{
			name: "uses probed source bitrate",
			info: ffmpeg.StreamInfo{Height: 1080, VideoBitrate: 8_000_000},
			want: 8000,
		},
		{
			name: "downscale keeps resolution baseline floor",
			info: ffmpeg.StreamInfo{Height: 2160, VideoBitrate: 16_000_000},
			want: 5000,
		},
	}
	for _, tc := range tests {
		tc := tc
		t.Run(tc.name, func(t *testing.T) {
			if got := transcodeTargetVideoKbps(tc.info); got != tc.want {
				t.Fatalf("transcodeTargetVideoKbps() = %d, want %d", got, tc.want)
			}
		})
	}
}

func TestTranscodeDecodeProfileUnknownCodec(t *testing.T) {
	t.Parallel()
	profile := transcodeDecodeProfile(ffmpeg.StreamInfo{VideoCodec: "wmv3"})
	if !profile.ForceSoftware {
		t.Fatal("expected ForceSoftware for wmv3")
	}
	profile = transcodeDecodeProfile(ffmpeg.StreamInfo{VideoCodec: "h264"})
	if profile.ForceSoftware || profile.Codec != capabilities.CodecH264 {
		t.Fatalf("expected h264 decode profile, got %+v", profile)
	}
}

func TestTranscodeEncodeProfileForMode(t *testing.T) {
	t.Parallel()
	info := ffmpeg.StreamInfo{Height: 1080, VideoBitrate: 8_000_000}

	quality := transcodeEncodeProfileForMode(info, "quality")
	if quality.Quality != encode.PresetMedium {
		t.Fatalf("quality preset = %q, want medium", quality.Quality)
	}
	if quality.Bitrate.Max == quality.Bitrate.Target {
		t.Fatal("expected quality mode to use variable max bitrate")
	}

	optimized := transcodeEncodeProfileForMode(info, "optimized")
	if optimized.Quality != encode.PresetVeryfast {
		t.Fatalf("optimized preset = %q, want veryfast", optimized.Quality)
	}
	if optimized.Bitrate.Max != optimized.Bitrate.Target {
		t.Fatal("expected optimized mode to hard-cap max bitrate")
	}

	datasaver := transcodeEncodeProfileForMode(info, "datasaver")
	if datasaver.Quality != encode.PresetVeryfast {
		t.Fatalf("datasaver preset = %q, want veryfast", datasaver.Quality)
	}
	if datasaver.Bitrate.Max != datasaver.Bitrate.Target {
		t.Fatal("expected datasaver mode to hard-cap max bitrate")
	}
	if datasaver.Bitrate.Target != "1500k" {
		t.Fatalf("datasaver target = %q, want 1500k", datasaver.Bitrate.Target)
	}
}

func TestParseTranscodeProfileMode(t *testing.T) {
	t.Parallel()
	if got := parseTranscodeProfileMode("optimized"); got != transcodeProfileOptimized {
		t.Fatalf("got %q", got)
	}
	if got := parseTranscodeProfileMode("datasaver"); got != transcodeProfileDataSaver {
		t.Fatalf("datasaver got %q", got)
	}
	if got := parseTranscodeProfileMode("data-saver"); got != transcodeProfileDataSaver {
		t.Fatalf("data-saver got %q", got)
	}
	if got := parseTranscodeProfileMode(""); got != transcodeProfileQuality {
		t.Fatalf("default got %q", got)
	}
}

func TestTranscodeMaxHeightForMode(t *testing.T) {
	oldMax := settings.Config.Integrations.Media.Transcode.MaxResolution
	settings.Config.Integrations.Media.Transcode.MaxResolution = 1080
	t.Cleanup(func() {
		settings.Config.Integrations.Media.Transcode.MaxResolution = oldMax
	})

	if got := transcodeMaxHeightForMode(transcodeProfileQuality); got != 1080 {
		t.Fatalf("quality maxH = %d, want 1080", got)
	}
	if got := transcodeMaxHeightForMode(transcodeProfileDataSaver); got != 720 {
		t.Fatalf("datasaver maxH = %d, want 720", got)
	}

	settings.Config.Integrations.Media.Transcode.MaxResolution = 480
	if got := transcodeMaxHeightForMode(transcodeProfileDataSaver); got != 480 {
		t.Fatalf("datasaver respects lower global maxH = %d, want 480", got)
	}
}

func TestTranscodeHandlerRejectsMissingToken(t *testing.T) {
	t.Parallel()
	d := &requestContext{user: testTranscodeUser(1)}
	req := httptest.NewRequest(http.MethodGet, "/api/media/transcode?source=default&file=/a.mkv", nil)
	status, err := transcodeHandler(httptest.NewRecorder(), req, d)
	if status != http.StatusForbidden || err == nil {
		t.Fatalf("expected 403, got status=%d err=%v", status, err)
	}
}

func TestTranscodeHandlerRejectsRange(t *testing.T) {
	t.Parallel()
	d := &requestContext{user: testTranscodeUser(1)}
	token, err := mintStreamGrant(d, "default", "/a.mkv", 0, nil)
	if err != nil {
		t.Fatal(err)
	}
	req := httptest.NewRequest(http.MethodGet, "/api/media/transcode?source=default&file=/a.mkv&streamToken="+token, nil)
	req.Header.Set("Range", "bytes=0-")
	status, err := transcodeHandler(httptest.NewRecorder(), req, d)
	if status != http.StatusRequestedRangeNotSatisfiable || err == nil {
		t.Fatalf("expected 416, got status=%d err=%v", status, err)
	}
}

func TestTranscodeHandlerRejectsMultiFile(t *testing.T) {
	t.Parallel()
	d := &requestContext{user: testTranscodeUser(1)}
	req := httptest.NewRequest(http.MethodGet, "/api/media/transcode?source=default&file=/a.mkv&file=/b.mkv&streamToken=tok", nil)
	status, err := transcodeHandler(httptest.NewRecorder(), req, d)
	if status != http.StatusForbidden || err == nil {
		t.Fatalf("expected 403 for multi-file, got status=%d err=%v", status, err)
	}
}
