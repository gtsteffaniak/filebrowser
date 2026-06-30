package settings

import "testing"

func TestMediaMaxConcurrent(t *testing.T) {
	t.Parallel()
	prev := Config.Integrations.Media.MaxConcurrent
	t.Cleanup(func() {
		Config.Integrations.Media.MaxConcurrent = prev
	})

	Config.Integrations.Media.MaxConcurrent = 0
	if got := MediaMaxConcurrent(); got != defaultMediaMaxConcurrent {
		t.Fatalf("default = %d, want %d", got, defaultMediaMaxConcurrent)
	}

	Config.Integrations.Media.MaxConcurrent = 4
	if got := MediaMaxConcurrent(); got != 4 {
		t.Fatalf("custom = %d, want 4", got)
	}
}

func TestNormalizeMediaLimits(t *testing.T) {
	t.Parallel()
	prev := Config.Integrations.Media.MaxConcurrent
	t.Cleanup(func() {
		Config.Integrations.Media.MaxConcurrent = prev
	})

	Config.Integrations.Media.MaxConcurrent = 0
	normalizeMediaLimits()
	if Config.Integrations.Media.MaxConcurrent != defaultMediaMaxConcurrent {
		t.Fatalf("maxConcurrent = %d", Config.Integrations.Media.MaxConcurrent)
	}
}
