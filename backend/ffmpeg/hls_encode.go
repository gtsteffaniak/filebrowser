package ffmpeg

import (
	"strings"

	goffmpeg "github.com/gtsteffaniak/go-ffmpeg"
	"github.com/gtsteffaniak/go-ffmpeg/encode"
)

// HLSTranscodeProfile names browser transcode quality presets (filebrowser API).
type HLSTranscodeProfile string

const (
	HLSProfileQuality   HLSTranscodeProfile = "quality"
	HLSProfileOptimized HLSTranscodeProfile = "optimized"
	HLSProfileDataSaver HLSTranscodeProfile = "datasaver"
)

// ParseHLSTranscodeProfile normalizes profile query values.
func ParseHLSTranscodeProfile(raw string) HLSTranscodeProfile {
	switch strings.ToLower(strings.TrimSpace(raw)) {
	case string(HLSProfileOptimized):
		return HLSProfileOptimized
	case string(HLSProfileDataSaver), "data-saver", "data_saver":
		return HLSProfileDataSaver
	default:
		return HLSProfileQuality
	}
}

func HLSDecodeProfile(info StreamInfo) encode.VideoDecodeProfile {
	return goffmpeg.HLSDecodeProfileForOnDemand(info)
}

// HLSEncodeProfile selects output encode settings for HLS transcode.
func HLSEncodeProfile(info StreamInfo, mode HLSTranscodeProfile, maxHeight int) encode.VideoProfile {
	return goffmpeg.HLSVideoProfile(info, hlsPreset(mode), maxHeight)
}
