package exifundefined

import (
	"encoding/binary"

	"github.com/dsoprea/go-logging"

	"github.com/dsoprea/go-exif/v3/common"
)

type Tag9000ExifVersion struct {
	ExifVersion string
}

func (Tag9000ExifVersion) EncoderName() string {
	return "Codec9000ExifVersion"
}

func (ev Tag9000ExifVersion) String() string {
	return ev.ExifVersion
}

type Codec9000ExifVersion struct {
}

func (Codec9000ExifVersion) Encode(value interface{}, byteOrder binary.ByteOrder) (encoded []byte, unitCount uint32, err error) {
	defer func() {
		if state := recover(); state != nil {
			err = log.Wrap(state.(error))
		}
	}()

	s, ok := value.(Tag9000ExifVersion)
	if ok == false {
		log.Panicf("can only encode a Tag9000ExifVersion")
	}

	return []byte(s.ExifVersion), uint32(len(s.ExifVersion)), nil
}

func (Codec9000ExifVersion) Decode(valueContext *exifcommon.ValueContext) (value EncodeableValue, err error) {
	defer func() {
		if state := recover(); state != nil {
			err = log.Wrap(state.(error))
		}
	}()

	valueContext.SetUndefinedValueType(exifcommon.TypeAsciiNoNul)

	valueString, err := valueContext.ReadAsciiNoNul()
	log.PanicIf(err)

	ev := Tag9000ExifVersion{
		ExifVersion: valueString,
	}

	return ev, nil
}

func init() {
	registerEncoder(
		Tag9000ExifVersion{},
		Codec9000ExifVersion{})

	registerDecoder(
		exifcommon.IfdExifStandardIfdIdentity.UnindexedString(),
		0x9000,
		Codec9000ExifVersion{})
}
