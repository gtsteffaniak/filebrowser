package rifs

import (
	"fmt"
	"io"

	"github.com/dsoprea/go-logging"
)

// BouncebackStats describes operation counts.
type BouncebackStats struct {
	reads  int
	writes int
	seeks  int
	syncs  int
}

func (bbs BouncebackStats) String() string {
	return fmt.Sprintf(
		"BouncebackStats<READS=(%d) WRITES=(%d) SEEKS=(%d) SYNCS=(%d)>",
		bbs.reads, bbs.writes, bbs.seeks, bbs.syncs)
}

type bouncebackBase struct {
	currentPosition int64

	stats BouncebackStats
}

// Position returns the position that we're supposed to be at.
func (bb *bouncebackBase) Position() int64 {

	// TODO(dustin): Add test

	return bb.currentPosition
}

// StatsReads returns the number of reads that have been attempted.
func (bb *bouncebackBase) StatsReads() int {

	// TODO(dustin): Add test

	return bb.stats.reads
}

// StatsWrites returns the number of write operations.
func (bb *bouncebackBase) StatsWrites() int {

	// TODO(dustin): Add test

	return bb.stats.writes
}

// StatsSeeks returns the number of seeks.
func (bb *bouncebackBase) StatsSeeks() int {

	// TODO(dustin): Add test

	return bb.stats.seeks
}

// StatsSyncs returns the number of corrective seeks ("bounce-backs").
func (bb *bouncebackBase) StatsSyncs() int {

	// TODO(dustin): Add test

	return bb.stats.syncs
}

// Seek does a seek to an arbitrary place in the `io.ReadSeeker`.
func (bb *bouncebackBase) seek(s io.Seeker, offset int64, whence int) (newPosition int64, err error) {
	defer func() {
		if state := recover(); state != nil {
			err = log.Wrap(state.(error))
		}
	}()

	// If the seek is relative, make sure we're where we're supposed to be *first*.
	if whence != io.SeekStart {
		err = bb.checkPosition(s)
		log.PanicIf(err)
	}

	bb.stats.seeks++

	newPosition, err = s.Seek(offset, whence)
	log.PanicIf(err)

	// Update our internal tracking.
	bb.currentPosition = newPosition

	return newPosition, nil
}

func (bb *bouncebackBase) checkPosition(s io.Seeker) (err error) {
	defer func() {
		if state := recover(); state != nil {
			err = log.Wrap(state.(error))
		}
	}()

	// Make sure we're where we're supposed to be.

	// This should have no overhead, and enables us to collect stats.
	realCurrentPosition, err := s.Seek(0, io.SeekCurrent)
	log.PanicIf(err)

	if realCurrentPosition != bb.currentPosition {
		bb.stats.syncs++

		_, err = s.Seek(bb.currentPosition, io.SeekStart)
		log.PanicIf(err)
	}

	return nil
}

// BouncebackReader wraps a ReadSeeker, keeps track of our position, and
// seeks back to it before writing. This allows an underlying ReadWriteSeeker
// with an unstable position can still be used for a prolonged series of writes.
type BouncebackReader struct {
	rs io.ReadSeeker

	bouncebackBase
}

// NewBouncebackReader returns a `*BouncebackReader` struct.
func NewBouncebackReader(rs io.ReadSeeker) (br *BouncebackReader, err error) {
	defer func() {
		if state := recover(); state != nil {
			err = log.Wrap(state.(error))
		}
	}()

	initialPosition, err := rs.Seek(0, io.SeekCurrent)
	log.PanicIf(err)

	bb := bouncebackBase{
		currentPosition: initialPosition,
	}

	br = &BouncebackReader{
		rs:             rs,
		bouncebackBase: bb,
	}

	return br, nil
}

// Seek does a seek to an arbitrary place in the `io.ReadSeeker`.
func (br *BouncebackReader) Seek(offset int64, whence int) (newPosition int64, err error) {
	defer func() {
		if state := recover(); state != nil {
			err = log.Wrap(state.(error))
		}
	}()

	newPosition, err = br.bouncebackBase.seek(br.rs, offset, whence)
	log.PanicIf(err)

	return newPosition, nil
}

// Seek does a standard read.
func (br *BouncebackReader) Read(p []byte) (n int, err error) {
	defer func() {
		if state := recover(); state != nil {
			err = log.Wrap(state.(error))
		}
	}()

	br.bouncebackBase.stats.reads++

	err = br.bouncebackBase.checkPosition(br.rs)
	log.PanicIf(err)

	// Do read.

	n, err = br.rs.Read(p)
	if err != nil {
		if err == io.EOF {
			return 0, io.EOF
		}

		log.Panic(err)
	}

	// Update our internal tracking.
	br.bouncebackBase.currentPosition += int64(n)

	return n, nil
}

// BouncebackWriter wraps a WriteSeeker, keeps track of our position, and
// seeks back to it before writing. This allows an underlying ReadWriteSeeker
// with an unstable position can still be used for a prolonged series of writes.
type BouncebackWriter struct {
	ws io.WriteSeeker

	bouncebackBase
}

// NewBouncebackWriter returns a new `BouncebackWriter` struct.
func NewBouncebackWriter(ws io.WriteSeeker) (bw *BouncebackWriter, err error) {
	defer func() {
		if state := recover(); state != nil {
			err = log.Wrap(state.(error))
		}
	}()

	initialPosition, err := ws.Seek(0, io.SeekCurrent)
	log.PanicIf(err)

	bb := bouncebackBase{
		currentPosition: initialPosition,
	}

	bw = &BouncebackWriter{
		ws:             ws,
		bouncebackBase: bb,
	}

	return bw, nil
}

// Seek puts us at a specific position in the internal writer for the next
// write/seek.
func (bw *BouncebackWriter) Seek(offset int64, whence int) (newPosition int64, err error) {
	defer func() {
		if state := recover(); state != nil {
			err = log.Wrap(state.(error))
		}
	}()

	newPosition, err = bw.bouncebackBase.seek(bw.ws, offset, whence)
	log.PanicIf(err)

	return newPosition, nil
}

// Write performs a write against the internal `WriteSeeker` starting at the
// position that we're supposed to be at.
func (bw *BouncebackWriter) Write(p []byte) (n int, err error) {
	defer func() {
		if state := recover(); state != nil {
			err = log.Wrap(state.(error))
		}
	}()

	bw.bouncebackBase.stats.writes++

	// Make sure we're where we're supposed to be.

	realCurrentPosition, err := bw.ws.Seek(0, io.SeekCurrent)
	log.PanicIf(err)

	if realCurrentPosition != bw.bouncebackBase.currentPosition {
		bw.bouncebackBase.stats.seeks++

		_, err = bw.ws.Seek(bw.bouncebackBase.currentPosition, io.SeekStart)
		log.PanicIf(err)
	}

	// Do write.

	n, err = bw.ws.Write(p)
	log.PanicIf(err)

	// Update our internal tracking.
	bw.bouncebackBase.currentPosition += int64(n)

	return n, nil
}
