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
		return ffmpegPreviewParams{quality: 1}, nil
	}

	opts, err := getPreviewOptions(previewSize)
	if err != nil {
		return ffmpegPreviewParams{}, err
	}

	params := ffmpegPreviewParams{
		width:   opts.Width,
		height:  opts.Height,
		quality: ffmpegQualityFromPreview(opts.Quality),
	}
	if opts.ResizeMode == ResizeModeFill {
		params.scaleMode = ops.ScaleFill
	} else {
		params.scaleMode = ops.ScaleFit
	}
	return params, nil
}

func ffmpegQualityFromPreview(q Quality) int {
	switch q {
	case QualityHigh, QualityMedium:
		return 2
	case QualityLow:
		return 5
	default:
		return 5
	}
}
