package ffmpeg

import (
	"context"
	"fmt"
	"io"
	"os"
	"strings"

	goffmpeg "github.com/gtsteffaniak/go-ffmpeg"
	"github.com/gtsteffaniak/go-ffmpeg/capabilities"
	"github.com/gtsteffaniak/go-ffmpeg/ops"
	"github.com/gtsteffaniak/go-logger/logger"
)

// Service wraps go-ffmpeg for filebrowser media operations.
type Service struct {
	inner        *goffmpeg.Service
	cacheDir     string
	exiftoolPath string
}

// FFmpegService is kept for existing callers.
type FFmpegService = Service

var global *Service

// InitOptions configures startup initialization.
type InitOptions struct {
	FFmpegPath           string
	MaxConcurrent        int
	CacheDir             string
	SkipHWTests          bool
	HardwareAcceleration bool
	ExiftoolPath         string
}

// Initialize creates the global ffmpeg service and runs capability detection.
func Initialize(ctx context.Context, opts InitOptions) error {
	if opts.MaxConcurrent < 1 {
		opts.MaxConcurrent = 4
	}
	if opts.CacheDir == "" {
		opts.CacheDir = os.TempDir()
	}

	if opts.HardwareAcceleration {
		logger.Info("Detecting ffmpeg hardware codec support...")
	}

	svc, err := goffmpeg.New(ctx, goffmpeg.Config{
		FFmpegPath:    opts.FFmpegPath,
		MaxConcurrent: opts.MaxConcurrent,
		Logger:        goffmpeg.NopLogger(),
		SkipHWTests:   opts.SkipHWTests,
	})
	if err != nil {
		global = nil
		return err
	}

	global = &Service{
		inner:        svc,
		cacheDir:     opts.CacheDir,
		exiftoolPath: opts.ExiftoolPath,
	}

	logCapabilities(svc, opts.HardwareAcceleration)
	return nil
}

// Get returns the initialized service, or nil when ffmpeg is unavailable.
func Get() *Service {
	return global
}

// Enabled reports whether ffmpeg initialized successfully.
func Enabled() bool {
	return global != nil && global.inner != nil
}

// FFmpegPath returns the resolved ffmpeg binary path.
func (s *Service) FFmpegPath() string {
	if s == nil || s.inner == nil {
		return ""
	}
	return s.inner.FFmpegPath()
}

// FFprobePath returns the resolved ffprobe binary path.
func (s *Service) FFprobePath() string {
	if s == nil || s.inner == nil {
		return ""
	}
	return s.inner.FFprobePath()
}

func (s *Service) Acquire(ctx context.Context) error {
	if s == nil || s.inner == nil {
		return fmt.Errorf("ffmpeg service not available")
	}
	return s.inner.Acquire(ctx)
}

func (s *Service) Release() {
	if s == nil || s.inner == nil {
		return
	}
	s.inner.Release()
}

// VideoPreview extracts a JPEG preview frame to w.
func (s *Service) VideoPreview(ctx context.Context, w io.Writer, videoPath string, percentageSeek int) error {
	if s == nil || s.inner == nil {
		return fmt.Errorf("ffmpeg service not available")
	}
	if err := s.Acquire(ctx); err != nil {
		return err
	}
	defer s.Release()

	return s.inner.VideoPreview(ctx, w, ops.PreviewOptions{
		Input:       videoPath,
		SeekPercent: float64(percentageSeek),
		Quality:     10,
	})
}

// ExtractSubtitle returns embedded subtitle content as WebVTT.
func (s *Service) ExtractSubtitle(ctx context.Context, videoPath string, streamIndex int) (string, error) {
	if s == nil || s.inner == nil {
		return "", fmt.Errorf("ffmpeg service not available")
	}

	fileInfo, err := os.Stat(videoPath)
	if err != nil {
		return "", fmt.Errorf("failed to stat file: %w", err)
	}
	cacheKey := fmt.Sprintf("subtitle_content:%s:%d:%d", videoPath, streamIndex, fileInfo.ModTime().Unix())
	if content, ok := SubtitleContentCache.Get(cacheKey); ok {
		return content, nil
	}

	if err = s.Acquire(ctx); err != nil {
		return "", err
	}
	defer s.Release()

	content, err := s.inner.ExtractSubtitle(ctx, videoPath, streamIndex)
	if err != nil {
		return "", err
	}

	SubtitleContentCache.Set(cacheKey, content)
	return content, nil
}

func logCapabilities(svc *goffmpeg.Service, detectHardware bool) {
	caps := svc.Capabilities()
	if caps == nil {
		return
	}

	logger.Infof("ffmpeg enabled: version %s @ %s", caps.FFmpegVersion, caps.FFmpegPath)

	if !detectHardware {
		return
	}

	hw := hardwareCodecSummary(svc)
	if hw == "" {
		logger.Warning("no ffmpeg hardware codec support found")
		return
	}
	logger.Infof("supported ffmpeg hardware codecs: %s", hw)
}

func hardwareCodecSummary(svc *goffmpeg.Service) string {
	seen := make(map[string]struct{})
	var parts []string

	add := func(entry string) {
		if entry == "" {
			return
		}
		if _, ok := seen[entry]; ok {
			return
		}
		seen[entry] = struct{}{}
		parts = append(parts, entry)
	}

	for _, opt := range svc.AvailableEncodeOptions() {
		if opt.Accel == capabilities.AccelNone {
			continue
		}
		add(fmt.Sprintf("%s encode via %s (%s)", opt.Codec, opt.Encoder, capabilities.AccelLabel(opt.Accel)))
	}
	for _, opt := range svc.AvailableDecodeOptions() {
		if opt.Accel == capabilities.AccelNone {
			continue
		}
		add(fmt.Sprintf("%s decode via %s (%s)", opt.Codec, opt.Decoder, capabilities.AccelLabel(opt.Accel)))
	}

	return strings.Join(parts, ", ")
}
