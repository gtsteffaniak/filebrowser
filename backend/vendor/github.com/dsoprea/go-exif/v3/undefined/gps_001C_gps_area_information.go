package exifundefined

import (
	"encoding/binary"

	"github.com/dsoprea/go-logging"

	"github.com/dsoprea/go-exif/v3/common"
)

type Tag001CGPSAreaInformation struct {
	string
}

func (Tag001CGPSAreaInformation) EncoderName() string {
	return "Codec001CGPSAreaInformation"
}

func (gai Tag001CGPSAreaInformation) String() string {
	return gai.string
}

type Codec001CGPSAreaInformation struct {
}

func (Codec001CGPSAreaInformation) Encode(value interface{}, byteOrder binary.ByteOrder) (encoded []byte, unitCount uint32, err error) {
	defer func() {
		if state := recover(); state != nil {
			err = log.Wrap(state.(error))
		}
	}()

	s, ok := value.(Tag001CGPSAreaInformation)
	if ok == false {
		log.Panicf("can only encode a Tag001CGPSAreaInformation")
	}

	return []byte(s.string), uint32(len(s.string)), nil
}

func (Codec001CGPSAreaInformation) Decode(valueContext *exifcommon.ValueContext) (value EncodeableValue, err error) {
	defer func() {
		if state := recover(); state != nil {
			err = log.Wrap(state.(error))
		}
	}()

	valueContext.SetUndefinedValueType(exifcommon.TypeAsciiNoNul)

	valueString, err := valueContext.ReadAsciiNoNul()
	log.PanicIf(err)

	return Tag001CGPSAreaInformation{valueString}, nil
}

func init() {
	registerEncoder(
		Tag001CGPSAreaInformation{},
		Codec001CGPSAreaInformation{})

	registerDecoder(
		exifcommon.IfdGpsInfoStandardIfdIdentity.UnindexedString(),
		0x001c,
		Codec001CGPSAreaInformation{})
}
