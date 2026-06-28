package ffmpeg

import "testing"

func TestParseHLSTranscodeProfile(t *testing.T) {
	t.Parallel()
	tests := []struct {
		raw  string
		want HLSTranscodeProfile
	}{
		{"", HLSProfileQuality},
		{"quality", HLSProfileQuality},
		{"QUALITY", HLSProfileQuality},
		{"  quality  ", HLSProfileQuality},
		{"optimized", HLSProfileOptimized},
		{"OPTIMIZED", HLSProfileOptimized},
		{"datasaver", HLSProfileDataSaver},
		{"data-saver", HLSProfileDataSaver},
		{"data_saver", HLSProfileDataSaver},
		{" Data-Saver ", HLSProfileDataSaver},
		{"unknown", HLSProfileQuality},
		{"balanced", HLSProfileQuality},
	}
	for _, tc := range tests {
		got := ParseHLSTranscodeProfile(tc.raw)
		if got != tc.want {
			t.Errorf("ParseHLSTranscodeProfile(%q) = %q, want %q", tc.raw, got, tc.want)
		}
	}
}
