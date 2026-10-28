package ffmpeg

import (
	"context"
	"fmt"
	"io"
	"os"
	"strings"
	"sync"
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

	// gpus caches the host GPU enumeration, which shells out on some platforms
	// (PowerShell on Windows) and so must not run per settings request.
	gpusOnce sync.Once
	gpus     []platform.GPUDevice
}

// gpuList returns the cached host GPU enumeration. A Service is replaced on
// re-initialization, so the cache is refreshed whenever ffmpeg is re-detected.
func (s *Service) gpuList() []platform.GPUDevice {
	if s == nil {
		return nil
	}
	s.gpusOnce.Do(func() { s.gpus, _ = platform.ListGPUs() })
	return s.gpus
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

	logCapabilities(svc, opts.HardwareAcceleration, opts.AccelDevice)
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

// ResolveVideoEncoder returns the encoder selection for an encode profile so
// callers can surface which encoder/accel a transcode will use.
func (s *Service) ResolveVideoEncoder(profile goffmpeg.VideoProfile) (capabilities.EncoderSelection, error) {
	if s == nil || s.inner == nil {
		return capabilities.EncoderSelection{}, fmt.Errorf("ffmpeg service unavailable")
	}
	return s.inner.ResolveVideoEncoder(profile)
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
}

// AccelDeviceOptions lists concrete acceleration devices for the admin UI.
// The default auto-selected GPU (when stored as "auto") is included by name;
// there is no separate "auto" entry.
func (s *Service) AccelDeviceOptions() []AccelDeviceOption {
	out := []AccelDeviceOption{}
	if caps := s.Capabilities(); caps != nil {
		defaultLabel, defaultID, gpuOpts := resolvedAccelDeviceOptions(caps, s.gpuList())
		if defaultID != "" {
			out = append(out, AccelDeviceOption{ID: defaultID, Label: defaultLabel})
		}
		out = append(out, gpuOpts...)
	}
	out = append(out, AccelDeviceOption{ID: "software", Label: "CPU (software)"})
	return out
}

// ResolveAccelDevice maps stored "auto"/empty to the concrete device the UI
// should display; explicit software aliases and device IDs are unchanged.
func (s *Service) ResolveAccelDevice(device string) string {
	dev := strings.TrimSpace(device)
	switch strings.ToLower(dev) {
	case "software", "cpu", "none":
		return "software"
	}
	if dev != "" && !strings.EqualFold(dev, "auto") {
		return dev
	}
	caps := s.Capabilities()
	if caps == nil {
		return "software"
	}
	defaultLabel, defaultID, gpuOpts := resolvedAccelDeviceOptions(caps, s.gpuList())
	if defaultID != "" {
		return defaultID
	}
	if len(gpuOpts) == 1 {
		return gpuOpts[0].ID
	}
	for _, opt := range gpuOpts {
		if opt.Label == defaultLabel {
			return opt.ID
		}
	}
	if id, _ := wslNVIDIAAccelDefault(caps); id != "" {
		return id
	}
	return "software"
}

// resolvedAccelDeviceOptions labels the default auto-selected GPU and lists
// every other enumerated device.
func resolvedAccelDeviceOptions(caps *capabilities.Capabilities, gpus []platform.GPUDevice) (autoLabel, resolvedID string, opts []AccelDeviceOption) {
	resolved := caps.Platform.Details["render_device"]
	autoAccel := autoGPUAccel(caps)
	wslID, wslLabel := wslNVIDIAAccelDefault(caps)
	seen := map[string]bool{}
	for _, g := range gpus {
		label := gpuClassLabel(g) + " (" + gpuDisplayName(g) + ")"
		if seen[accelDeviceID(g)] {
			continue
		}
		seen[accelDeviceID(g)] = true
		switch {
		case resolvedID == "" && wslID != "" && g.Vendor == "nvidia" && g.RenderNode == "" && resolved == "" && autoAccel.Vendor == "":
			// WSL exposes the NVIDIA GPU without a DRM node; the enumerated
			// entry is the auto default, so it must not be listed twice.
			autoLabel, resolvedID = label, accelDeviceID(g)
			continue
		case g.RenderNode != "" && g.RenderNode == resolved:
			autoLabel, resolvedID = label, g.RenderNode
			continue
		case resolvedID == "" && autoAccel.Vendor != "" && g.Vendor == autoAccel.Vendor:
			// Vendor-level default for platforms without a render node (macOS,
			// synthetic GPUs): this device backs the preferred hardware encoder,
			// so it is the resolved auto selection, not an extra option.
			autoLabel, resolvedID = label, accelDeviceID(g)
			continue
		}
		if id := accelDeviceID(g); id != "" {
			opts = append(opts, AccelDeviceOption{ID: id, Label: label})
		}
	}
	if autoLabel == "" {
		autoLabel = autoAccel.Name
	}
	if resolvedID == "" {
		if wslID != "" {
			if autoLabel == "" {
				autoLabel = wslLabel
			}
			resolvedID = wslID
		}
	}
	return autoLabel, resolvedID, opts
}

// wslNVIDIAAccelDefault exposes dgpu when WSL has an NVIDIA driver but no DRM nodes.
func wslNVIDIAAccelDefault(caps *capabilities.Capabilities) (id, label string) {
	if caps == nil || !caps.Platform.WSL || !caps.Platform.NVIDIA {
		return "", ""
	}
	name := strings.TrimSpace(caps.Platform.Details["gpu"])
	if name == "" {
		name = platform.NVIDIAGPUName()
	}
	if name == "" {
		name = "NVIDIA GPU"
	}
	return "dgpu", "Dedicated GPU (" + name + ")"
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
	switch g.DeviceClass {
	case "soc":
		return "SoC GPU"
	case "igpu":
		return "Integrated GPU"
	case "dgpu":
		return "Dedicated GPU"
	}
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

// autoGPUAccel describes the GPU driving the detected default hardware
// encoder (e.g. Apple VideoToolbox → "Apple GPU"/vendor "apple").
func autoGPUAccel(caps *capabilities.Capabilities) capabilities.AccelGPU {
	var fallback capabilities.AccelType
	for _, opt := range caps.EncodeOptions {
		if !opt.Available || opt.Accel == "" || opt.Accel == capabilities.AccelNone {
			continue
		}
		if opt.Codec != capabilities.CodecH264 {
			continue
		}
		if opt.Default {
			if gpu := capabilities.GPUForAccel(caps, opt.Accel); gpu.Name != "" {
				return gpu
			}
		}
		fallback = opt.Accel
	}
	gpu := capabilities.GPUForAccel(caps, fallback)
	if gpu.Name == "CPU" || gpu.Vendor == "software" {
		return capabilities.AccelGPU{}
	}
	return gpu
}

// EncoderBackendOption is one selectable entry in the encoder backend list.
type EncoderBackendOption struct {
	ID    string `json:"id"`
	Label string `json:"label"`
}

// EncoderBackendOptions lists concrete H.264 encode paths for the admin UI.
// Labels are "device - encoder" so multiple hardware backends stay distinct.
// An empty stored encoderBackend (Default in the UI) lets go-ffmpeg pick from
// the selected acceleration device.
func (s *Service) EncoderBackendOptions() []EncoderBackendOption {
	caps := s.Capabilities()
	if caps == nil {
		return []EncoderBackendOption{{ID: "cpu", Label: encoderBackendLabel(nil, capabilities.EncodeOption{Encoder: "libx264"})}}
	}
	var out []EncoderBackendOption
	seen := map[string]bool{}
	for _, opt := range caps.EncodeOptions {
		if opt.Codec != capabilities.CodecH264 || !opt.Available {
			continue
		}
		id := encoderBackendOptionID(opt)
		if seen[id] {
			continue
		}
		seen[id] = true
		out = append(out, EncoderBackendOption{ID: id, Label: encoderBackendLabel(caps, opt)})
	}
	if len(out) == 0 {
		return []EncoderBackendOption{{ID: "cpu", Label: encoderBackendLabel(caps, capabilities.EncodeOption{Encoder: "libx264"})}}
	}
	return out
}

func encoderBackendOptionID(opt capabilities.EncodeOption) string {
	if opt.Accel == "" || opt.Accel == capabilities.AccelNone {
		return "cpu"
	}
	return opt.Encoder
}

func encoderBackendLabel(caps *capabilities.Capabilities, opt capabilities.EncodeOption) string {
	device := "CPU (software)"
	if opt.Accel != "" && opt.Accel != capabilities.AccelNone {
		if caps != nil {
			gpu := capabilities.GPUForAccel(caps, opt.Accel)
			switch {
			case gpu.RenderNode != "":
				device = gpu.RenderNode
			case gpu.Name != "" && gpu.Name != "CPU":
				device = gpu.Name
			default:
				device = string(opt.Accel)
			}
		} else {
			device = string(opt.Accel)
		}
	}
	encoder := opt.Encoder
	if encoder == "" {
		encoder = "libx264"
	}
	return device + " - " + encoder
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

// ProbeVideoFPS returns the average frame rate of the first video stream.
func (s *Service) ProbeVideoFPS(ctx context.Context, path string) (float64, error) {
	if s == nil || s.inner == nil {
		return 0, fmt.Errorf("ffmpeg service not available")
	}
	return s.inner.ProbeVideoFPS(ctx, path)
}

// VideoPreview extracts a JPEG preview frame to w.
//
// go-ffmpeg deprecated Service.VideoPreview in favor of CaptureFrame, so this
// wrapper keeps the PreviewOptions call shape used by the preview service while
// delegating to the supported capture API. go-ffmpeg Service methods acquire
// concurrency slots automatically; do not Acquire before calling.
func (s *Service) VideoPreview(ctx context.Context, w io.Writer, opts ops.PreviewOptions) error {
	if s == nil || s.inner == nil {
		return fmt.Errorf("ffmpeg service not available")
	}

	frame := ops.PreviewOptionsToFrame(opts, 0)
	// Mirror the legacy ResolvePreviewSeekSec behavior: an explicit SeekSec
	// wins, otherwise SeekPercent (defaulting to 10%) is resolved against the
	// duration inside CaptureFrame.
	if opts.SeekSec > 0 {
		frame.At = ops.AtSec(opts.SeekSec)
	} else {
		percent := opts.SeekPercent
		if percent <= 0 || percent > 100 {
			percent = 10
		}
		frame.At = ops.AtPercent(percent)
	}

	return s.inner.CaptureFrame(ctx, w, frame)
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

func logCapabilities(svc *goffmpeg.Service, wantsHardware bool, accelDevice string) {
	caps := svc.Capabilities()
	if caps == nil {
		return
	}

	logger.Debugf("Media Enabled: version %s @ %s", caps.FFmpegVersion, caps.FFmpegPath)

	warnIfAccelDeviceUnavailable(caps, accelDevice)

	hw := hardwareCodecSummary(svc)
	if hw == "" {
		if wantsHardware {
			logger.Warning("hardware acceleration is enabled but no ffmpeg hardware codec support was found")
		} else {
			logger.Debug("no ffmpeg hardware codec support found")
		}
		logHWEncodeProbeFailures(caps)
		return
	}
	if wantsHardware {
		// go-ffmpeg already logs a single hardware support overview line at
		// startup; keep the detailed per-codec list at debug only.
		logger.Debugf("ffmpeg hardware codecs: %s", hw)
		logHWEncodeProbeFailures(caps)
		return
	}
	logger.Debugf("supported ffmpeg hardware codecs: %s", hw)
}

// warnIfAccelDeviceUnavailable surfaces a stale configured device. The stored
// device (e.g. a GPU name saved on another host) may no longer be present; the
// resolver falls back to auto, but silently, so the transcodes look like a
// plain software encode with no explanation. Resolve with no side effects and
// report the fallback at info.
func warnIfAccelDeviceUnavailable(caps *capabilities.Capabilities, accelDevice string) {
	if reason, ok := accelDeviceFallbackReason(accelDevice); ok {
		logger.Warningf("ffmpeg accel device %q is unavailable (%s); using automatic selection instead", strings.TrimSpace(accelDevice), reason)
	}
}

// accelDeviceFallbackReason reports whether a configured accel device cannot be
// used on this host and why. A blank/"auto"/"software" value is not a device
// selection, so it never warns.
func accelDeviceFallbackReason(accelDevice string) (string, bool) {
	dev := strings.TrimSpace(accelDevice)
	switch strings.ToLower(dev) {
	case "", "auto", "software", "cpu", "none":
		return "", false
	}
	sel, err := goffmpeg.ParseGPUSelector(dev)
	if err != nil {
		return fmt.Sprintf("unparseable device (%v)", err), true
	}
	choice, err := goffmpeg.ResolveGPUSelector(sel, goffmpeg.GPUFallbackOnUnavailable)
	if err != nil {
		return err.Error(), true
	}
	if choice.Fallback {
		if choice.FallbackReason != "" {
			return choice.FallbackReason, true
		}
		return "not present on this host", true
	}
	return "", false
}

// logHWEncodeProbeFailures lists hardware encoders that failed their functional
// probe, distinctly from decoders, so a "hardware decode works but encode
// silently used libx264" case is visible. Only reports when there is at least
// one failure, and only for h264/hevc/av1 (the codecs transcode uses).
func logHWEncodeProbeFailures(caps *capabilities.Capabilities) {
	var failures []string
	for _, row := range caps.EncoderProbeResults() {
		if row.Available || row.Accel == "" || row.Accel == capabilities.AccelNone {
			continue
		}
		entry := fmt.Sprintf("%s (%s, %s)", row.Encoder, row.Accel, row.Code)
		if row.Hint != "" {
			entry += ": " + row.Hint
		}
		failures = append(failures, entry)
	}
	if len(failures) == 0 {
		return
	}
	logger.Debugf("ffmpeg hardware encoders unavailable: %s", strings.Join(failures, "; "))
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
