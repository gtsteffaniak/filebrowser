package rifs

import (
	"errors"
	"io"
	"os"

	"github.com/dsoprea/go-logging"
)

var (
	// ErrSeekBeyondBound is returned when a seek is requested beyond the
	// statically-given file-size. No writes or seeks beyond boundaries are
	// supported with a statically-given file size.
	ErrSeekBeyondBound = errors.New("seek beyond boundary")
)

// BoundedReadWriteSeeker is a thin filter that ensures that no seeks can be done
// to offsets smaller than the one we were given. This supports libraries that
// might be expecting to read from the front of the stream being used on data
// that is in the middle of a stream instead.
type BoundedReadWriteSeeker struct {
	io.ReadWriteSeeker

	currentOffset int64
	minimumOffset int64

	staticFileSize int64
}

// NewBoundedReadWriteSeeker returns a new BoundedReadWriteSeeker instance.
func NewBoundedReadWriteSeeker(rws io.ReadWriteSeeker, minimumOffset int64, staticFileSize int64) (brws *BoundedReadWriteSeeker, err error) {
	defer func() {
		if state := recover(); state != nil {
			err = log.Wrap(state.(error))
		}
	}()

	if minimumOffset < 0 {
		log.Panicf("BoundedReadWriteSeeker minimum offset must be zero or larger: (%d)", minimumOffset)
	}

	// We'll always started at a relative offset of zero.
	_, err = rws.Seek(minimumOffset, os.SEEK_SET)
	log.PanicIf(err)

	brws = &BoundedReadWriteSeeker{
		ReadWriteSeeker: rws,

		currentOffset: 0,
		minimumOffset: minimumOffset,

		staticFileSize: staticFileSize,
	}

	return brws, nil
}

// Seek moves the offset to the given offset. Prevents offset from ever being
// moved left of `brws.minimumOffset`.
func (brws *BoundedReadWriteSeeker) Seek(offset int64, whence int) (updatedOffset int64, err error) {
	defer func() {
		if state := recover(); state != nil {
			err = log.Wrap(state.(error))
		}
	}()

	fileSize := brws.staticFileSize

	// If we weren't given a static file-size, look it up whenever it is needed.
	if whence == os.SEEK_END && fileSize == 0 {
		realFileSizeRaw, err := brws.ReadWriteSeeker.Seek(0, os.SEEK_END)
		log.PanicIf(err)

		fileSize = realFileSizeRaw - brws.minimumOffset
	}

	updatedOffset, err = CalculateSeek(brws.currentOffset, offset, whence, fileSize)
	log.PanicIf(err)

	if brws.staticFileSize != 0 && updatedOffset > brws.staticFileSize {
		//updatedOffset = int64(brws.staticFileSize)

		// NOTE(dustin): Presumably, this will only be disruptive to writes that are beyond the boundaries, which, if we're being used at all, should already account for the boundary and prevent this error from ever happening. So, time will tell how disruptive this is.
		return 0, ErrSeekBeyondBound
	}

	if updatedOffset != brws.currentOffset {
		updatedRealOffset := updatedOffset + brws.minimumOffset

		_, err = brws.ReadWriteSeeker.Seek(updatedRealOffset, os.SEEK_SET)
		log.PanicIf(err)

		brws.currentOffset = updatedOffset
	}

	return updatedOffset, nil
}

// Read forwards writes to the inner RWS.
func (brws *BoundedReadWriteSeeker) Read(buffer []byte) (readCount int, err error) {
	defer func() {
		if state := recover(); state != nil {
			err = log.Wrap(state.(error))
		}
	}()

	if brws.staticFileSize != 0 {
		availableCount := brws.staticFileSize - brws.currentOffset
		if availableCount == 0 {
			return 0, io.EOF
		}

		if int64(len(buffer)) > availableCount {
			buffer = buffer[:availableCount]
		}
	}

	readCount, err = brws.ReadWriteSeeker.Read(buffer)
	brws.currentOffset += int64(readCount)

	if err != nil {
		if err == io.EOF {
			return 0, err
		}

		log.Panic(err)
	}

	return readCount, nil
}

// Write forwards writes to the inner RWS.
func (brws *BoundedReadWriteSeeker) Write(buffer []byte) (writtenCount int, err error) {
	defer func() {
		if state := recover(); state != nil {
			err = log.Wrap(state.(error))
		}
	}()

	if brws.staticFileSize != 0 {
		log.Panicf("writes can not be performed if a static file-size was given")
	}

	writtenCount, err = brws.ReadWriteSeeker.Write(buffer)
	brws.currentOffset += int64(writtenCount)

	log.PanicIf(err)

	return writtenCount, nil
}

// MinimumOffset returns the configured minimum-offset.
func (brws *BoundedReadWriteSeeker) MinimumOffset() int64 {
	return brws.minimumOffset
}
