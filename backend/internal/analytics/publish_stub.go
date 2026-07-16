//go:build !analytics

package analytics

// PublishSupported reports whether this build can enable and send analytics.
func PublishSupported() bool {
	return false
}

func StartReporter() {}

func NotifyAnalyticsEnabled() {}

func NotifyAnalyticsDisabled() {}
