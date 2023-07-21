package exifcommon

import (
	"errors"
	"io"

	"encoding/binary"

	"github.com/dsoprea/go-logging"
)

var (
	parser *Parser
)

var (
	// ErrNotFarValue indicates that an offset-based lookup was attempted for a
	// non-offset-based (embedded) value.
	ErrNotFarValue = errors.New("not a far value")
)

// ValueContext embeds all of the parameters required to find and extract the
// actual tag value.
type ValueContext struct {
	unitCount      uint32
	valueOffset    uint32
	rawValueOffset []byte
	rs             io.ReadSeeker

	tagType   TagTypePrimitive
	byteOrder binary.ByteOrder

	// undefinedValueTagType is the effective type to use if this is an
	// "undefined" value.
	undefinedValueTagType TagTypePrimitive

	ifdPath string
	tagId   uint16
}

// TODO(dustin): We can update newValueContext() to derive `valueOffset` itself (from `rawValueOffset`).

// NewValueContext returns a new ValueContext struct.
func NewValueContext(ifdPath string, tagId uint16, unitCount, valueOffset uint32, rawValueOffset []byte, rs io.ReadSeeker, tagType TagTypePrimitive, byteOrder binary.ByteOrder) *ValueContext {
	return &ValueContext{
		unitCount:      unitCount,
		valueOffset:    valueOffset,
		rawValueOffset: rawValueOffset,
		rs:             rs,

		tagType:   tagType,
		byteOrder: byteOrder,

		ifdPath: ifdPath,
		tagId:   tagId,
	}
}

// SetUndefinedValueType sets the effective type if this is an unknown-type tag.
func (vc *ValueContext) SetUndefinedValueType(tagType TagTypePrimitive) {
	if vc.tagType != TypeUndefined {
		log.Panicf("can not set effective type for unknown-type tag because this is *not* an unknown-type tag")
	}

	vc.undefinedValueTagType = tagType
}

// UnitCount returns the embedded unit-count.
func (vc *ValueContext) UnitCount() uint32 {
	return vc.unitCount
}

// ValueOffset returns the value-offset decoded as a `uint32`.
func (vc *ValueContext) ValueOffset() uint32 {
	return vc.valueOffset
}

// RawValueOffset returns the uninterpreted value-offset. This is used for
// embedded values (values small enough to fit within the offset bytes rather
// than needing to be stored elsewhere and referred to by an actual offset).
func (vc *ValueContext) RawValueOffset() []byte {
	return vc.rawValueOffset
}

// AddressableData returns the block of data that we can dereference into.
func (vc *ValueContext) AddressableData() io.ReadSeeker {

	// RELEASE)dustin): Rename from AddressableData() to ReadSeeker()

	return vc.rs
}

// ByteOrder returns the byte-order of numbers.
func (vc *ValueContext) ByteOrder() binary.ByteOrder {
	return vc.byteOrder
}

// IfdPath returns the path of the IFD containing this tag.
func (vc *ValueContext) IfdPath() string {
	return vc.ifdPath
}

// TagId returns the ID of the tag that we represent.
func (vc *ValueContext) TagId() uint16 {
	return vc.tagId
}

// isEmbedded returns whether the value is embedded or a reference. This can't
// be precalculated since the size is not defined for all types (namely the
// "undefined" types).
func (vc *ValueContext) isEmbedded() bool {
	tagType := vc.effectiveValueType()

	return (tagType.Size() * int(vc.unitCount)) <= 4
}

// SizeInBytes returns the number of bytes that this value requires. The
// underlying call will panic if the type is UNDEFINED. It is the
// responsibility of the caller to preemptively check that.
func (vc *ValueContext) SizeInBytes() int {
	tagType := vc.effectiveValueType()

	return tagType.Size() * int(vc.unitCount)
}

// effectiveValueType returns the effective type of the unknown-type tag or, if
// not unknown, the actual type.
func (vc *ValueContext) effectiveValueType() (tagType TagTypePrimitive) {
	if vc.tagType == TypeUndefined {
		tagType = vc.undefinedValueTagType

		if tagType == 0 {
			log.Panicf("undefined-value type not set")
		}
	} else {
		tagType = vc.tagType
	}

	return tagType
}

