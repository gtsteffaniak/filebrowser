package ffmpeg

import "testing"

func TestHLSTranscodeDecisionHelpers(t *testing.T) {
	t.Parallel()

	h264AAC := StreamInfo{HasVideo: true, VideoCodec: "h264", AudioCodec: "aac", Height: 1080}
	h264EAC3 := StreamInfo{HasVideo: true, VideoCodec: "h264", AudioCodec: "eac3", Height: 1080}
	hevcAAC := StreamInfo{HasVideo: true, VideoCodec: "hevc", AudioCodec: "aac", Height: 1080}
	tallH264 := StreamInfo{HasVideo: true, VideoCodec: "h264", AudioCodec: "aac", Height: 2160}

	tests := []struct {
		name           string
		info           StreamInfo
		mode           HLSTranscodeProfile
		maxHeight      int
		wantFMP4Remux  bool
		wantRemux      bool
		wantVideoCopy  bool
		wantFullEncode bool
	}{
		{
			name:           "h264+aac quality remuxes",
			info:           h264AAC,
			mode:           HLSProfileQuality,
			wantFMP4Remux:  true,
			wantRemux:      true,
			wantVideoCopy:  false,
			wantFullEncode: false,
		},
		{
			name:           "h264+eac3 quality copies video",
			info:           h264EAC3,
			mode:           HLSProfileQuality,
			wantFMP4Remux:  false,
			wantRemux:      false,
			wantVideoCopy:  true,
			wantFullEncode: false,
		},
		{
			name:           "h264+eac3 optimized transcodes",
			info:           h264EAC3,
			mode:           HLSProfileOptimized,
			wantFMP4Remux:  false,
			wantRemux:      false,
			wantVideoCopy:  false,
			wantFullEncode: true,
		},
		{
			name:           "h264+eac3 datasaver transcodes",
			info:           h264EAC3,
			mode:           HLSProfileDataSaver,
			wantFMP4Remux:  false,
			wantRemux:      false,
			wantVideoCopy:  false,
			wantFullEncode: true,
		},
		{
			name:           "hevc quality transcodes",
			info:           hevcAAC,
			mode:           HLSProfileQuality,
			wantFMP4Remux:  false,
			wantRemux:      false,
			wantVideoCopy:  false,
			wantFullEncode: true,
		},
		{
			name:           "tall h264 downscales",
			info:           tallH264,
			mode:           HLSProfileQuality,
			maxHeight:      1080,
			wantFMP4Remux:  true,
			wantRemux:      false,
			wantVideoCopy:  false,
			wantFullEncode: true,
		},
	}

	for _, tc := range tests {
		tc := tc
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()
			if got := CanFMP4StreamCopy(tc.info); got != tc.wantFMP4Remux {
				t.Fatalf("CanFMP4StreamCopy() = %v, want %v", got, tc.wantFMP4Remux)
			}
			if got := HLSUseVideoCopy(tc.info, tc.mode, tc.maxHeight); got != tc.wantVideoCopy {
				t.Fatalf("HLSUseVideoCopy() = %v, want %v", got, tc.wantVideoCopy)
			}
			if got := HLSNeedsFullVideoTranscode(tc.info, tc.mode, tc.maxHeight); got != tc.wantFullEncode {
				t.Fatalf("HLSNeedsFullVideoTranscode() = %v, want %v", got, tc.wantFullEncode)
			}

			in := BuildHLSSegmentBuildInput(tc.info, tc.mode, tc.maxHeight)
			if in.Remux != tc.wantRemux {
				t.Fatalf("BuildHLSSegmentBuildInput().Remux = %v, want %v", in.Remux, tc.wantRemux)
			}
			if in.VideoCopy != tc.wantVideoCopy {
				t.Fatalf("BuildHLSSegmentBuildInput().VideoCopy = %v, want %v", in.VideoCopy, tc.wantVideoCopy)
			}
		})
	}
}

func TestCanH264VideoCopy(t *testing.T) {
	t.Parallel()
	h264EAC3 := StreamInfo{HasVideo: true, VideoCodec: "h264", AudioCodec: "eac3"}
	h264AAC := StreamInfo{HasVideo: true, VideoCodec: "h264", AudioCodec: "aac"}
	if !CanH264VideoCopy(h264EAC3) {
		t.Fatal("h264+eac3 should allow H.264 video copy")
	}
	if CanH264VideoCopy(h264AAC) {
		t.Fatal("h264+aac should not use H.264 video copy path")
	}
}
