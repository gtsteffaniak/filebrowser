package settings

import "testing"

func TestTranscodeDataSaverBitrateKbps(t *testing.T) {
	t.Parallel()
	prev := Config.Integrations.Media.Transcode.DataSaverBitrateKbps
	t.Cleanup(func() {
		Config.Integrations.Media.Transcode.DataSaverBitrateKbps = prev
	})

	Config.Integrations.Media.Transcode.DataSaverBitrateKbps = 0
	if got := TranscodeDataSaverBitrateKbps(); got != defaultTranscodeDataSaverBitrateKbps {
		t.Fatalf("default = %d, want %d", got, defaultTranscodeDataSaverBitrateKbps)
	}

	Config.Integrations.Media.Transcode.DataSaverBitrateKbps = 1100
	if got := TranscodeDataSaverBitrateKbps(); got != 1100 {
		t.Fatalf("custom = %d, want 1100", got)
	}
}

func TestNormalizeMediaTranscode(t *testing.T) {
	t.Parallel()
	prev := Config.Integrations.Media.Transcode
	t.Cleanup(func() {
		Config.Integrations.Media.Transcode = prev
	})

	Config.Integrations.Media.Transcode = MediaTranscode{}
	normalizeMediaTranscode()
	if Config.Integrations.Media.Transcode.DataSaverBitrateKbps != defaultTranscodeDataSaverBitrateKbps {
		t.Fatalf("dataSaverBitrateKbps = %d", Config.Integrations.Media.Transcode.DataSaverBitrateKbps)
	}
}