// readRawEncoded returns the encoded bytes for the value that we represent.
func (vc *ValueContext) readRawEncoded() (rawBytes []byte, err error) {
	defer func() {
		if state := recover(); state != nil {
			err = log.Wrap(state.(error))
		}
	}()

	tagType := vc.effectiveValueType()

	unitSizeRaw := uint32(tagType.Size())

	if vc.isEmbedded() == true {
		byteLength := unitSizeRaw * vc.unitCount
		return vc.rawValueOffset[:byteLength], nil
	}

	_, err = vc.rs.Seek(int64(vc.valueOffset), io.SeekStart)
	log.PanicIf(err)

	rawBytes = make([]byte, vc.unitCount*unitSizeRaw)

	_, err = io.ReadFull(vc.rs, rawBytes)
	log.PanicIf(err)

	return rawBytes, nil
}

// GetFarOffset returns the offset if the value is not embedded [within the
// pointer itself] or an error if an embedded value.
func (vc *ValueContext) GetFarOffset() (offset uint32, err error) {
	if vc.isEmbedded() == true {
		return 0, ErrNotFarValue
	}

	return vc.valueOffset, nil
}

// ReadRawEncoded returns the encoded bytes for the value that we represent.
func (vc *ValueContext) ReadRawEncoded() (rawBytes []byte, err error) {

	// TODO(dustin): Remove this method and rename readRawEncoded in its place.

	return vc.readRawEncoded()
}

// Format returns a string representation for the value.
//
// Where the type is not ASCII, `justFirst` indicates whether to just stringify
// the first item in the slice (or return an empty string if the slice is
// empty).
//
// Since this method lacks the information to process undefined-type tags (e.g.
// byte-order, tag-ID, IFD type), it will return an error if attempted. See
// `Undefined()`.
func (vc *ValueContext) Format() (value string, err error) {
	defer func() {
		if state := recover(); state != nil {
			err = log.Wrap(state.(error))
		}
	}()

	rawBytes, err := vc.readRawEncoded()
	log.PanicIf(err)

	phrase, err := FormatFromBytes(rawBytes, vc.effectiveValueType(), false, vc.byteOrder)
	log.PanicIf(err)

	return phrase, nil
}

// FormatFirst is similar to `Format` but only gets and stringifies the first
// item.
func (vc *ValueContext) FormatFirst() (value string, err error) {
	defer func() {
		if state := recover(); state != nil {
			err = log.Wrap(state.(error))
		}
	}()

	rawBytes, err := vc.readRawEncoded()
	log.PanicIf(err)

	phrase, err := FormatFromBytes(rawBytes, vc.tagType, true, vc.byteOrder)
	log.PanicIf(err)

	return phrase, nil
}

// ReadBytes parses the encoded byte-array from the value-context.
func (vc *ValueContext) ReadBytes() (value []byte, err error) {
	defer func() {
		if state := recover(); state != nil {
			err = log.Wrap(state.(error))
		}
	}()

	rawValue, err := vc.readRawEncoded()
	log.PanicIf(err)

	value, err = parser.ParseBytes(rawValue, vc.unitCount)
	log.PanicIf(err)

	return value, nil
}

// ReadAscii parses the encoded NUL-terminated ASCII string from the value-
// context.
func (vc *ValueContext) ReadAscii() (value string, err error) {
	defer func() {
		if state := recover(); state != nil {
			err = log.Wrap(state.(error))
		}
	}()

	rawValue, err := vc.readRawEncoded()
	log.PanicIf(err)

	value, err = parser.ParseAscii(rawValue, vc.unitCount)
	log.PanicIf(err)

	return value, nil
}

// ReadAsciiNoNul parses the non-NUL-terminated encoded ASCII string from the
// value-context.
func (vc *ValueContext) ReadAsciiNoNul() (value string, err error) {
	defer func() {
		if state := recover(); state != nil {
			err = log.Wrap(state.(error))
		}
	}()

	rawValue, err := vc.readRawEncoded()
	log.PanicIf(err)

	value, err = parser.ParseAsciiNoNul(rawValue, vc.unitCount)
	log.PanicIf(err)

	return value, nil
}

// ReadShorts parses the list of encoded shorts from the value-context.
func (vc *ValueContext) ReadShorts() (value []uint16, err error) {
	defer func() {
		if state := recover(); state != nil {
			err = log.Wrap(state.(error))
		}
	}()

	rawValue, err := vc.readRawEncoded()
	log.PanicIf(err)

	value, err = parser.ParseShorts(rawValue, vc.unitCount, vc.byteOrder)
	log.PanicIf(err)

	return value, nil
}

// ReadLongs parses the list of encoded, unsigned longs from the value-context.
func (vc *ValueContext) ReadLongs() (value []uint32, err error) {
	defer func() {
		if state := recover(); state != nil {
			err = log.Wrap(state.(error))
		}
	}()

	rawValue, err := vc.readRawEncoded()
	log.PanicIf(err)

	value, err = parser.ParseLongs(rawValue, vc.unitCount, vc.byteOrder)
	log.PanicIf(err)

	return value, nil
}

