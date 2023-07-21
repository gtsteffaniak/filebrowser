package rifs

import (
	"io"
	"os"

	"github.com/dsoprea/go-logging"
)

// SeekType is a convenience type to associate the different seek-types with
// printable descriptions.
type SeekType int

// String returns a descriptive string.
func (n SeekType) String() string {
	if n == io.SeekCurrent {
		return "SEEK-CURRENT"
	} else if n == io.SeekEnd {
		return "SEEK-END"
	} else if n == io.SeekStart {
		return "SEEK-START"
	}

	log.Panicf("unknown seek-type: (%d)", n)
	return ""
}

// CalculateSeek calculates an offset in a file-stream given the parameters.
func CalculateSeek(currentOffset int64, delta int64, whence int, fileSize int64) (finalOffset int64, err error) {
	defer func() {
		if state := recover(); state != nil {
			err = log.Wrap(state.(error))
			finalOffset = 0
		}
	}()

	if whence == os.SEEK_SET {
		finalOffset = delta
	} else if whence == os.SEEK_CUR {
		finalOffset = currentOffset + delta
	} else if whence == os.SEEK_END {
		finalOffset = fileSize + delta
	} else {
		log.Panicf("whence not valid: (%d)", whence)
	}

	if finalOffset < 0 {
		finalOffset = 0
	}

	return finalOffset, nil
}
