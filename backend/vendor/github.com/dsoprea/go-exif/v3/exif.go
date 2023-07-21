package exif

import (
	"bufio"
	"bytes"
	"errors"
	"fmt"
	"io"
	"os"

	"encoding/binary"
	"io/ioutil"

	"github.com/dsoprea/go-logging"

	"github.com/dsoprea/go-exif/v3/common"
)

const (
	// ExifAddressableAreaStart is the absolute offset in the file that all
	// offsets are relative to.
	ExifAddressableAreaStart = uint32(0x0)

	// ExifDefaultFirstIfdOffset is essentially the number of bytes in addition
	// to `ExifAddressableAreaStart` that you have to move in order to escape
	// the rest of the header and get to the earliest point where we can put
	// stuff (which has to be the first IFD). This is the size of the header
	// sequence containing the two-character byte-order, two-character fixed-
	// bytes, and the four bytes describing the first-IFD offset.
	ExifDefaultFirstIfdOffset = uint32(2 + 2 + 4)
)

const (
	// ExifSignatureLength is the number of bytes in the EXIF signature (which
	// customarily includes the first IFD offset).
	ExifSignatureLength = 8
)

var (
	exifLogger = log.NewLogger("exif.exif")

	ExifBigEndianSignature    = [4]byte{'M', 'M', 0x00, 0x2a}
	ExifLittleEndianSignature = [4]byte{'I', 'I', 0x2a, 0x00}
)

var (
	ErrNoExif          = errors.New("no exif data")
	ErrExifHeaderError = errors.New("exif header error")
)

// SearchAndExtractExif searches for an EXIF blob in the byte-slice.
func SearchAndExtractExif(data []byte) (rawExif []byte, err error) {
	defer func() {
		if state := recover(); state != nil {
			err = log.Wrap(state.(error))
		}
	}()

	b := bytes.NewBuffer(data)

	rawExif, err = SearchAndExtractExifWithReader(b)
	if err != nil {
		if err == ErrNoExif {
			return nil, err
		}

		log.Panic(err)
	}

	return rawExif, nil
}

// SearchAndExtractExifN searches for an EXIF blob in the byte-slice, but skips
// the given number of EXIF blocks first. This is a forensics tool that helps
// identify multiple EXIF blocks in a file.
func SearchAndExtractExifN(data []byte, n int) (rawExif []byte, err error) {
	defer func() {
		if state := recover(); state != nil {
			err = log.Wrap(state.(error))
		}
	}()

	skips := 0
	totalDiscarded := 0
	for {
		b := bytes.NewBuffer(data)

		var discarded int

		rawExif, discarded, err = searchAndExtractExifWithReaderWithDiscarded(b)
		if err != nil {
			if err == ErrNoExif {
				return nil, err
			}

			log.Panic(err)
		}

		exifLogger.Debugf(nil, "Read EXIF block (%d).", skips)

		totalDiscarded += discarded

		if skips >= n {
			exifLogger.Debugf(nil, "Reached requested EXIF block (%d).", n)
			break
		}

		nextOffset := discarded + 1
		exifLogger.Debugf(nil, "Skipping EXIF block (%d) by seeking to position (%d).", skips, nextOffset)

		data = data[nextOffset:]
		skips++
	}

	exifLogger.Debugf(nil, "Found EXIF blob (%d) bytes from initial position.", totalDiscarded)
	return rawExif, nil
}

// searchAndExtractExifWithReaderWithDiscarded searches for an EXIF blob using
// an `io.Reader`. We can't know how much long the EXIF data is without parsing
// it, so this will likely grab up a lot of the image-data, too.
//
// This function returned the count of preceding bytes.
func searchAndExtractExifWithReaderWithDiscarded(r io.Reader) (rawExif []byte, discarded int, err error) {
	defer func() {
		if state := recover(); state != nil {
			err = log.Wrap(state.(error))
		}
	}()

	// Search for the beginning of the EXIF information. The EXIF is near the
	// beginning of most JPEGs, so this likely doesn't have a high cost (at
	// least, again, with JPEGs).

	br := bufio.NewReader(r)

	for {
		window, err := br.Peek(ExifSignatureLength)
		if err != nil {
			if err == io.EOF {
				return nil, 0, ErrNoExif
			}

			log.Panic(err)
		}

		_, err = ParseExifHeader(window)
		if err != nil {
			if log.Is(err, ErrNoExif) == true {
				// No EXIF. Move forward by one byte.

				_, err := br.Discard(1)
				log.PanicIf(err)

				discarded++

				continue
			}

			// Some other error.
			log.Panic(err)
		}

		break
	}

	exifLogger.Debugf(nil, "Found EXIF blob (%d) bytes from initial position.", discarded)

	rawExif, err = ioutil.ReadAll(br)
	log.PanicIf(err)

	return rawExif, discarded, nil
}

// RELEASE(dustin): We should replace the implementation of SearchAndExtractExifWithReader with searchAndExtractExifWithReaderWithDiscarded and drop the latter.

