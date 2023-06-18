package exifundefined

import (
	"fmt"
	"strings"

	"crypto/sha1"
	"encoding/binary"

	"github.com/dsoprea/go-logging"

	"github.com/dsoprea/go-exif/v3/common"
)

type Tag927CMakerNote struct {
	MakerNoteType  []byte
	MakerNoteBytes []byte
}

func (Tag927CMakerNote) EncoderName() string {
	return "Codec927CMakerNote"
}

func (mn Tag927CMakerNote) String() string {
	parts := make([]string, len(mn.MakerNoteType))

	for i, c := range mn.MakerNoteType {
		parts[i] = fmt.Sprintf("%02x", c)
	}

	h := sha1.New()

	_, err := h.Write(mn.MakerNoteBytes)
	log.PanicIf(err)

	digest := h.Sum(nil)

	return fmt.Sprintf("MakerNote<TYPE-ID=[%s] LEN=(%d) SHA1=[%020x]>", strings.Join(parts, " "), len(mn.MakerNoteBytes), digest)
}

type Codec927CMakerNote struct {
}

func (Codec927CMakerNote) Encode(value interface{}, byteOrder binary.ByteOrder) (encoded []byte, unitCount uint32, err error) {
	defer func() {
		if state := recover(); state != nil {
			err = log.Wrap(state.(error))
		}
	}()

	mn, ok := value.(Tag927CMakerNote)
	if ok == false {
		log.Panicf("can only encode a Tag927CMakerNote")
	}

	// TODO(dustin): Confirm this size against the specification.

	return mn.MakerNoteBytes, uint32(len(mn.MakerNoteBytes)), nil
}

func (Codec927CMakerNote) Decode(valueContext *exifcommon.ValueContext) (value EncodeableValue, err error) {
	defer func() {
		if state := recover(); state != nil {
			err = log.Wrap(state.(error))
		}
	}()

	// MakerNote
	// TODO(dustin): !! This is the Wild Wild West. This very well might be a child IFD, but any and all OEM's define their own formats. If we're going to be writing changes and this is complete EXIF (which may not have the first eight bytes), it might be fine. However, if these are just IFDs they'll be relative to the main EXIF, this will invalidate the MakerNote data for IFDs and any other implementations that use offsets unless we can interpret them all. It be best to return to this later and just exclude this from being written for now, though means a loss of a wealth of image metadata.
	//                  -> We can also just blindly try to interpret as an IFD and just validate that it's looks good (maybe it will even have a 'next ifd' pointer that we can validate is 0x0).

	valueContext.SetUndefinedValueType(exifcommon.TypeByte)

	valueBytes, err := valueContext.ReadBytes()
	log.PanicIf(err)

	// TODO(dustin): Doesn't work, but here as an example.
	//             ie := NewIfdEnumerate(valueBytes, byteOrder)

	// // TODO(dustin): !! Validate types (might have proprietary types, but it might be worth splitting the list between valid and not valid; maybe fail if a certain proportion are invalid, or maybe aren't less then a certain small integer)?
	//             ii, err := ie.Collect(0x0)

	//             for _, entry := range ii.RootIfd.Entries {
	//                 fmt.Printf("ENTRY: 0x%02x %d\n", entry.TagId, entry.TagType)
	//             }

	var makerNoteType []byte
	if len(valueBytes) >= 20 {
		makerNoteType = valueBytes[:20]
	} else {
		makerNoteType = valueBytes
	}

	mn := Tag927CMakerNote{
		MakerNoteType: makerNoteType,

		// MakerNoteBytes has the whole length of bytes. There's always
		// the chance that the first 20 bytes includes actual data.
		MakerNoteBytes: valueBytes,
	}

	return mn, nil
}

func init() {
	registerEncoder(
		Tag927CMakerNote{},
		Codec927CMakerNote{})

	registerDecoder(
		exifcommon.IfdExifStandardIfdIdentity.UnindexedString(),
		0x927c,
		Codec927CMakerNote{})
}
