package ffmpeg

import (
	"context"
	"fmt"
	"io"
	"os"
	"strings"
	"time"

	goffmpeg "github.com/gtsteffaniak/go-ffmpeg"
	"github.com/gtsteffaniak/go-ffmpeg/capabilities"
	gtlogger "github.com/gtsteffaniak/go-ffmpeg/gtlogger"
	"github.com/gtsteffaniak/go-ffmpeg/ops"
	"github.com/gtsteffaniak/go-logger/logger"
)

// Service wraps go-ffmpeg for filebrowser media operations.
// Each Service owns a single concurrency pool (go-ffmpeg MaxConcurrent).
type Service struct {
	inner        *goffmpeg.Service
	cacheDir     string
	exiftoolPath string
}

// FFmpegService is kept for existing callers.
type FFmpegService = Service

var (
	// global is the preview/media pool: thumbnails, subtitles, metadata, image conversion.
	global *Service
)

// InitOptions configures startup initialization.
type InitOptions struct {
	FFmpegPath    string
	MaxConcurrent int
	CacheDir      string
	GPU           string
	SkipHWTests   bool
	LogHardware   bool
	ExiftoolPath  string
	Debug         bool
}

// Initialize creates the global preview/media ffmpeg service and runs capability detection.
func Initialize(ctx context.Context, opts InitOptions) error {
	svc, err := newService(ctx, opts)
	if err != nil {
		global = nil
		return err
	}
	global = svc
	logCapabilities(svc.inner, opts.LogHardware)
	return nil
}

func newService(ctx context.Context, opts InitOptions) (*Service, error) {
	if opts.MaxConcurrent < 1 {
		opts.MaxConcurrent = 4
	}
	if opts.CacheDir == "" {
		opts.CacheDir = os.TempDir()
	}

	if opts.LogHardware {
		logger.Infof("Detecting ffmpeg hardware codec support (gpu: %s)...", opts.GPU)
	}

	inner, err := goffmpeg.New(ctx, goffmpeg.Config{
		FFmpegPath:    opts.FFmpegPath,
		MaxConcurrent: opts.MaxConcurrent,
		Logger:        ffmpegLogger(opts.Debug),
		GPU:           opts.GPU,
		SkipHWTests:   opts.SkipHWTests,
		VerboseFFmpeg: opts.Debug,
	})
	if err != nil {
		return nil, err
	}

	return &Service{
		inner:        inner,
		cacheDir:     opts.CacheDir,
		exiftoolPath: opts.ExiftoolPath,
	}, nil
}

// Get returns the preview/media ffmpeg service, or nil when ffmpeg is unavailable.
func Get() *Service {
	return global
}

// Enabled reports whether ffmpeg initialized successfully.
func Enabled() bool {
	return global != nil && global.inner != nil
}

// Capabilities returns detected ffmpeg capabilities, or nil when unavailable.
func (s *Service) Capabilities() *capabilities.Capabilities {
	if s == nil || s.inner == nil {
		return nil
	}
	return s.inner.Capabilities()
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
	waitStart := time.Now()
	err := s.inner.Acquire(ctx)
	if wait := time.Since(waitStart); wait >= 200*time.Millisecond {
		if err != nil {
			logger.Infof("ffmpeg preview acquire failed after %s: %v", formatAcquireWait(wait), err)
		} else {
			logger.Infof("ffmpeg preview acquire waited %s", formatAcquireWait(wait))
		}
	}
	return err
}

func formatAcquireWait(d time.Duration) string {
	if d < 0 {
		d = 0
	}
	return fmt.Sprintf("%.0fms", float64(d)/float64(time.Millisecond))
}

func (s *Service) Release() {
	if s == nil || s.inner == nil {
		return
	}
	s.inner.Release()
}

// VideoPreview extracts a JPEG preview frame to w.
func (s *Service) VideoPreviewAtTime(ctx context.Context, w io.Writer, videoPath string, seekSec float64) error {
	if s == nil || s.inner == nil {
		return fmt.Errorf("ffmpeg service not available")
	}
	if err := s.Acquire(ctx); err != nil {
		return err
	}
	defer s.Release()

	dur, err := s.inner.GetMediaDuration(ctx, videoPath)
	if err != nil {
		return err
	}

	seekPct := 10.0
	if dur > 0 {
		if seekSec <= 0 {
			seekPct = 0.01
		} else {
			seekPct = (seekSec / dur) * 100
			if seekPct > 100 {
				seekPct = 100
			}
			if seekPct < 0.01 {
				seekPct = 0.01
			}
		}
	}

	return s.inner.VideoPreview(ctx, w, ops.PreviewOptions{
		Input:       videoPath,
		SeekPercent: seekPct,
		Quality:     10,
	})
}

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

func ffmpegLogger(debug bool) goffmpeg.Logger {
	if !debug {
		return goffmpeg.NopLogger()
	}
	return gtlogger.WithGroup(logger.GetGlobalLogger())
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
