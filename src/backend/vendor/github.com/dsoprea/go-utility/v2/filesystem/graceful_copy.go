package rifs

import (
	"fmt"
	"io"
)

const (
	defaultCopyBufferSize = 1024 * 1024
)

// GracefulCopy willcopy while enduring lesser normal issues.
//
// - We'll ignore EOF if the read byte-count is more than zero. Only an EOF when
//   zero bytes were read will terminate the loop.
//
// - Ignore short-writes. If less bytes were written than the bytes that were
//   given, we'll keep trying until done.
func GracefulCopy(w io.Writer, r io.Reader, buffer []byte) (copyCount int, err error) {
	if buffer == nil {
		buffer = make([]byte, defaultCopyBufferSize)
	}

	for {
		readCount, err := r.Read(buffer)
		if err != nil {
			if err != io.EOF {
				err = fmt.Errorf("read error: %s", err.Error())
				return 0, err
			}

			// Only break on EOF if no bytes were actually read.
			if readCount == 0 {
				break
			}
		}

		writeBuffer := buffer[:readCount]

		for len(writeBuffer) > 0 {
			writtenCount, err := w.Write(writeBuffer)
			if err != nil {
				err = fmt.Errorf("write error: %s", err.Error())
				return 0, err
			}

			writeBuffer = writeBuffer[writtenCount:]
		}

		copyCount += readCount
	}

	return copyCount, nil
}
