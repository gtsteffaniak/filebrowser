package ffmpeg

import (
	"testing"

	"github.com/gtsteffaniak/go-ffmpeg/capabilities"
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
