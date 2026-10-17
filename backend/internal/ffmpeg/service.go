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
	"github.com/gtsteffaniak/go-ffmpeg/platform"
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
	// AccelDevice scopes hardware acceleration to one device: "auto" (default),
	// "software"/"cpu", "igpu", "dgpu", a DRM render node, or a GPU name
	// substring. Parsed by go-ffmpeg; an unavailable device falls back to auto.
	AccelDevice string
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
	if sel, selErr := goffmpeg.ParseGPUSelector(opts.AccelDevice); selErr == nil {
		// GPUStrictness stays at the zero value: a selected device that is not
		// present falls back to auto instead of failing ffmpeg init.
		cfg.GPUSelector = sel
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

// Capabilities exposes the detected ffmpeg capability matrix, or nil when the
// service is unavailable.
func (s *Service) Capabilities() *capabilities.Capabilities {
	if s == nil || s.inner == nil {
		return nil
	}
	return s.inner.Capabilities()
}

// AccelDeviceOption is one selectable entry in the acceleration device list.
type AccelDeviceOption struct {
	ID    string `json:"id"`
	Label string `json:"label"`
	// ResolvedID is the concrete device the "auto" option resolves to, so the
	// UI can display it when a stored selection pins that same device.
	ResolvedID string `json:"resolvedId,omitempty"`
}

// AccelDeviceOptions lists the acceleration devices the admin UI can offer.
// Every option is a concrete device labeled "<class> (<device name>)": the
// auto entry carries whichever device auto-selection resolves to, remaining
// GPUs are listed by name so several same-class cards stay distinguishable.
func (s *Service) AccelDeviceOptions() []AccelDeviceOption {
	auto := AccelDeviceOption{ID: "auto"}
	gpuOpts := []AccelDeviceOption{}
	if caps := s.Capabilities(); caps != nil {
		auto.Label, auto.ResolvedID, gpuOpts = resolvedAccelDeviceOptions(caps)
	}
	if auto.Label == "" {
		auto.Label = "Auto"
	}
	out := []AccelDeviceOption{auto}
	out = append(out, gpuOpts...)
	out = append(out, AccelDeviceOption{ID: "software", Label: "CPU (software)"})
	return out
}

// resolvedAccelDeviceOptions builds the auto option label and per-device
// options; the device auto resolves to is folded into the auto entry.
func resolvedAccelDeviceOptions(caps *capabilities.Capabilities) (autoLabel, resolvedID string, opts []AccelDeviceOption) {
	resolved := caps.Platform.Details["render_device"]
	gpus, _ := platform.ListGPUs()
	for _, g := range gpus {
		label := gpuClassLabel(g) + " (" + gpuDisplayName(g) + ")"
		if g.RenderNode != "" && g.RenderNode == resolved {
			autoLabel, resolvedID = label, g.RenderNode
			continue
		}
		if id := accelDeviceID(g); id != "" {
			opts = append(opts, AccelDeviceOption{ID: id, Label: label})
		}
	}
	if autoLabel == "" {
		autoLabel = autoGPUName(caps)
	}
	return autoLabel, resolvedID, opts
}

// accelDeviceID prefers the render node so the selection can be passed back to
// ffmpeg; GPU name is the fallback on platforms without DRM render nodes.
func accelDeviceID(g platform.GPUDevice) string {
	if g.RenderNode != "" {
		return g.RenderNode
	}
	return g.Name
}

// gpuClassLabel names the device class shown in the dropdown.
func gpuClassLabel(g platform.GPUDevice) string {
	if g.Integrated {
		return "Integrated GPU"
	}
	return "Dedicated GPU"
}

func gpuDisplayName(g platform.GPUDevice) string {
	if g.Name != "" {
		return g.Name
	}
	return g.RenderNode
}

// autoGPUName names the GPU driving the detected default hardware encoder on
// platforms where individual devices cannot be enumerated (e.g. "Apple GPU").
func autoGPUName(caps *capabilities.Capabilities) string {
	var fallback capabilities.AccelType
	for _, opt := range caps.EncodeOptions {
		if !opt.Available || opt.Accel == "" || opt.Accel == capabilities.AccelNone {
			continue
		}
		if opt.Codec != capabilities.CodecH264 {
			continue
		}
		if opt.Default {
			if name := capabilities.GPUForAccel(caps, opt.Accel).Name; name != "" {
				return name
			}
		}
		fallback = opt.Accel
	}
	if name := capabilities.GPUForAccel(caps, fallback).Name; name != "" && name != "CPU" {
		return name
	}
	return ""
}

// EncoderBackendOption is one selectable entry in the encoder backend list.
type EncoderBackendOption struct {
	ID      string `json:"id"`
	Label   string `json:"label"`
	Default bool   `json:"default,omitempty"`
}

// EncoderBackendOptions lists the detected backends that can encode H.264 —
// the codec every transcode preset outputs. "cpu" covers software encoders;
// the detected default is flagged so the UI can describe the auto choice.
func (s *Service) EncoderBackendOptions() []EncoderBackendOption {
	out := []EncoderBackendOption{{ID: "cpu", Label: "CPU (software)"}}
	caps := s.Capabilities()
	if caps == nil {
		return out
	}
	seen := map[string]bool{"cpu": true}
	for _, opt := range caps.EncodeOptions {
		if opt.Codec != capabilities.CodecH264 || !opt.Available {
			continue
		}
		id := string(opt.Accel)
		if id == "" || id == string(capabilities.AccelNone) {
			id = "cpu"
		}
		if seen[id] {
			continue
		}
		seen[id] = true
		label := opt.Label
		if label == "" {
			label = opt.Encoder
		}
		out = append(out, EncoderBackendOption{ID: id, Label: label, Default: opt.Default})
	}
	return out
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

// hardwareCodecSummary lists detected hardware encode/decode paths. It reads
// the capability maps directly so hardware is still reported when detected
// but not preferred (e.g. hardware acceleration disabled).
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