// ReadFloats parses the list of encoded, floats from the value-context.
func (vc *ValueContext) ReadFloats() (value []float32, err error) {
	defer func() {
		if state := recover(); state != nil {
			err = log.Wrap(state.(error))
		}
	}()

	rawValue, err := vc.readRawEncoded()
	log.PanicIf(err)

	value, err = parser.ParseFloats(rawValue, vc.unitCount, vc.byteOrder)
	log.PanicIf(err)

	return value, nil
}

// ReadDoubles parses the list of encoded, doubles from the value-context.
func (vc *ValueContext) ReadDoubles() (value []float64, err error) {
	defer func() {
		if state := recover(); state != nil {
			err = log.Wrap(state.(error))
		}
	}()

	rawValue, err := vc.readRawEncoded()
	log.PanicIf(err)

	value, err = parser.ParseDoubles(rawValue, vc.unitCount, vc.byteOrder)
	log.PanicIf(err)

	return value, nil
}

// ReadRationals parses the list of encoded, unsigned rationals from the value-
// context.
func (vc *ValueContext) ReadRationals() (value []Rational, err error) {
	defer func() {
		if state := recover(); state != nil {
			err = log.Wrap(state.(error))
		}
	}()

	rawValue, err := vc.readRawEncoded()
	log.PanicIf(err)

	value, err = parser.ParseRationals(rawValue, vc.unitCount, vc.byteOrder)
	log.PanicIf(err)

	return value, nil
}

// ReadSignedLongs parses the list of encoded, signed longs from the value-context.
func (vc *ValueContext) ReadSignedLongs() (value []int32, err error) {
	defer func() {
		if state := recover(); state != nil {
			err = log.Wrap(state.(error))
		}
	}()

	rawValue, err := vc.readRawEncoded()
	log.PanicIf(err)

	value, err = parser.ParseSignedLongs(rawValue, vc.unitCount, vc.byteOrder)
	log.PanicIf(err)

	return value, nil
}

// ReadSignedRationals parses the list of encoded, signed rationals from the
// value-context.
func (vc *ValueContext) ReadSignedRationals() (value []SignedRational, err error) {
	defer func() {
		if state := recover(); state != nil {
			err = log.Wrap(state.(error))
		}
	}()

	rawValue, err := vc.readRawEncoded()
	log.PanicIf(err)

	value, err = parser.ParseSignedRationals(rawValue, vc.unitCount, vc.byteOrder)
	log.PanicIf(err)

	return value, nil
}

// Values knows how to resolve the given value. This value is always a list
// (undefined-values aside), so we're named accordingly.
//
// Since this method lacks the information to process unknown-type tags (e.g.
// byte-order, tag-ID, IFD type), it will return an error if attempted. See
// `Undefined()`.
func (vc *ValueContext) Values() (values interface{}, err error) {
	defer func() {
		if state := recover(); state != nil {
			err = log.Wrap(state.(error))
		}
	}()

	if vc.tagType == TypeByte {
		values, err = vc.ReadBytes()
		log.PanicIf(err)
	} else if vc.tagType == TypeAscii {
		values, err = vc.ReadAscii()
		log.PanicIf(err)
	} else if vc.tagType == TypeAsciiNoNul {
		values, err = vc.ReadAsciiNoNul()
		log.PanicIf(err)
	} else if vc.tagType == TypeShort {
		values, err = vc.ReadShorts()
		log.PanicIf(err)
	} else if vc.tagType == TypeLong {
		values, err = vc.ReadLongs()
		log.PanicIf(err)
	} else if vc.tagType == TypeRational {
		values, err = vc.ReadRationals()
		log.PanicIf(err)
	} else if vc.tagType == TypeSignedLong {
		values, err = vc.ReadSignedLongs()
		log.PanicIf(err)
	} else if vc.tagType == TypeSignedRational {
		values, err = vc.ReadSignedRationals()
		log.PanicIf(err)
	} else if vc.tagType == TypeFloat {
		values, err = vc.ReadFloats()
		log.PanicIf(err)
	} else if vc.tagType == TypeDouble {
		values, err = vc.ReadDoubles()
		log.PanicIf(err)
	} else if vc.tagType == TypeUndefined {
		log.Panicf("will not parse undefined-type value")

		// Never called.
		return nil, nil
	} else {
		log.Panicf("value of type [%s] is unparseable", vc.tagType)
		// Never called.
		return nil, nil
	}

	return values, nil
}

func init() {
	parser = new(Parser)
}
