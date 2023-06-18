package exifundefined

import (
	"errors"

	"encoding/binary"

	"github.com/dsoprea/go-exif/v3/common"
)

const (
	// UnparseableUnknownTagValuePlaceholder is the string to use for an unknown
	// undefined tag.
	UnparseableUnknownTagValuePlaceholder = "!UNKNOWN"

	// UnparseableHandledTagValuePlaceholder is the string to use for a known
	// value that is not parseable.
	UnparseableHandledTagValuePlaceholder = "!MALFORMED"
)

var (
	// ErrUnparseableValue is the error for a value that we should have been
	// able to parse but were not able to.
	ErrUnparseableValue = errors.New("unparseable undefined tag")
)

// UndefinedValueEncoder knows how to encode an undefined-type tag's value to
// bytes.
type UndefinedValueEncoder interface {
	Encode(value interface{}, byteOrder binary.ByteOrder) (encoded []byte, unitCount uint32, err error)
}

// EncodeableValue wraps a value with the information that will be needed to re-
// encode it later.
type EncodeableValue interface {
	EncoderName() string
	String() string
}

// UndefinedValueDecoder knows how to decode an undefined-type tag's value from
// bytes.
type UndefinedValueDecoder interface {
	Decode(valueContext *exifcommon.ValueContext) (value EncodeableValue, err error)
}
