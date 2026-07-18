//go:build analytics

package analytics

import (
	"bytes"
	"context"
	"io"
	"net/http"
	"sync"
	"time"

	"github.com/gtsteffaniak/go-logger/logger"
)

const (
	time24h             = 24 * time.Hour
	time30d             = 30 * time24h
	initialSendDelay    = time24h
	recurringSendPeriod = time30d
	analyticsURL        = "https://api.filebrowserquantum.com/v1/events"
)

var (
	reporterMu     sync.Mutex
	reporterCancel context.CancelFunc
)

// PublishSupported reports whether this build can enable and send analytics.
func PublishSupported() bool {
	return true
}

// StartReporter schedules the first send 24 hours after startup when analytics is enabled,
// then sends monthly while the server keeps running.
func StartReporter() {
	if !PublishSupported() {
		return
	}
	scheduleReporter(time.Now())
}

// NotifyAnalyticsEnabled resets the reporter to send 24 hours from now, then monthly.
func NotifyAnalyticsEnabled() {
	if !PublishSupported() {
		return
	}
	scheduleReporter(time.Now())
}

// NotifyAnalyticsDisabled stops any pending or recurring analytics sends.
func NotifyAnalyticsDisabled() {
	reporterMu.Lock()
	defer reporterMu.Unlock()
	if reporterCancel != nil {
		reporterCancel()
		reporterCancel = nil
	}
}

func scheduleReporter(anchor time.Time) {
	reporterMu.Lock()
	defer reporterMu.Unlock()

	if reporterCancel != nil {
		reporterCancel()
		reporterCancel = nil
	}

	if !Enabled() {
		return
	}

	ctx, cancel := context.WithCancel(context.Background())
	reporterCancel = cancel

	go runReporter(ctx, anchor)
}

func runReporter(ctx context.Context, anchor time.Time) {
	if !waitUntil(ctx, anchor.Add(initialSendDelay)) {
		return
	}
	sendSnapshot()

	for {
		if !waitFor(ctx, recurringSendPeriod) {
			return
		}
		sendSnapshot()
	}
}

func waitUntil(ctx context.Context, deadline time.Time) bool {
	delay := time.Until(deadline)
	if delay <= 0 {
		select {
		case <-ctx.Done():
			return false
		default:
			return true
		}
	}

	timer := time.NewTimer(delay)
	defer timer.Stop()

	select {
	case <-ctx.Done():
		return false
	case <-timer.C:
		return true
	}
}

func waitFor(ctx context.Context, duration time.Duration) bool {
	timer := time.NewTimer(duration)
	defer timer.Stop()

	select {
	case <-ctx.Done():
		return false
	case <-timer.C:
		return true
	}
}

func sendSnapshot() {
	if !Enabled() || !PublishSupported() {
		return
	}
	body, err := PublishSnapshot()
	if err != nil {
		logger.Debugf("analytics snapshot send skipped: %v", err)
		return
	}

	req, err := http.NewRequest(http.MethodPost, analyticsURL, bytes.NewReader(body))
	if err != nil {
		logger.Debugf("analytics request failed: %v", err)
		return
	}
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("User-Agent", userAgent)
	req.Header.Set("X-App", xApp)

	client := &http.Client{Timeout: 15 * time.Second}
	resp, err := client.Do(req)
	if err != nil {
		logger.Debugf("analytics send failed: %v", err)
		return
	}
	defer resp.Body.Close()
	io.Copy(io.Discard, resp.Body)

	if resp.StatusCode != http.StatusCreated {
		logger.Debugf("analytics send returned status %d", resp.StatusCode)
		return
	}
	logger.Debug("analytics deployment snapshot sent")
}
