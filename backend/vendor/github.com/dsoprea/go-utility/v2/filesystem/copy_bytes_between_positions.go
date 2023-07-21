package rifs

import (
	"io"
	"os"

	"github.com/dsoprea/go-logging"
)

// CopyBytesBetweenPositions will copy bytes from one position in the given RWS
// to an earlier position in the same RWS.
func CopyBytesBetweenPositions(rws io.ReadWriteSeeker, fromPosition, toPosition int64, count int) (n int, err error) {
	defer func() {
		if state := recover(); state != nil {
			err = log.Wrap(state.(error))
		}
	}()

	if fromPosition <= toPosition {
		log.Panicf("from position (%d) must be larger than to position (%d)", fromPosition, toPosition)
	}

	br, err := NewBouncebackReader(rws)
	log.PanicIf(err)

	_, err = br.Seek(fromPosition, os.SEEK_SET)
	log.PanicIf(err)

	bw, err := NewBouncebackWriter(rws)
	log.PanicIf(err)

	_, err = bw.Seek(toPosition, os.SEEK_SET)
	log.PanicIf(err)

	written, err := io.CopyN(bw, br, int64(count))
	log.PanicIf(err)

	n = int(written)
	return n, nil
}
