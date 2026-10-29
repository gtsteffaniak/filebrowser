package preview

import (
	"github.com/gtsteffaniak/go-ffmpeg/ops"
)

// ffmpegMJPEGPreviewQuality matches historical preview encode (-q:v 10 on the legacy 1–10 scale).
const ffmpegMJPEGPreviewQuality = 72

type ffmpegPreviewParams struct {
	width     int
	height    int
	scaleMode ops.ScaleMode
	quality   int
}

func ffmpegPreviewParamsForSize(previewSize string) (ffmpegPreviewParams, error) {
	if previewSize == "original" {
		return ffmpegPreviewParams{quality: ffmpegMJPEGPreviewQuality}, nil
	}

	opts, err := getPreviewOptions(previewSize)
	if err != nil {
		return ffmpegPreviewParams{}, err
	}

	// Video thumbnails always fit within the box (aspect ratio kept). No crop, on
	// either the go-media or the ffmpeg path, so the ResizeMode fill setting used
	// by still-image previews does not apply here.
	params := ffmpegPreviewParams{
		width:     opts.Width,
		height:    opts.Height,
		scaleMode: ops.ScaleFit,
		quality:   ffmpegMJPEGPreviewQuality, // ~legacy -q:v 10; visual quality comes from target dimensions
	}
	return params, nil
}
