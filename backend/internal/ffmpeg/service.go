package ffmpeg

import (
	"context"
	"fmt"
	"io"
	"os"
	"strings"
	"sync/atomic"

	goffmpeg "github.com/gtsteffaniak/go-ffmpeg"
	"github.com/gtsteffaniak/go-ffmpeg/capabilities"
	"github.com/gtsteffaniak/go-ffmpeg/ops"
	"github.com/gtsteffaniak/go-logger/logger"
)

// Service wraps go-ffmpeg for filebrowser media operations.
type Service struct {
	inner    *goffmpeg.Service
	cacheDir string
}

// FFmpegService is kept for existing callers.
type FFmpegService = Service

// global holds the process-wide ffmpeg service. It is swapped wholesale by
// ApplyMediaSettings, so it is stored atomically to keep concurrent readers
// (transcode admission, previews, media handlers) race-free.
var global atomic.Pointer[Service]

// InitOptions configures startup initialization.
type InitOptions struct {
	FFmpegPath           string
	MaxConcurrent        int // deprecated: use Concurrency
	Concurrency          goffmpeg.Concurrency
	CacheDir             string
	HardwareAcceleration bool
	// VerboseFFmpeg streams ffmpeg process output to stderr and raises the ffmpeg
	// log level. Wired to the media debug setting.
	VerboseFFmpeg bool
	// SkipHWTests skips the hardware encoder probe. Only set this when hardware
	// acceleration is pinned off in the config file, otherwise the admin cannot
	// enable it without a restart.
	SkipHWTests bool
	// EncoderHierarchy restricts which acceleration backends are preferred.
	// Pass []capabilities.AccelType{capabilities.AccelNone} to keep detection
	// (so hardware availability stays known) while preferring software.
	EncoderHierarchy []capabilities.AccelType
}

// Initialize creates the global ffmpeg service and runs capability detection.
// Hardware encoder smoke tests run so hardware acceleration can be enabled or
// disabled at runtime without a restart, unless the config file pins it off.
func Initialize(ctx context.Context, opts InitOptions) error {
	if opts.CacheDir == "" {
		opts.CacheDir = os.TempDir()
	}

	if opts.SkipHWTests {
		logger.Info("Skipping ffmpeg hardware codec detection (hardware acceleration disabled in config)")
	} else {
		logger.Info("Detecting ffmpeg codec support...")
	}

	cfg := goffmpeg.Config{
		FFmpegPath:       opts.FFmpegPath,
		Concurrency:      opts.Concurrency,
		Logger:           logger.GetGlobalLogger(),
		VerboseFFmpeg:    opts.VerboseFFmpeg,
		SkipHWTests:      opts.SkipHWTests,
		EncoderHierarchy: opts.EncoderHierarchy,
	}
	if opts.Concurrency.MaxProbe == 0 && opts.Concurrency.MaxDecode == 0 && opts.Concurrency.MaxEncode == 0 {
		if opts.MaxConcurrent < 1 {
			opts.MaxConcurrent = 4
		}
		cfg.MaxConcurrent = opts.MaxConcurrent
	}

	svc, err := goffmpeg.New(ctx, cfg)
	if err != nil {
		global.Store(nil)
		return err
	}

	global.Store(&Service{
		inner:    svc,
		cacheDir: opts.CacheDir,
	})

	logCapabilities(svc, opts.HardwareAcceleration)
	return nil
}

// HardwareAccelerationAvailable reports whether any hardware encoder is usable.
func HardwareAccelerationAvailable() bool {
	return global.Load().HardwareAccelerationAvailable()
}

// HardwareAccelerationAvailable reports whether any hardware encoder is usable.
func (s *Service) HardwareAccelerationAvailable() bool {
	if s == nil || s.inner == nil {
		return false
	}
	caps := s.inner.Capabilities()
	if caps == nil {
		return false
	}
	for _, enc := range caps.Encoders {
		if enc.Available && enc.Kind != "" && enc.Kind != "software" {
			return true
		}
	}
	return false
}

// Get returns the initialized service, or nil when ffmpeg is unavailable.
func Get() *Service {
	return global.Load()
}

// Enabled reports whether ffmpeg initialized successfully.
func Enabled() bool {
	svc := global.Load()
	return svc != nil && svc.inner != nil
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
// go-ffmpeg Service methods acquire concurrency slots automatically; do not Acquire before calling.
func (s *Service) VideoPreview(ctx context.Context, w io.Writer, opts ops.PreviewOptions) error {
	if s == nil || s.inner == nil {
		return fmt.Errorf("ffmpeg service not available")
	}
	return s.inner.VideoPreview(ctx, w, opts)
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

	content, err := s.inner.ExtractSubtitle(ctx, videoPath, streamIndex)
	if err != nil {
		return "", err
	}

	SubtitleContentCache.Set(cacheKey, content)
	return content, nil
}

func logCapabilities(svc *goffmpeg.Service, wantsHardware bool) {
	caps := svc.Capabilities()
	if caps == nil {
		return
	}

	logger.Debugf("Media Enabled: version %s @ %s", caps.FFmpegVersion, caps.FFmpegPath)

	hw := hardwareCodecSummary(svc)
	if hw == "" {
		if wantsHardware {
			logger.Warning("hardware acceleration is enabled but no ffmpeg hardware codec support was found")
		} else {
			logger.Debug("no ffmpeg hardware codec support found")
		}
		return
	}
	logger.Debugf("supported ffmpeg hardware codecs: %s", hw)
}

// hardwareCodecSummary lists the hardware encode/decode paths that detection
// found usable. It reads the capability maps directly instead of the
// hierarchy-filtered encode/decode options, so hardware stays reported when it
// is detected but not preferred (for example when hardware acceleration is
// disabled and software encoders are selected instead).
func hardwareCodecSummary(svc *goffmpeg.Service) string {
	if svc == nil {
		return ""
	}
	return hardwareCodecSummaryFromCaps(svc.Capabilities())
}

func hardwareCodecSummaryFromCaps(caps *capabilities.Capabilities) string {
	if caps == nil {
		return ""
	}

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

	for _, name := range caps.HardwareEncoders() {
		add(fmt.Sprintf("%s encode via %s (%s)", encoderCodec(name), name,
			capabilities.AccelLabel(capabilities.EncoderAccel(name))))
	}
	for _, name := range caps.HardwareDecoders() {
		add(fmt.Sprintf("%s decode via %s (%s)", caps.Decoders[name].Codec, name,
			capabilities.AccelLabel(capabilities.DecoderAccel(name))))
	}

	return strings.Join(parts, ", ")
}

func encoderCodec(name string) capabilities.VideoCodec {
	for _, codec := range []capabilities.VideoCodec{
		capabilities.CodecH264,
		capabilities.CodecHEVC,
		capabilities.CodecAV1,
		capabilities.CodecVP9,
	} {
		if capabilities.EncoderMatchesCodec(name, codec) {
			return codec
		}
	}
	return ""
}