// SearchAndExtractExifWithReader searches for an EXIF blob using an
// `io.Reader`. We can't know how much long the EXIF data is without parsing it,
// so this will likely grab up a lot of the image-data, too.
func SearchAndExtractExifWithReader(r io.Reader) (rawExif []byte, err error) {
	defer func() {
		if state := recover(); state != nil {
			err = log.Wrap(state.(error))
		}
	}()

	rawExif, _, err = searchAndExtractExifWithReaderWithDiscarded(r)
	if err != nil {
		if err == ErrNoExif {
			return nil, err
		}

		log.Panic(err)
	}

	return rawExif, nil
}

// SearchFileAndExtractExif returns a slice from the beginning of the EXIF data
// to the end of the file (it's not practical to try and calculate where the
// data actually ends).
func SearchFileAndExtractExif(filepath string) (rawExif []byte, err error) {
	defer func() {
		if state := recover(); state != nil {
			err = log.Wrap(state.(error))
		}
	}()

	// Open the file.

	f, err := os.Open(filepath)
	log.PanicIf(err)

	defer f.Close()

	rawExif, err = SearchAndExtractExifWithReader(f)
	log.PanicIf(err)

	return rawExif, nil
}

type ExifHeader struct {
	ByteOrder      binary.ByteOrder
	FirstIfdOffset uint32
}

func (eh ExifHeader) String() string {
	return fmt.Sprintf("ExifHeader<BYTE-ORDER=[%v] FIRST-IFD-OFFSET=(0x%02x)>", eh.ByteOrder, eh.FirstIfdOffset)
}

// ParseExifHeader parses the bytes at the very top of the header.
//
// This will panic with ErrNoExif on any data errors so that we can double as
// an EXIF-detection routine.
func ParseExifHeader(data []byte) (eh ExifHeader, err error) {
	defer func() {
		if state := recover(); state != nil {
			err = log.Wrap(state.(error))
		}
	}()

	// Good reference:
	//
	//      CIPA DC-008-2016; JEITA CP-3451D
	//      -> http://www.cipa.jp/std/documents/e/DC-008-Translation-2016-E.pdf

	if len(data) < ExifSignatureLength {
		exifLogger.Warningf(nil, "Not enough data for EXIF header: (%d)", len(data))
		return eh, ErrNoExif
	}

	if bytes.Equal(data[:4], ExifBigEndianSignature[:]) == true {
		exifLogger.Debugf(nil, "Byte-order is big-endian.")
		eh.ByteOrder = binary.BigEndian
	} else if bytes.Equal(data[:4], ExifLittleEndianSignature[:]) == true {
		eh.ByteOrder = binary.LittleEndian
		exifLogger.Debugf(nil, "Byte-order is little-endian.")
	} else {
		return eh, ErrNoExif
	}

	eh.FirstIfdOffset = eh.ByteOrder.Uint32(data[4:8])

	return eh, nil
}

// Visit recursively invokes a callback for every tag.
func Visit(rootIfdIdentity *exifcommon.IfdIdentity, ifdMapping *exifcommon.IfdMapping, tagIndex *TagIndex, exifData []byte, visitor TagVisitorFn, so *ScanOptions) (eh ExifHeader, furthestOffset uint32, err error) {
	defer func() {
		if state := recover(); state != nil {
			err = log.Wrap(state.(error))
		}
	}()

	eh, err = ParseExifHeader(exifData)
	log.PanicIf(err)

	ebs := NewExifReadSeekerWithBytes(exifData)
	ie := NewIfdEnumerate(ifdMapping, tagIndex, ebs, eh.ByteOrder)

	_, err = ie.Scan(rootIfdIdentity, eh.FirstIfdOffset, visitor, so)
	log.PanicIf(err)

	furthestOffset = ie.FurthestOffset()

	return eh, furthestOffset, nil
}

// Collect recursively builds a static structure of all IFDs and tags.
func Collect(ifdMapping *exifcommon.IfdMapping, tagIndex *TagIndex, exifData []byte) (eh ExifHeader, index IfdIndex, err error) {
	defer func() {
		if state := recover(); state != nil {
			err = log.Wrap(state.(error))
		}
	}()

	eh, err = ParseExifHeader(exifData)
	log.PanicIf(err)

	ebs := NewExifReadSeekerWithBytes(exifData)
	ie := NewIfdEnumerate(ifdMapping, tagIndex, ebs, eh.ByteOrder)

	index, err = ie.Collect(eh.FirstIfdOffset)
	log.PanicIf(err)

	return eh, index, nil
}

// BuildExifHeader constructs the bytes that go at the front of the stream.
func BuildExifHeader(byteOrder binary.ByteOrder, firstIfdOffset uint32) (headerBytes []byte, err error) {
	defer func() {
		if state := recover(); state != nil {
			err = log.Wrap(state.(error))
		}
	}()

	b := new(bytes.Buffer)

	var signatureBytes []byte
	if byteOrder == binary.BigEndian {
		signatureBytes = ExifBigEndianSignature[:]
	} else {
		signatureBytes = ExifLittleEndianSignature[:]
	}

	_, err = b.Write(signatureBytes)
	log.PanicIf(err)

	err = binary.Write(b, byteOrder, firstIfdOffset)
	log.PanicIf(err)

	return b.Bytes(), nil
}
