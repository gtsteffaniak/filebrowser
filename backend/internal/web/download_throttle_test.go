package web

import (
	"bytes"
	"context"
	"io"
	"strings"
	"testing"
	"time"

	"golang.org/x/time/rate"
)

type fixedReadSeeker struct {
	data []byte
	off  int64
}

func (f *fixedReadSeeker) Read(p []byte) (int, error) {
	if f.off >= int64(len(f.data)) {
		return 0, io.EOF
	}
	n := copy(p, f.data[f.off:])
	f.off += int64(n)
	return n, nil
}

func (f *fixedReadSeeker) Seek(offset int64, whence int) (int64, error) {
	var base int64
	switch whence {
	case io.SeekStart:
		base = 0
	case io.SeekCurrent:
		base = f.off
	case io.SeekEnd:
		base = int64(len(f.data))
	default:
		return 0, io.ErrUnexpectedEOF
	}
	f.off = base + offset
	if f.off < 0 {
		f.off = 0
	}
	return f.off, nil
}

func TestThrottledReadSeekerChunksLargeRead(t *testing.T) {
	t.Parallel()
	const burst = 1024
	const readSize = 32 << 10
	payload := bytes.Repeat([]byte("x"), readSize)
	rs := &fixedReadSeeker{data: payload}
	ctx, cancel := context.WithTimeout(context.Background(), 2*time.Second)
	defer cancel()
	tr := NewThrottledReadSeeker(rs, rate.Inf, burst, ctx)

	buf := make([]byte, readSize)
	n, err := tr.Read(buf)
	if err != nil {
		t.Fatalf("Read: %v", err)
	}
	if n != readSize {
		t.Fatalf("Read n = %d, want %d", n, readSize)
	}
}

func TestWaitLimiterBytesRejectsCancelledContext(t *testing.T) {
	t.Parallel()
	lim := rate.NewLimiter(1, 1024)
	ctx, cancel := context.WithCancel(context.Background())
	cancel()
	err := waitLimiterBytes(ctx, lim, 4096)
	if err == nil {
		t.Fatal("expected context error")
	}
	if !strings.Contains(err.Error(), "context") {
		t.Fatalf("unexpected error: %v", err)
	}
}
