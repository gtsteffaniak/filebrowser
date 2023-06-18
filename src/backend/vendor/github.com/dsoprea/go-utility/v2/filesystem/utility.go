package rifs

import (
	"io"
	"os"

	"github.com/dsoprea/go-logging"
)

// GetOffset returns the current offset of the Seeker and just panics if unable
// to find it.
func GetOffset(s io.Seeker) int64 {
	offsetRaw, err := s.Seek(0, os.SEEK_CUR)
	log.PanicIf(err)

	return offsetRaw
}
