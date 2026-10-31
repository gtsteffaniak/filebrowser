package preview

import (
	"bytes"
	"context"
	"fmt"
	"os"

	"github.com/gtsteffaniak/filebrowser/backend/internal/gomedia"
	"github.com/gtsteffaniak/go-ffmpeg/ops"
	"github.com/gtsteffaniak/go-logger/logger"
)

// GenerateVideoPreview generates a single preview image from a video using ffmpeg.
// videoPath: path to the input video file.
// previewSize: small, large, xlarge, or original.
// percentageSeek: percentage of video duration to seek to (0–100).
// Returns: JPEG image bytes.
func (s *Service) GenerateVideoPreview(ctx context.Context, videoPath, previewSize string, percentageSeek int) ([]byte, error) {
	// Validate file exists before processing
	if _, err := os.Stat(videoPath); err != nil {
		logger.Errorf("Video file does not exist or is not accessible: '%s': %v", videoPath, err)
		return nil, fmt.Errorf("video file not accessible: %w", err)
	}

	params, err := ffmpegPreviewParamsForSize(previewSize)
	if err != nil {
		return nil, err
	}

	// go-media fast path: fit within the box, no crop. Falls back to ffmpeg when
	// unsupported (e.g. HEVC, VP9, AV1) or failed.
	// In shadow mode (dev only) the ffmpeg preview also runs, for comparison.
	// The shadow run can outlive the request, so it uses a context that is not cancelled with it.
	shadowCtx := context.WithoutCancel(ctx)
	legacy := func() ([]byte, bool) {
		if s.ffmpegService == nil {
			return nil, false
		}
		var buf bytes.Buffer
		previewErr := s.ffmpegService.VideoPreview(shadowCtx, &buf, ops.PreviewOptions{
			Input:       videoPath,
			SeekPercent: float64(percentageSeek),
			// Exact seek: the default fast seek returns the keyframe before the
			// target, which is black for many videos (e.g. Big Bunny at 10%).
			Accuracy:  ops.CaptureExact,
			Width:     params.width,
			Height:    params.height,
			ScaleMode: params.scaleMode,
			Quality:   params.quality,
		})
		return buf.Bytes(), previewErr == nil
	}
	if data, ok := gomedia.VideoThumbnail(videoPath, params.width, params.height, params.quality, float64(percentageSeek), legacy); ok {
		return data, nil
	}

	if s.ffmpegService == nil {
		logger.Errorf("FFmpeg service not available for file '%s'", videoPath)
		return nil, fmt.Errorf("FFmpeg service not available")
	}

	var buf bytes.Buffer
	err = s.ffmpegService.VideoPreview(ctx, &buf, ops.PreviewOptions{
		Input:       videoPath,
		SeekPercent: float64(percentageSeek),
		// Exact seek: the default fast seek returns the keyframe before the
		// target, which is black for many videos (e.g. Big Bunny at 10%).
		Accuracy:  ops.CaptureExact,
		Width:     params.width,
		Height:    params.height,
		ScaleMode: params.scaleMode,
		Quality:   params.quality,
	})
	if err != nil {
		return nil, err
	}

	previewBytes := buf.Bytes()
	if len(previewBytes) == 0 {
		logger.Errorf("FFmpeg service returned empty result for '%s'", videoPath)
		return nil, fmt.Errorf("video preview generation returned empty result")
	}

	return previewBytes, nil
}
