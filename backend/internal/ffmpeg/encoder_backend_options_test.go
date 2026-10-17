package ffmpeg

import (
	"strings"
	"testing"

	"github.com/gtsteffaniak/go-ffmpeg/capabilities"
	"github.com/gtsteffaniak/go-ffmpeg/platform"
)

func TestEncoderBackendLabelDeviceAndEncoder(t *testing.T) {
	caps := capabilities.NewCapabilities()
	caps.SelectedGPU = &platform.GPUChoice{
		Enabled:      true,
		Name:         "NVIDIA RTX",
		RenderDevice: "/dev/dri/renderD128",
	}
	if caps.Platform.Details == nil {
		caps.Platform.Details = map[string]string{}
	}
	caps.Platform.Details["render_device"] = "/dev/dri/renderD128"
	label := encoderBackendLabel(caps, capabilities.EncodeOption{
		Accel:   capabilities.AccelNVENC,
		Encoder: "h264_nvenc",
	})
	if !strings.Contains(label, "/dev/dri/renderD128") || !strings.Contains(label, "h264_nvenc") {
		t.Fatalf("label = %q, want device - encoder", label)
	}
}

func TestEncoderBackendOptionIDUsesEncoderNameForHardware(t *testing.T) {
	id := encoderBackendOptionID(capabilities.EncodeOption{
		Accel:   capabilities.AccelVAAPI,
		Encoder: "h264_vaapi",
	})
	if id != "h264_vaapi" {
		t.Fatalf("id = %q, want h264_vaapi", id)
	}
}
