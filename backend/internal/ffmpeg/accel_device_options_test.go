package ffmpeg

import (
	"strings"
	"testing"

	"github.com/gtsteffaniak/go-ffmpeg/capabilities"
	"github.com/gtsteffaniak/go-ffmpeg/platform"
)

func TestAccelDeviceOptionsOmitsAuto(t *testing.T) {
	s := &Service{}
	opts := s.AccelDeviceOptions()
	for _, opt := range opts {
		if opt.ID == "auto" {
			t.Fatalf("options must not include auto: %+v", opts)
		}
	}
	if len(opts) != 1 || opts[0].ID != "software" {
		t.Fatalf("without capabilities want software only, got %+v", opts)
	}
}

func TestResolveAccelDeviceWithoutCapabilities(t *testing.T) {
	s := &Service{}
	if got := s.ResolveAccelDevice("auto"); got != "software" {
		t.Fatalf("ResolveAccelDevice(auto) = %q, want software", got)
	}
	if got := s.ResolveAccelDevice("/dev/dri/renderD128"); got != "/dev/dri/renderD128" {
		t.Fatalf("ResolveAccelDevice(explicit) = %q", got)
	}
}

func TestResolveAccelDeviceWSLNVIDIA(t *testing.T) {
	caps := capabilities.NewCapabilities()
	caps.Platform = capabilities.PlatformInfo{OS: "linux", WSL: true, NVIDIA: true}
	caps.Platform.Details = map[string]string{"gpu": "NVIDIA GeForce RTX 3050"}
	// Inject caps through a minimal service wrapper is not available; test helper directly.
	id, label := wslNVIDIAAccelDefault(caps)
	if id != "dgpu" || label != "Dedicated GPU (NVIDIA GeForce RTX 3050)" {
		t.Fatalf("wsl default = %q / %q", id, label)
	}
}

func TestAccelDeviceFallbackReason(t *testing.T) {
	// A blank or auto/software value is not a device selection.
	for _, dev := range []string{"", "auto", "software", "cpu", "none", "AUTO"} {
		if _, ok := accelDeviceFallbackReason(dev); ok {
			t.Fatalf("%q must not report a fallback", dev)
		}
	}
	// A GPU name that is not on this host must be reported as falling back.
	// The synthetic device will not match host enumeration in CI.
	if reason, ok := accelDeviceFallbackReason("Definitely Not A Real GPU 9999"); !ok {
		t.Fatal("an unmatched device name must report a fallback reason")
	} else if reason == "" {
		t.Fatal("the fallback reason must be non-empty")
	}
}

func wslCaps() *capabilities.Capabilities {
	caps := capabilities.NewCapabilities()
	caps.Platform = capabilities.PlatformInfo{OS: "linux", WSL: true, NVIDIA: true}
	caps.Platform.Details = map[string]string{"gpu": "NVIDIA GeForce RTX 3050"}
	return caps
}

// WSL enumerates the NVIDIA GPU without a render node; the device menu must
// list it once, whether it is resolved through enumeration or the WSL default.
func TestAccelDeviceOptionsWSLListsNVIDIAOnce(t *testing.T) {
	gpus := []platform.GPUDevice{{Name: "NVIDIA GeForce RTX 3050", Vendor: "nvidia", DeviceClass: "dgpu"}}
	label, id, opts := resolvedAccelDeviceOptions(wslCaps(), gpus)
	if id == "" || label == "" {
		t.Fatalf("expected a resolved default, got %q/%q", id, label)
	}
	for _, o := range opts {
		if o.ID == id || strings.Contains(o.Label, "RTX 3050") {
			t.Fatalf("device listed twice: default=%q opts=%+v", id, opts)
		}
	}
}

func TestAccelDeviceOptionsKeepsDistinctGPUs(t *testing.T) {
	gpus := []platform.GPUDevice{
		{Name: "AMD Radeon Graphics", Vendor: "amd", DeviceClass: "igpu", Integrated: true, RenderNode: "/dev/dri/renderD128"},
		{Name: "NVIDIA GeForce RTX 3050", Vendor: "nvidia", DeviceClass: "dgpu", RenderNode: "/dev/dri/renderD129"},
	}
	caps := capabilities.NewCapabilities()
	caps.Platform.Details = map[string]string{"render_device": "/dev/dri/renderD128"}
	_, id, opts := resolvedAccelDeviceOptions(caps, gpus)
	if id != "/dev/dri/renderD128" || len(opts) != 1 || opts[0].ID != "/dev/dri/renderD129" {
		t.Fatalf("id=%q opts=%+v", id, opts)
	}
}
