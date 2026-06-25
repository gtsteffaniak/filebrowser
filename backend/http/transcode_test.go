package http

import (
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/gtsteffaniak/filebrowser/backend/common/settings"
	"github.com/gtsteffaniak/filebrowser/backend/database/users"
	"github.com/gtsteffaniak/filebrowser/backend/ffmpeg"
	"github.com/gtsteffaniak/go-ffmpeg/capabilities"
)

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

func TestTranscodeTargetVideoKbps(t *testing.T) {
	t.Parallel()
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
	settings.Config.Integrations.Media.Transcode.MaxResolution = 1080
	for _, tc := range tests {
		tc := tc
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()
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

func TestTranscodeHandlerRejectsMissingToken(t *testing.T) {
	t.Parallel()
	d := &requestContext{user: &users.User{ID: 1}}
	req := httptest.NewRequest(http.MethodGet, "/api/media/transcode?source=default&file=/a.mkv", nil)
	status, err := transcodeHandler(httptest.NewRecorder(), req, d)
	if status != http.StatusForbidden || err == nil {
		t.Fatalf("expected 403, got status=%d err=%v", status, err)
	}
}

func TestTranscodeHandlerRejectsRange(t *testing.T) {
	t.Parallel()
	d := &requestContext{user: &users.User{ID: 1}}
	token, err := mintStreamGrant(d, "default", "/a.mkv")
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
	d := &requestContext{user: &users.User{ID: 1}}
	req := httptest.NewRequest(http.MethodGet, "/api/media/transcode?source=default&file=/a.mkv&file=/b.mkv&streamToken=tok", nil)
	status, err := transcodeHandler(httptest.NewRecorder(), req, d)
	if status != http.StatusForbidden || err == nil {
		t.Fatalf("expected 403 for multi-file, got status=%d err=%v", status, err)
	}
}
