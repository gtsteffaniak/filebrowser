package rifs

import (
	"io"
)

// ReadCounter proxies read requests and maintains a counter of bytes read.
type ReadCounter struct {
	r       io.Reader
	counter int
}

// NewReadCounter returns a new `ReadCounter` struct wrapping a `Reader`.
func NewReadCounter(r io.Reader) *ReadCounter {
	return &ReadCounter{
		r: r,
	}
}

// Count returns the total number of bytes read.
func (rc *ReadCounter) Count() int {
	return rc.counter
}

// Reset resets the counter to zero.
func (rc *ReadCounter) Reset() {
	rc.counter = 0
}

// Read forwards a read to the underlying `Reader` while bumping the counter.
func (rc *ReadCounter) Read(b []byte) (n int, err error) {
	n, err = rc.r.Read(b)
	rc.counter += n

	return n, err
}
