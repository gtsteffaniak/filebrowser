package exifundefined

import (
	"bytes"
	"fmt"

	"encoding/binary"

	"github.com/dsoprea/go-logging"

	"github.com/dsoprea/go-exif/v3/common"
)

type TagA302CfaPattern struct {
	HorizontalRepeat uint16
	VerticalRepeat   uint16
	CfaValue         []byte
}

func (TagA302CfaPattern) EncoderName() string {
	return "CodecA302CfaPattern"
}

func (cp TagA302CfaPattern) String() string {
	return fmt.Sprintf("TagA302CfaPattern<HORZ-REPEAT=(%d) VERT-REPEAT=(%d) CFA-VALUE=(%d)>", cp.HorizontalRepeat, cp.VerticalRepeat, len(cp.CfaValue))
}

type CodecA302CfaPattern struct {
}

func (CodecA302CfaPattern) Encode(value interface{}, byteOrder binary.ByteOrder) (encoded []byte, unitCount uint32, err error) {
	defer func() {
		if state := recover(); state != nil {
			err = log.Wrap(state.(error))
		}
	}()

	// TODO(dustin): Add test.

	cp, ok := value.(TagA302CfaPattern)
	if ok == false {
		log.Panicf("can only encode a TagA302CfaPattern")
	}

	b := new(bytes.Buffer)

	err = binary.Write(b, byteOrder, cp.HorizontalRepeat)
	log.PanicIf(err)

	err = binary.Write(b, byteOrder, cp.VerticalRepeat)
	log.PanicIf(err)

	_, err = b.Write(cp.CfaValue)
	log.PanicIf(err)

	encoded = b.Bytes()

	// TODO(dustin): Confirm this size against the specification.

	return encoded, uint32(len(encoded)), nil
}

func (CodecA302CfaPattern) Decode(valueContext *exifcommon.ValueContext) (value EncodeableValue, err error) {
	defer func() {
		if state := recover(); state != nil {
			err = log.Wrap(state.(error))
		}
	}()

	// TODO(dustin): Add test using known good data.

	valueContext.SetUndefinedValueType(exifcommon.TypeByte)

	valueBytes, err := valueContext.ReadBytes()
	log.PanicIf(err)

	cp := TagA302CfaPattern{}

	cp.HorizontalRepeat = valueContext.ByteOrder().Uint16(valueBytes[0:2])
	cp.VerticalRepeat = valueContext.ByteOrder().Uint16(valueBytes[2:4])

	expectedLength := int(cp.HorizontalRepeat * cp.VerticalRepeat)
	cp.CfaValue = valueBytes[4 : 4+expectedLength]

	return cp, nil
}

func init() {
	registerEncoder(
		TagA302CfaPattern{},
		CodecA302CfaPattern{})

	registerDecoder(
		exifcommon.IfdExifStandardIfdIdentity.UnindexedString(),
		0xa302,
		CodecA302CfaPattern{})
}
