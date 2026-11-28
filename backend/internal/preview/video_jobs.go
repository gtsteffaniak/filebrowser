package preview

import (
	"context"
	"fmt"
	"sync"
)

type videoPreviewJob struct {
	done    chan struct{}
	cancel  context.CancelFunc
	waiters int
	data    []byte
	err     error
}

type videoPreviewJobs struct {
	mu     sync.Mutex
	active map[string]*videoPreviewJob
}

// run shares a video thumbnail generation until its last viewer leaves. A new
// viewer waits for an abandoned job to clean up before starting a replacement.
func (g *videoPreviewJobs) run(ctx context.Context, key string, generate func(context.Context) ([]byte, error)) ([]byte, error) {
	var job *videoPreviewJob
	for {
		if err := ctx.Err(); err != nil {
			return nil, err
		}

		g.mu.Lock()
		job = g.active[key]
		if job != nil && job.waiters == 0 {
			g.mu.Unlock()
			select {
			case <-ctx.Done():
				return nil, ctx.Err()
			case <-job.done:
				continue
			}
		}

		if job == nil {
			work, cancel := context.WithCancel(context.WithoutCancel(ctx))
			job = &videoPreviewJob{done: make(chan struct{}), cancel: cancel}
			if g.active == nil {
				g.active = make(map[string]*videoPreviewJob)
			}
			g.active[key] = job
			go g.generate(work, key, job, generate)
		}
		job.waiters++
		g.mu.Unlock()
		break
	}

	defer func() {
		g.mu.Lock()
		defer g.mu.Unlock()
		job.waiters--
		if job.waiters == 0 {
			job.cancel()
		}
	}()
	select {
	case <-ctx.Done():
		return nil, ctx.Err()
	case <-job.done:
		return job.data, job.err
	}
}

func (g *videoPreviewJobs) generate(ctx context.Context, key string, job *videoPreviewJob, generate func(context.Context) ([]byte, error)) {
	defer func() {
		// The HTTP panic recovery cannot catch a panic in a shared worker.
		if recovered := recover(); recovered != nil {
			job.err = fmt.Errorf("video preview generation panicked: %v", recovered)
		}
		job.cancel()
		g.mu.Lock()
		defer g.mu.Unlock()
		delete(g.active, key)
		close(job.done)
	}()
	job.data, job.err = generate(ctx)
}
