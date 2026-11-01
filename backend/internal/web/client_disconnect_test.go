package web

import (
	"context"
	"errors"
	"fmt"
	"net"
	"net/http"
	"os"
	"runtime"
	"syscall"
	"testing"
)

func TestIsClientDisconnect(t *testing.T) {
	tests := []struct {
		name string
		req  *http.Request
		err  error
		want bool
	}{
		{
			name: "nil error is not a disconnect",
			err:  nil,
			want: false,
		},
		{
			name: "cancelled request context is a disconnect",
			req:  newRequestWithCancelledContext(),
			err:  errors.New("some write error"),
			want: true,
		},
		{
			name: "context.Canceled is a disconnect",
			err:  fmt.Errorf("write failed: %w", context.Canceled),
			want: true,
		},
		{
			name: "closed connection is a disconnect",
			err:  fmt.Errorf("write failed: %w", net.ErrClosed),
			want: true,
		},
		{
			name: "wrapped deadline exceeded is a disconnect",
			err:  fmt.Errorf("write failed: %w", os.ErrDeadlineExceeded),
			want: true,
		},
		{
			name: "wrapped platform connection reset is a disconnect",
			err:  fmt.Errorf("write tcp: %w", platformConnResetErrno()),
			want: true,
		},
		{
			name: "unrelated write error is not a disconnect",
			err:  errors.New("disk full"),
			want: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := isClientDisconnect(tt.req, tt.err); got != tt.want {
				t.Fatalf("isClientDisconnect() = %t, want %t", got, tt.want)
			}
		})
	}
}

// platformConnResetErrno returns the errno a client disconnect surfaces as on the
// running platform.
func platformConnResetErrno() syscall.Errno {
	if runtime.GOOS == "windows" {
		return errnoWSAConnAborted
	}
	return errnoConnReset
}

func newRequestWithCancelledContext() *http.Request {
	req, err := http.NewRequestWithContext(context.Background(), http.MethodGet, "/", nil)
	if err != nil {
		panic(err)
	}
	ctx, cancel := context.WithCancel(req.Context())
	cancel()
	return req.WithContext(ctx)
}
