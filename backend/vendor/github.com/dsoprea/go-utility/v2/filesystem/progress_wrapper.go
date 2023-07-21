package rifs

import (
	"io"
	"time"

	"github.com/dsoprea/go-logging"
)

// ProgressFunc receives progress updates.
type ProgressFunc func(n int, duration time.Duration, isEof bool) error

// WriteProgressWrapper wraps a reader and calls a callback after each read with
// count and duration info.
type WriteProgressWrapper struct {
	w          io.Writer
	progressCb ProgressFunc
}

// NewWriteProgressWrapper returns a new WPW instance.
func NewWriteProgressWrapper(w io.Writer, progressCb ProgressFunc) io.Writer {
	return &WriteProgressWrapper{
		w:          w,
		progressCb: progressCb,
	}
}

// Write does a write and calls the callback.
func (wpw *WriteProgressWrapper) Write(buffer []byte) (n int, err error) {
	defer func() {
		if state := recover(); state != nil {
			err = log.Wrap(state.(error))
		}
	}()

	startAt := time.Now()

	n, err = wpw.w.Write(buffer)
	log.PanicIf(err)

	duration := time.Since(startAt)

	err = wpw.progressCb(n, duration, false)
	log.PanicIf(err)

	return n, nil
}

// ReadProgressWrapper wraps a reader and calls a callback after each read with
// count and duration info.
type ReadProgressWrapper struct {
	r          io.Reader
	progressCb ProgressFunc
}

// NewReadProgressWrapper returns a new RPW instance.
func NewReadProgressWrapper(r io.Reader, progressCb ProgressFunc) io.Reader {
	return &ReadProgressWrapper{
		r:          r,
		progressCb: progressCb,
	}
}

// Read reads data and calls the callback.
func (rpw *ReadProgressWrapper) Read(buffer []byte) (n int, err error) {
	defer func() {
		if state := recover(); state != nil {
			err = log.Wrap(state.(error))
		}
	}()

	startAt := time.Now()

	n, err = rpw.r.Read(buffer)

	duration := time.Since(startAt)

	if err != nil {
		if err == io.EOF {
			errInner := rpw.progressCb(n, duration, true)
			log.PanicIf(errInner)

			return n, err
		}

		log.Panic(err)
	}

	err = rpw.progressCb(n, duration, false)
	log.PanicIf(err)

	return n, nil
}
