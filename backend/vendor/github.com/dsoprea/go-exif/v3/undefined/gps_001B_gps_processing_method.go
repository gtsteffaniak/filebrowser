package exifundefined

import (
	"encoding/binary"

	"github.com/dsoprea/go-logging"

	"github.com/dsoprea/go-exif/v3/common"
)

type Tag001BGPSProcessingMethod struct {
	string
}

func (Tag001BGPSProcessingMethod) EncoderName() string {
	return "Codec001BGPSProcessingMethod"
}

func (gpm Tag001BGPSProcessingMethod) String() string {
	return gpm.string
}

type Codec001BGPSProcessingMethod struct {
}

func (Codec001BGPSProcessingMethod) Encode(value interface{}, byteOrder binary.ByteOrder) (encoded []byte, unitCount uint32, err error) {
	defer func() {
		if state := recover(); state != nil {
			err = log.Wrap(state.(error))
		}
	}()

	s, ok := value.(Tag001BGPSProcessingMethod)
	if ok == false {
		log.Panicf("can only encode a Tag001BGPSProcessingMethod")
	}

	return []byte(s.string), uint32(len(s.string)), nil
}

func (Codec001BGPSProcessingMethod) Decode(valueContext *exifcommon.ValueContext) (value EncodeableValue, err error) {
	defer func() {
		if state := recover(); state != nil {
			err = log.Wrap(state.(error))
		}
	}()

	valueContext.SetUndefinedValueType(exifcommon.TypeAsciiNoNul)

	valueString, err := valueContext.ReadAsciiNoNul()
	log.PanicIf(err)

	return Tag001BGPSProcessingMethod{valueString}, nil
}

func init() {
	registerEncoder(
		Tag001BGPSProcessingMethod{},
		Codec001BGPSProcessingMethod{})

	registerDecoder(
		exifcommon.IfdGpsInfoStandardIfdIdentity.UnindexedString(),
		0x001b,
		Codec001BGPSProcessingMethod{})
}
