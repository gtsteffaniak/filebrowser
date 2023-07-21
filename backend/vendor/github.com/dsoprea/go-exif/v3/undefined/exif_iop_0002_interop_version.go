package exifundefined

import (
	"encoding/binary"

	"github.com/dsoprea/go-logging"

	"github.com/dsoprea/go-exif/v3/common"
)

type Tag0002InteropVersion struct {
	InteropVersion string
}

func (Tag0002InteropVersion) EncoderName() string {
	return "Codec0002InteropVersion"
}

func (iv Tag0002InteropVersion) String() string {
	return iv.InteropVersion
}

type Codec0002InteropVersion struct {
}

func (Codec0002InteropVersion) Encode(value interface{}, byteOrder binary.ByteOrder) (encoded []byte, unitCount uint32, err error) {
	defer func() {
		if state := recover(); state != nil {
			err = log.Wrap(state.(error))
		}
	}()

	s, ok := value.(Tag0002InteropVersion)
	if ok == false {
		log.Panicf("can only encode a Tag0002InteropVersion")
	}

	return []byte(s.InteropVersion), uint32(len(s.InteropVersion)), nil
}

func (Codec0002InteropVersion) Decode(valueContext *exifcommon.ValueContext) (value EncodeableValue, err error) {
	defer func() {
		if state := recover(); state != nil {
			err = log.Wrap(state.(error))
		}
	}()

	valueContext.SetUndefinedValueType(exifcommon.TypeAsciiNoNul)

	valueString, err := valueContext.ReadAsciiNoNul()
	log.PanicIf(err)

	iv := Tag0002InteropVersion{
		InteropVersion: valueString,
	}

	return iv, nil
}

func init() {
	registerEncoder(
		Tag0002InteropVersion{},
		Codec0002InteropVersion{})

	registerDecoder(
		exifcommon.IfdExifIopStandardIfdIdentity.UnindexedString(),
		0x0002,
		Codec0002InteropVersion{})
}
