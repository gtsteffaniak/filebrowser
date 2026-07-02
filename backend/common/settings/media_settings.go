package settings

const defaultMediaMaxConcurrent = 2

func normalizeMediaLimits() {
	m := &Config.Integrations.Media
	if m.MaxConcurrent <= 0 {
		m.MaxConcurrent = defaultMediaMaxConcurrent
	}
}

// MediaMaxConcurrent returns configured transcode ffmpeg pool size (unused; config compatibility only).
func MediaMaxConcurrent() int {
	n := Config.Integrations.Media.MaxConcurrent
	if n <= 0 {
		return defaultMediaMaxConcurrent
	}
	return n
}
