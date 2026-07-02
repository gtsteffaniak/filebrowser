package settings_test

import (
	"testing"

	"github.com/gtsteffaniak/filebrowser/backend/common/settings"
)

func TestMediaGPUSettingsDefaultWhenEmpty(t *testing.T) {
	prevGPU := settings.Config.Integrations.Media.GPU
	t.Cleanup(func() { settings.Config.Integrations.Media.GPU = prevGPU })

	settings.Config.Integrations.Media.GPU = ""
	got := settings.MediaGPUSettings()
	if !got.Enabled || got.GPU != "default" || !got.LogHardware {
		t.Fatalf("empty gpu = %+v, want default enabled", got)
	}
}

func TestMediaGPUSettingsEnabledWhenSet(t *testing.T) {
	prevGPU := settings.Config.Integrations.Media.GPU
	t.Cleanup(func() { settings.Config.Integrations.Media.GPU = prevGPU })

	settings.Config.Integrations.Media.GPU = "default"
	got := settings.MediaGPUSettings()
	if !got.Enabled || got.GPU != "default" || !got.LogHardware {
		t.Fatalf("default gpu = %+v", got)
	}
}

func TestMediaGPUSettingsDisabledWhenSoftware(t *testing.T) {
	prevGPU := settings.Config.Integrations.Media.GPU
	t.Cleanup(func() { settings.Config.Integrations.Media.GPU = prevGPU })

	settings.Config.Integrations.Media.GPU = "software"
	got := settings.MediaGPUSettings()
	if got.Enabled || got.GPU != "" || got.LogHardware {
		t.Fatalf("software gpu = %+v, want disabled", got)
	}
}
