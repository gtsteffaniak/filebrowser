package preview

import (
	"github.com/gtsteffaniak/go-ffmpeg/ops"
)

type ffmpegPreviewParams struct {
	width     int
	height    int
	scaleMode ops.ScaleMode
	quality   int
}

func ffmpegPreviewParamsForSize(previewSize string) (ffmpegPreviewParams, error) {
	if previewSize == "original" {
		return ffmpegPreviewParams{quality: ops.LegacyPreviewDefaultQuality}, nil
	}

	opts, err := getPreviewOptions(previewSize)
	if err != nil {
		return ffmpegPreviewParams{}, err
	}

	params := ffmpegPreviewParams{
		width:   opts.Width,
		height:  opts.Height,
		quality: ops.LegacyPreviewDefaultQuality, // 1–100 scale; equivalent to historic MJPEG -q:v 10
	}
	if opts.ResizeMode == ResizeModeFill {
		params.scaleMode = ops.ScaleFill
	} else {
		params.scaleMode = ops.ScaleFit
	}
	return params, nil
}
