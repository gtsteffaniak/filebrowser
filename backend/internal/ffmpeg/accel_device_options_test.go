package ffmpeg

import (
	"strings"
	"testing"

	"github.com/gtsteffaniak/go-ffmpeg/capabilities"
	"github.com/gtsteffaniak/go-ffmpeg/platform"
)

func TestAccelDeviceOptionsIncludesAuto(t *testing.T) {
	s := &Service{}
	opts := s.AccelDeviceOptions()
	if len(opts) != 2 || opts[0].ID != "auto" || opts[1].ID != "software" {
		t.Fatalf("without capabilities want auto + software, got %+v", opts)
	}
}

func TestResolveAccelDeviceWithoutCapabilities(t *testing.T) {
	s := &Service{}
	if got := s.ResolveAccelDevice("auto"); got != "auto" {
		t.Fatalf("ResolveAccelDevice(auto) = %q, want auto", got)
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
	label, _, opts := resolvedAccelDeviceOptions(wslCaps(), gpus)
	if label == "" {
		t.Fatalf("expected an auto label, got %q", label)
	}
	if len(opts) != 1 || !strings.Contains(opts[0].Label, "RTX 3050") {
		t.Fatalf("want one NVIDIA option, got %+v", opts)
	}
}

func TestAccelDeviceOptionsKeepsDistinctGPUs(t *testing.T) {
	gpus := []platform.GPUDevice{
		{Name: "AMD Radeon Graphics", Vendor: "amd", DeviceClass: "igpu", Integrated: true, RenderNode: "/dev/dri/renderD128"},
		{Name: "NVIDIA GeForce RTX 3050", Vendor: "nvidia", DeviceClass: "dgpu", RenderNode: "/dev/dri/renderD129"},
	}
	caps := capabilities.NewCapabilities()
	caps.Platform.Details = map[string]string{"render_device": "/dev/dri/renderD128"}
	caps.DeviceChain = []platform.GPUChoice{
		{Name: "Intel UHD Graphics", Vendor: "intel", Integrated: true, RenderDevice: "/dev/dri/renderD128"},
		{Name: "NVIDIA GeForce RTX 3050", Vendor: "nvidia"},
	}
	label, _, opts := resolvedAccelDeviceOptions(caps, gpus)
	if !strings.Contains(label, "renderD128") && !strings.Contains(label, "UHD") {
		t.Fatalf("auto label should prefer integrated chain device, got %q", label)
	}
	if len(opts) != 2 {
		t.Fatalf("want both GPUs listed, got %+v", opts)
	}
}
