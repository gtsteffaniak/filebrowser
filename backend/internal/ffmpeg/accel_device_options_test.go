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
	if len(opts) != 1 || opts[0].ID != "software" {
		t.Fatalf("without capabilities want software only, got %+v", opts)
	}
	for _, o := range opts {
		if o.ID == "auto" {
			t.Fatalf("auto must not be a selectable option: %+v", opts)
		}
	}
	if got := s.AutoAccelDevice(); got != "" {
		t.Fatalf("AutoAccelDevice without capabilities = %q, want empty", got)
	}
}

// The option list never contains an "auto" id, and the device auto resolves to
// is listed as a real option so the dropdown can show it.
func TestAutoDeviceIsListedAsOption(t *testing.T) {
	gpus := []platform.GPUDevice{
		{Name: "AMD Radeon Graphics", Vendor: "amd", DeviceClass: "igpu", Integrated: true, RenderNode: "/dev/dri/renderD128"},
		{Name: "NVIDIA GeForce RTX 3050", Vendor: "nvidia", DeviceClass: "dgpu", RenderNode: "/dev/dri/renderD129"},
	}
	caps := capabilities.NewCapabilities()
	caps.DeviceChain = []platform.GPUChoice{
		{Name: "AMD Radeon Graphics", Vendor: "amd", Integrated: true, RenderDevice: "/dev/dri/renderD128"},
	}
	label, autoID, opts := resolvedAccelDeviceOptions(caps, gpus)
	if autoID != "/dev/dri/renderD128" {
		t.Fatalf("autoID = %q, want the integrated render node", autoID)
	}
	if label == "" {
		t.Fatal("auto label must be set")
	}
	found := false
	for _, o := range opts {
		if o.ID == "auto" {
			t.Fatalf("auto must not be an option: %+v", opts)
		}
		if o.ID == autoID {
			found = true
		}
	}
	if !found {
		t.Fatalf("auto device %q missing from options %+v", autoID, opts)
	}
}

// When auto resolves to a device that was not enumerated, it is prepended so the
// dropdown can still select it.
func TestAutoDevicePrependedWhenNotEnumerated(t *testing.T) {
	_, autoID, opts := resolvedAccelDeviceOptions(wslCaps(), nil)
	if autoID != "NVIDIA GeForce RTX 3050" {
		t.Fatalf("autoID = %q, want the WSL NVIDIA name", autoID)
	}
	if len(opts) != 1 || opts[0].ID != autoID {
		t.Fatalf("want the WSL device prepended, got %+v", opts)
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
	if id != "NVIDIA GeForce RTX 3050" || label != "Dedicated GPU (NVIDIA GeForce RTX 3050)" {
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
