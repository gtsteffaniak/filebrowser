package rifs

import (
	"io"
	"os"

	"github.com/dsoprea/go-logging"
)

// SeekableBuffer is a simple memory structure that satisfies
// `io.ReadWriteSeeker`.
type SeekableBuffer struct {
	data     []byte
	position int64
}

// NewSeekableBuffer is a factory that returns a `*SeekableBuffer`.
func NewSeekableBuffer() *SeekableBuffer {
	data := make([]byte, 0)

	return &SeekableBuffer{
		data: data,
	}
}

// NewSeekableBufferWithBytes is a factory that returns a `*SeekableBuffer`.
func NewSeekableBufferWithBytes(originalData []byte) *SeekableBuffer {
	data := make([]byte, len(originalData))
	copy(data, originalData)

	return &SeekableBuffer{
		data: data,
	}
}

func len64(data []byte) int64 {
	return int64(len(data))
}

// Bytes returns the underlying slice.
func (sb *SeekableBuffer) Bytes() []byte {
	return sb.data
}

// Len returns the number of bytes currently stored.
func (sb *SeekableBuffer) Len() int {
	return len(sb.data)
}

// Write does a standard write to the internal slice.
func (sb *SeekableBuffer) Write(p []byte) (n int, err error) {
	defer func() {
		if state := recover(); state != nil {
			err = log.Wrap(state.(error))
		}
	}()

	// The current position we're already at is past the end of the data we
	// actually have. Extend our buffer up to our current position.
	if sb.position > len64(sb.data) {
		extra := make([]byte, sb.position-len64(sb.data))
		sb.data = append(sb.data, extra...)
	}

	positionFromEnd := len64(sb.data) - sb.position
	tailCount := positionFromEnd - len64(p)

	var tailBytes []byte
	if tailCount > 0 {
		tailBytes = sb.data[len64(sb.data)-tailCount:]
		sb.data = append(sb.data[:sb.position], p...)
	} else {
		sb.data = append(sb.data[:sb.position], p...)
	}

	if tailBytes != nil {
		sb.data = append(sb.data, tailBytes...)
	}

	dataSize := len64(p)
	sb.position += dataSize

	return int(dataSize), nil
}

// Read does a standard read against the internal slice.
func (sb *SeekableBuffer) Read(p []byte) (n int, err error) {
	defer func() {
		if state := recover(); state != nil {
			err = log.Wrap(state.(error))
		}
	}()

	if sb.position >= len64(sb.data) {
		return 0, io.EOF
	}

	n = copy(p, sb.data[sb.position:])
	sb.position += int64(n)

	return n, nil
}

// Truncate either chops or extends the internal buffer.
func (sb *SeekableBuffer) Truncate(size int64) (err error) {
	defer func() {
		if state := recover(); state != nil {
			err = log.Wrap(state.(error))
		}
	}()

	sizeInt := int(size)
	if sizeInt < len(sb.data)-1 {
		sb.data = sb.data[:sizeInt]
	} else {
		new := make([]byte, sizeInt-len(sb.data))
		sb.data = append(sb.data, new...)
	}

	return nil
}

// Seek does a standard seek on the internal slice.
func (sb *SeekableBuffer) Seek(offset int64, whence int) (n int64, err error) {
	defer func() {
		if state := recover(); state != nil {
			err = log.Wrap(state.(error))
		}
	}()

	if whence == os.SEEK_SET {
		sb.position = offset
	} else if whence == os.SEEK_END {
		sb.position = len64(sb.data) + offset
	} else if whence == os.SEEK_CUR {
		sb.position += offset
	} else {
		log.Panicf("seek whence is not valid: (%d)", whence)
	}

	if sb.position < 0 {
		sb.position = 0
	}

	return sb.position, nil
}
