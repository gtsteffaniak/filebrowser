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
		return ffmpegPreviewParams{quality: 10}, nil
	}

	opts, err := getPreviewOptions(previewSize)
	if err != nil {
		return ffmpegPreviewParams{}, err
	}

	params := ffmpegPreviewParams{
		width:   opts.Width,
		height:  opts.Height,
		quality: 10, // fastest MJPEG encode (-q:v 10); visual quality comes from target dimensions
	}
	if opts.ResizeMode == ResizeModeFill {
		params.scaleMode = ops.ScaleFill
	} else {
		params.scaleMode = ops.ScaleFit
	}
	return params, nil
}
