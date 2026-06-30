package settings

import (
	"strings"

	"github.com/gtsteffaniak/go-ffmpeg/capabilities"
	"github.com/gtsteffaniak/go-logger/logger"
)

// MediaGPUConfig holds parsed ffmpeg GPU settings.
type MediaGPUConfig struct {
	GPU         string
	Enabled     bool
	LogHardware bool
}

func normalizeMediaGPU(raw string) MediaGPUConfig {
	trimmed := strings.TrimSpace(raw)
	if trimmed == "" {
		trimmed = "default"
	}
	if strings.EqualFold(trimmed, "software") {
		return MediaGPUConfig{}
	}
	if _, err := capabilities.ResolveGPUChoice(trimmed); err != nil {
		logger.Warningf("invalid gpu %q: %v; hardware acceleration disabled", raw, err)
		return MediaGPUConfig{}
	}
	return MediaGPUConfig{
		GPU:         trimmed,
		Enabled:     true,
		LogHardware: true,
	}
}

// MediaGPUSettings returns parsed GPU settings from the current config.
func MediaGPUSettings() MediaGPUConfig {
	return normalizeMediaGPU(Config.Integrations.Media.GPU)
}
