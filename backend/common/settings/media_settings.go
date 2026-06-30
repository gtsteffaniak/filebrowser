package settings

const defaultMediaMaxConcurrent = 2

func normalizeMediaLimits() {
	m := &Config.Integrations.Media
	if m.MaxConcurrent <= 0 {
		m.MaxConcurrent = defaultMediaMaxConcurrent
	}
}

// MediaMaxConcurrent returns the system-wide concurrent media job limit.
func MediaMaxConcurrent() int {
	n := Config.Integrations.Media.MaxConcurrent
	if n <= 0 {
		return defaultMediaMaxConcurrent
	}
	return n
}
