package rifs

import (
	"io"

	"github.com/dsoprea/go-logging"
)

// BoundedReadWriteSeekCloser wraps a RWS that is also a closer with boundaries.
// This proxies the RWS methods to the inner BRWS inside.
type BoundedReadWriteSeekCloser struct {
	io.Closer
	*BoundedReadWriteSeeker
}

// NewBoundedReadWriteSeekCloser returns a new BoundedReadWriteSeekCloser.
func NewBoundedReadWriteSeekCloser(rwsc ReadWriteSeekCloser, minimumOffset int64, staticFileSize int64) (brwsc *BoundedReadWriteSeekCloser, err error) {
	defer func() {
		if state := recover(); state != nil {
			err = log.Wrap(state.(error))
		}
	}()

	bs, err := NewBoundedReadWriteSeeker(rwsc, minimumOffset, staticFileSize)
	log.PanicIf(err)

	brwsc = &BoundedReadWriteSeekCloser{
		Closer:                 rwsc,
		BoundedReadWriteSeeker: bs,
	}

	return brwsc, nil
}

// Seek forwards calls to the inner RWS.
func (rwsc *BoundedReadWriteSeekCloser) Seek(offset int64, whence int) (newOffset int64, err error) {
	defer func() {
		if state := recover(); state != nil {
			err = log.Wrap(state.(error))
		}
	}()

	newOffset, err = rwsc.BoundedReadWriteSeeker.Seek(offset, whence)
	log.PanicIf(err)

	return newOffset, nil
}

// Read forwards calls to the inner RWS.
func (rwsc *BoundedReadWriteSeekCloser) Read(buffer []byte) (readCount int, err error) {
	defer func() {
		if state := recover(); state != nil {
			err = log.Wrap(state.(error))
		}
	}()

	readCount, err = rwsc.BoundedReadWriteSeeker.Read(buffer)
	if err != nil {
		if err == io.EOF {
			return 0, err
		}

		log.Panic(err)
	}

	return readCount, nil
}

// Write forwards calls to the inner RWS.
func (rwsc *BoundedReadWriteSeekCloser) Write(buffer []byte) (writtenCount int, err error) {
	defer func() {
		if state := recover(); state != nil {
			err = log.Wrap(state.(error))
		}
	}()

	writtenCount, err = rwsc.BoundedReadWriteSeeker.Write(buffer)
	log.PanicIf(err)

	return writtenCount, nil
}

// Close forwards calls to the inner RWS.
func (rwsc *BoundedReadWriteSeekCloser) Close() (err error) {
	defer func() {
		if state := recover(); state != nil {
			err = log.Wrap(state.(error))
		}
	}()

	err = rwsc.Closer.Close()
	log.PanicIf(err)

	return nil
}
