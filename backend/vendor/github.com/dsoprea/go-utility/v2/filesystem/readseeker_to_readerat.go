package rifs

import (
	"io"

	"github.com/dsoprea/go-logging"
)

// ReadSeekerToReaderAt is a wrapper that allows a ReadSeeker to masquerade as a
// ReaderAt.
type ReadSeekerToReaderAt struct {
	rs io.ReadSeeker
}

// NewReadSeekerToReaderAt returns a new ReadSeekerToReaderAt instance.
func NewReadSeekerToReaderAt(rs io.ReadSeeker) *ReadSeekerToReaderAt {
	return &ReadSeekerToReaderAt{
		rs: rs,
	}
}

// ReadAt is a wrapper that satisfies the ReaderAt interface.
//
// Note that a requirement of ReadAt is that it doesn't have an effect on the
// offset in the underlying resource as well as that concurrent calls can be
// made to it. Since we're capturing the current offset in the underlying
// resource and then seeking back to it before returning, it is the
// responsibility of the caller to serialize (i.e. use a mutex with) these
// requests in order to eliminate race-conditions in the parallel-usage
// scenario.
//
// Note also that, since ReadAt() is going to be called on a particular
// instance, that instance is going to internalize a file resource, that file-
// resource is provided by the OS, and [most] OSs are only gonna support one
// file-position per resource, locking is already going to be a necessary
// internal semantic of a ReaderAt implementation.
func (rstra *ReadSeekerToReaderAt) ReadAt(p []byte, offset int64) (n int, err error) {
	defer func() {
		if state := recover(); state != nil {
			err = log.Wrap(state.(error))
		}
	}()

	originalOffset, err := rstra.rs.Seek(0, io.SeekCurrent)
	log.PanicIf(err)

	defer func() {
		_, err := rstra.rs.Seek(originalOffset, io.SeekStart)
		log.PanicIf(err)
	}()

	_, err = rstra.rs.Seek(offset, io.SeekStart)
	log.PanicIf(err)

	// Note that all errors will be wrapped, here. The usage of this method is
	// such that typically no specific errors would be expected as part of
	// normal operation (in which case we'd check for those first and return
	// them directly).
	n, err = io.ReadFull(rstra.rs, p)
	log.PanicIf(err)

	return n, nil
}
