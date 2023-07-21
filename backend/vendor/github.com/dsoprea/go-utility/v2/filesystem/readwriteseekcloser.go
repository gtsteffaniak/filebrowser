package rifs

import (
	"io"
)

// ReadWriteSeekCloser satisfies `io.ReadWriteSeeker` and `io.Closer`
// interfaces.
type ReadWriteSeekCloser interface {
	io.ReadWriteSeeker
	io.Closer
}

type readWriteSeekNoopCloser struct {
	io.ReadWriteSeeker
}

// ReadWriteSeekNoopCloser wraps a `io.ReadWriteSeeker` with a no-op Close()
// call.
func ReadWriteSeekNoopCloser(rws io.ReadWriteSeeker) ReadWriteSeekCloser {
	return readWriteSeekNoopCloser{
		ReadWriteSeeker: rws,
	}
}

// Close does nothing but allows the RWS to satisfy `io.Closer`.:wq
func (readWriteSeekNoopCloser) Close() (err error) {
	return nil
}
