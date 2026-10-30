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

	params := ffmpegPreviewParams{
		width:   opts.Width,
		height:  opts.Height,
		quality: ffmpegMJPEGPreviewQuality, // ~legacy -q:v 10; visual quality comes from target dimensions
	}
	if opts.ResizeMode == ResizeModeFill {
		params.scaleMode = ops.ScaleFill
	} else {
		params.scaleMode = ops.ScaleFit
	}
	return params, nil
}
