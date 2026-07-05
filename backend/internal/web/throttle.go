package web

import (
	"context"
	"io"

	"golang.org/x/time/rate"
)

type throttledReadSeeker struct {
	rs      io.ReadSeeker
	limiter *rate.Limiter
	ctx     context.Context
}

// NewThrottledReadSeeker rate-limits reads from an io.ReadSeeker.
func NewThrottledReadSeeker(rs io.ReadSeeker, limit rate.Limit, burst int, ctx context.Context) io.ReadSeeker {
	return &throttledReadSeeker{
		rs:      rs,
		limiter: rate.NewLimiter(limit, burst),
		ctx:     ctx,
	}
}

func (r *throttledReadSeeker) Read(p []byte) (n int, err error) {
	n, err = r.rs.Read(p)
	if n > 0 {
		if waitErr := r.limiter.WaitN(r.ctx, n); waitErr != nil && err == nil {
			err = waitErr
		}
	}
	return
}

func (r *throttledReadSeeker) Seek(offset int64, whence int) (int64, error) {
	return r.rs.Seek(offset, whence)
}
