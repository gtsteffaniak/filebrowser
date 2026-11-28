package preview

import (
	"context"
	"sync"
	"sync/atomic"
	"testing"
	"time"

	"github.com/stretchr/testify/require"
)

func TestVideoPreviewJobsCancelOnlyTheLeavingViewer(t *testing.T) {
	t.Parallel()

	var jobs videoPreviewJobs
	var generated atomic.Int32
	ctx, cancel := context.WithCancel(t.Context())
	defer cancel()
	started := make(chan context.Context, 2)
	release := make(chan struct{})
	generate := func(work context.Context) ([]byte, error) {
		generated.Add(1)
		started <- work
		select {
		case <-work.Done():
			return nil, work.Err()
		case <-release:
			return []byte("thumbnail"), nil
		}
	}

	first := make(chan error, 1)
	go func() {
		_, err := jobs.run(ctx, "thumbnail", generate)
		first <- err
	}()
	var work context.Context
	select {
	case work = <-started:
	case <-time.After(2 * time.Second):
		t.Fatal("generation did not start")
	}

	type previewResult struct {
		data []byte
		err  error
	}
	second := make(chan previewResult, 1)
	go func() {
		data, err := jobs.run(t.Context(), "thumbnail", generate)
		second <- previewResult{data: data, err: err}
	}()
	require.Eventually(t, func() bool {
		jobs.mu.Lock()
		defer jobs.mu.Unlock()
		job := jobs.active["thumbnail"]
		return job != nil && job.waiters == 2
	}, 2*time.Second, time.Millisecond)

	cancel()
	select {
	case err := <-first:
		require.ErrorIs(t, err, context.Canceled)
	case <-time.After(2 * time.Second):
		t.Fatal("canceled viewer did not return")
	}
	require.NoError(t, work.Err())
	close(release)
	select {
	case result := <-second:
		require.NoError(t, result.err)
		require.Equal(t, []byte("thumbnail"), result.data)
	case <-time.After(2 * time.Second):
		t.Fatal("remaining viewer did not receive the thumbnail")
	}
	require.EqualValues(t, 1, generated.Load())
}

func TestVideoPreviewJobsRetryAfterLastViewerLeaves(t *testing.T) {
	t.Parallel()

	var jobs videoPreviewJobs
	ctx, cancel := context.WithCancel(t.Context())
	defer cancel()
	started := make(chan struct{})
	canceled := make(chan struct{})
	cleanup := make(chan struct{})
	finishCleanup := sync.OnceFunc(func() { close(cleanup) })
	t.Cleanup(finishCleanup)
	result := make(chan error, 1)
	go func() {
		_, err := jobs.run(ctx, "thumbnail", func(work context.Context) ([]byte, error) {
			close(started)
			<-work.Done()
			close(canceled)
			<-cleanup
			return nil, work.Err()
		})
		result <- err
	}()
	select {
	case <-started:
	case <-time.After(2 * time.Second):
		t.Fatal("generation did not start")
	}
	cancel()
	select {
	case err := <-result:
		require.ErrorIs(t, err, context.Canceled)
	case <-time.After(2 * time.Second):
		t.Fatal("canceled viewer did not return")
	}
	select {
	case <-canceled:
	case <-time.After(2 * time.Second):
		t.Fatal("work was not canceled when the last viewer left")
	}

	jobs.mu.Lock()
	abandoned := jobs.active["thumbnail"]
	jobs.mu.Unlock()
	require.NotNil(t, abandoned)

	request, stop := context.WithTimeout(t.Context(), 2*time.Second)
	defer stop()
	waiting := make(chan bool, 1)
	replacement := doneNotifyingContext{
		Context: request,
		onDone: sync.OnceFunc(func() {
			// Capture the job state when run evaluates the replacement's wait select.
			jobs.mu.Lock()
			defer jobs.mu.Unlock()
			waiting <- jobs.active["thumbnail"] == abandoned && abandoned.waiters == 0
		}),
	}
	retried := make(chan error, 1)
	go func() {
		_, err := jobs.run(replacement, "thumbnail", func(context.Context) ([]byte, error) {
			select {
			case <-cleanup:
				return []byte("new thumbnail"), nil
			default:
				return nil, context.DeadlineExceeded
			}
		})
		retried <- err
	}()
	select {
	case blocked := <-waiting:
		require.True(t, blocked, "replacement did not wait for the abandoned job")
	case err := <-retried:
		t.Fatalf("replacement completed before cleanup: %v", err)
	case <-request.Done():
		t.Fatal("replacement did not reach the wait for cleanup")
	}
	finishCleanup()
	select {
	case err := <-retried:
		require.NoError(t, err)
	case <-request.Done():
		t.Fatal("replacement did not finish after cleanup")
	}
}

type doneNotifyingContext struct {
	context.Context
	onDone func()
}

func (c doneNotifyingContext) Done() <-chan struct{} {
	c.onDone()
	return c.Context.Done()
}
