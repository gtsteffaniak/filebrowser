package exif

import (
	"io"

	"github.com/dsoprea/go-logging"
	"github.com/dsoprea/go-utility/v2/filesystem"
)

type ExifBlobSeeker interface {
	GetReadSeeker(initialOffset int64) (rs io.ReadSeeker, err error)
}

// ExifReadSeeker knows how to retrieve data from the EXIF blob relative to the
// beginning of the blob (so, absolute position (0) is the first byte of the
// EXIF data).
type ExifReadSeeker struct {
	rs io.ReadSeeker
}

func NewExifReadSeeker(rs io.ReadSeeker) *ExifReadSeeker {
	return &ExifReadSeeker{
		rs: rs,
	}
}

func NewExifReadSeekerWithBytes(exifData []byte) *ExifReadSeeker {
	sb := rifs.NewSeekableBufferWithBytes(exifData)
	edbs := NewExifReadSeeker(sb)

	return edbs
}

// Fork creates a new ReadSeeker instead that wraps a BouncebackReader to
// maintain its own position in the stream.
func (edbs *ExifReadSeeker) GetReadSeeker(initialOffset int64) (rs io.ReadSeeker, err error) {
	defer func() {
		if state := recover(); state != nil {
			err = log.Wrap(state.(error))
		}
	}()

	br, err := rifs.NewBouncebackReader(edbs.rs)
	log.PanicIf(err)

	_, err = br.Seek(initialOffset, io.SeekStart)
	log.PanicIf(err)

	return br, nil
}
