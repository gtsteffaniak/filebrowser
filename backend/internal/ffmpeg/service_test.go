package ffmpeg

import (
	"strings"
	"testing"

	"github.com/gtsteffaniak/go-ffmpeg/capabilities"
)

func TestHardwareCodecSummaryFromCaps(t *testing.T) {
	t.Run("reports detected hardware independent of the preference hierarchy", func(t *testing.T) {
		caps := capabilities.NewCapabilities()
		caps.EncoderHierarchy = []capabilities.AccelType{capabilities.AccelNone}
		caps.Encoders["libx264"] = capabilities.EncoderCapability{Name: "libx264", Kind: "software", Compiled: true, Available: true}
		caps.Encoders["h264_videotoolbox"] = capabilities.EncoderCapability{Name: "h264_videotoolbox", Kind: "videotoolbox", Compiled: true, Available: true}
		caps.Encoders["hevc_videotoolbox"] = capabilities.EncoderCapability{Name: "hevc_videotoolbox", Kind: "videotoolbox", Compiled: true, Available: true}
		caps.Decoders["hwaccel:videotoolbox:h264"] = capabilities.DecoderCapability{
			Name: "hwaccel:videotoolbox:h264", Kind: "videotoolbox", Codec: capabilities.CodecH264, Compiled: true, Available: true,
		}

		got := hardwareCodecSummaryFromCaps(caps)
		if !strings.Contains(got, "h264 encode via h264_videotoolbox (Apple VideoToolbox)") {
			t.Fatalf("summary = %q, want videotoolbox encode entry", got)
		}
		if !strings.Contains(got, "h264 decode via hwaccel:videotoolbox:h264 (Apple VideoToolbox)") {
			t.Fatalf("summary = %q, want videotoolbox decode entry", got)
		}
		if strings.Contains(got, "libx264") {
			t.Fatalf("summary = %q, must not list software encoders", got)
		}
	})

	t.Run("skips hardware that failed detection", func(t *testing.T) {
		caps := capabilities.NewCapabilities()
		caps.Encoders["h264_nvenc"] = capabilities.EncoderCapability{Name: "h264_nvenc", Kind: "nvenc", Compiled: true, Available: false}
		if got := hardwareCodecSummaryFromCaps(caps); got != "" {
			t.Fatalf("summary = %q, want empty when hardware is unavailable", got)
		}
	})

	t.Run("empty when only software is present", func(t *testing.T) {
		caps := capabilities.NewCapabilities()
		caps.Encoders["libx264"] = capabilities.EncoderCapability{Name: "libx264", Kind: "software", Compiled: true, Available: true}
		caps.Decoders["h264"] = capabilities.DecoderCapability{Name: "h264", Kind: "software", Compiled: true, Available: true}
		if got := hardwareCodecSummaryFromCaps(caps); got != "" {
			t.Fatalf("summary = %q, want empty", got)
		}
	})

	t.Run("nil capabilities", func(t *testing.T) {
		if got := hardwareCodecSummaryFromCaps(nil); got != "" {
			t.Fatalf("summary = %q, want empty", got)
		}
	})
}
