package ffmpeg

import "testing"

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
