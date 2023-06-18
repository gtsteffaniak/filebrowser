package rifs

import (
	"io"
)

// WriteCounter proxies write requests and maintains a counter of bytes written.
type WriteCounter struct {
	w       io.Writer
	counter int
}

// NewWriteCounter returns a new `WriteCounter` struct wrapping a `Writer`.
func NewWriteCounter(w io.Writer) *WriteCounter {
	return &WriteCounter{
		w: w,
	}
}

// Count returns the total number of bytes read.
func (wc *WriteCounter) Count() int {
	return wc.counter
}

// Reset resets the counter to zero.
func (wc *WriteCounter) Reset() {
	wc.counter = 0
}

// Write forwards a write to the underlying `Writer` while bumping the counter.
func (wc *WriteCounter) Write(b []byte) (n int, err error) {
	n, err = wc.w.Write(b)
	wc.counter += n

	return n, err
}
