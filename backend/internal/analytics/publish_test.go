//go:build analytics

package analytics

import (
	"context"
	"path/filepath"
	"testing"
	"time"

	"github.com/gtsteffaniak/filebrowser/backend/internal/state"
	"github.com/gtsteffaniak/filebrowser/backend/pkg/settings"
)

func TestWaitUntilPastDeadline(t *testing.T) {
	ctx := context.Background()
	if !waitUntil(ctx, time.Now().Add(-time.Second)) {
		t.Fatal("expected waitUntil to succeed for past deadline")
	}
}

func TestWaitUntilCancelled(t *testing.T) {
	ctx, cancel := context.WithCancel(context.Background())
	cancel()

	if waitUntil(ctx, time.Now().Add(time.Hour)) {
		t.Fatal("expected waitUntil to fail when context is cancelled")
	}
}

func TestWaitUntilFutureDeadline(t *testing.T) {
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	done := make(chan bool, 1)
	go func() {
		done <- waitUntil(ctx, time.Now().Add(20*time.Millisecond))
	}()

	select {
	case ok := <-done:
		if !ok {
			t.Fatal("expected waitUntil to succeed before timeout")
		}
	case <-time.After(time.Second):
		t.Fatal("waitUntil did not return in time")
	}
}

func TestWaitForDuration(t *testing.T) {
	ctx := context.Background()
	start := time.Now()
	if !waitFor(ctx, 15*time.Millisecond) {
		t.Fatal("expected waitFor to succeed")
	}
	if time.Since(start) < 10*time.Millisecond {
		t.Fatal("waitFor returned too early")
	}
}

func TestWaitForCancelled(t *testing.T) {
	ctx, cancel := context.WithCancel(context.Background())
	cancel()

	if waitFor(ctx, time.Hour) {
		t.Fatal("expected waitFor to fail when context is cancelled")
	}
}

func setupPublishTest(t *testing.T) {
	t.Helper()
	t.Setenv("FILEBROWSER_ONLYOFFICE_SECRET", "")
	settings.Initialize("../../../_docker/src/noauth/backend/config.yaml")
	settings.Env.IsPlaywright = true

	dbPath := filepath.Join(t.TempDir(), "filebrowser.sqlite")
	if _, err := state.Initialize(dbPath); err != nil {
		t.Fatal(err)
	}
	if err := state.SetAnalyticsEnabled(true); err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() {
		NotifyAnalyticsDisabled()
		_ = state.Close()
	})
}

func TestNotifyAnalyticsDisabledCancelsReporter(t *testing.T) {
	setupPublishTest(t)

	NotifyAnalyticsEnabled()

	reporterMu.Lock()
	cancel := reporterCancel
	reporterMu.Unlock()
	if cancel == nil {
		t.Fatal("expected reporter cancel function after NotifyAnalyticsEnabled")
	}

	NotifyAnalyticsDisabled()

	reporterMu.Lock()
	cancelAfter := reporterCancel
	reporterMu.Unlock()
	if cancelAfter != nil {
		t.Fatal("expected reporter cancel cleared after NotifyAnalyticsDisabled")
	}
}

func TestScheduleReporterSkipsWhenDisabled(t *testing.T) {
	setupPublishTest(t)
	if err := state.SetAnalyticsEnabled(false); err != nil {
		t.Fatal(err)
	}

	scheduleReporter(time.Now())

	reporterMu.Lock()
	cancel := reporterCancel
	reporterMu.Unlock()
	if cancel != nil {
		t.Fatal("expected no reporter when analytics is disabled")
	}
}
