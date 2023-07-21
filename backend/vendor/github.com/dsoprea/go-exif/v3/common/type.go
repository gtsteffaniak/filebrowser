package exifcommon

import (
	"errors"
	"fmt"
	"reflect"
	"strconv"
	"strings"
	"unicode"

	"encoding/binary"

	"github.com/dsoprea/go-logging"
)

var (
	typeLogger = log.NewLogger("exif.type")
)

var (
	// ErrNotEnoughData is used when there isn't enough data to accommodate what
	// we're trying to parse (sizeof(type) * unit_count).
	ErrNotEnoughData = errors.New("not enough data for type")

	// ErrWrongType is used when we try to parse anything other than the
	// current type.
	ErrWrongType = errors.New("wrong type, can not parse")

	// ErrUnhandledUndefinedTypedTag is used when we try to parse a tag that's
	// recorded as an "unknown" type but not a documented tag (therefore
	// leaving us not knowning how to read it).
	ErrUnhandledUndefinedTypedTag = errors.New("not a standard unknown-typed tag")
)

// TagTypePrimitive is a type-alias that let's us easily lookup type properties.
type TagTypePrimitive uint16

const (
	// TypeByte describes an encoded list of bytes.
	TypeByte TagTypePrimitive = 1

	// TypeAscii describes an encoded list of characters that is terminated
	// with a NUL in its encoded form.
	TypeAscii TagTypePrimitive = 2

	// TypeShort describes an encoded list of shorts.
	TypeShort TagTypePrimitive = 3

	// TypeLong describes an encoded list of longs.
	TypeLong TagTypePrimitive = 4

	// TypeRational describes an encoded list of rationals.
	TypeRational TagTypePrimitive = 5

	// TypeUndefined describes an encoded value that has a complex/non-clearcut
	// interpretation.
	TypeUndefined TagTypePrimitive = 7

	// We've seen type-8, but have no documentation on it.

	// TypeSignedLong describes an encoded list of signed longs.
	TypeSignedLong TagTypePrimitive = 9

	// TypeSignedRational describes an encoded list of signed rationals.
	TypeSignedRational TagTypePrimitive = 10

	// TypeFloat describes an encoded list of floats
	TypeFloat TagTypePrimitive = 11

	// TypeDouble describes an encoded list of doubles.
	TypeDouble TagTypePrimitive = 12

	// TypeAsciiNoNul is just a pseudo-type, for our own purposes.
	TypeAsciiNoNul TagTypePrimitive = 0xf0
)

// String returns the name of the type
func (typeType TagTypePrimitive) String() string {
	return TypeNames[typeType]
}

// Size returns the size of one atomic unit of the type.
func (tagType TagTypePrimitive) Size() int {
	switch tagType {
	case TypeByte, TypeAscii, TypeAsciiNoNul:
		return 1
	case TypeShort:
		return 2
	case TypeLong, TypeSignedLong, TypeFloat:
		return 4
	case TypeRational, TypeSignedRational, TypeDouble:
		return 8
	default:
		log.Panicf("can not determine tag-value size for type (%d): [%s]",
			tagType,
			TypeNames[tagType])
		// Never called.
		return 0
	}
}

// IsValid returns true if tagType is a valid type.
func (tagType TagTypePrimitive) IsValid() bool {

	// TODO(dustin): Add test

	return tagType == TypeByte ||
		tagType == TypeAscii ||
		tagType == TypeAsciiNoNul ||
		tagType == TypeShort ||
		tagType == TypeLong ||
		tagType == TypeRational ||
		tagType == TypeSignedLong ||
		tagType == TypeSignedRational ||
		tagType == TypeFloat ||
		tagType == TypeDouble ||
		tagType == TypeUndefined
}

var (
	// TODO(dustin): Rename TypeNames() to typeNames() and add getter.
	TypeNames = map[TagTypePrimitive]string{
		TypeByte:           "BYTE",
		TypeAscii:          "ASCII",
		TypeShort:          "SHORT",
		TypeLong:           "LONG",
		TypeRational:       "RATIONAL",
		TypeUndefined:      "UNDEFINED",
		TypeSignedLong:     "SLONG",
		TypeSignedRational: "SRATIONAL",
		TypeFloat:          "FLOAT",
		TypeDouble:         "DOUBLE",

		TypeAsciiNoNul: "_ASCII_NO_NUL",
	}

	typeNamesR = map[string]TagTypePrimitive{}
)

// Rational describes an unsigned rational value.
type Rational struct {
	// Numerator is the numerator of the rational value.
	Numerator uint32

	// Denominator is the numerator of the rational value.
	Denominator uint32
}

// SignedRational describes a signed rational value.
type SignedRational struct {
	// Numerator is the numerator of the rational value.
	Numerator int32

	// Denominator is the numerator of the rational value.
	Denominator int32
}

func isPrintableText(s string) bool {
	for _, c := range s {
		// unicode.IsPrint() returns false for newline characters.
		if c == 0x0d || c == 0x0a {
			continue
		} else if unicode.IsPrint(rune(c)) == false {
			return false
		}
	}

	return true
}

// Format returns a stringified value for the given encoding. Automatically
// parses. Automatically calculates count based on type size. This function
// also supports undefined-type values (the ones that we support, anyway) by
// way of the String() method that they all require. We can't be more specific
// because we're a base package and we can't refer to it.
func FormatFromType(value interface{}, justFirst bool) (phrase string, err error) {
	defer func() {
		if state := recover(); state != nil {
			err = log.Wrap(state.(error))
		}
	}()

	// TODO(dustin): !! Add test

	switch t := value.(type) {
	case []byte:
		return DumpBytesToString(t), nil
	case string:
		for i, c := range t {
			if c == 0 {
				t = t[:i]
				break
			}
		}

		if isPrintableText(t) == false {
			phrase = fmt.Sprintf("string with binary data (%d bytes)", len(t))
			return phrase, nil
		}

		return t, nil
	case []uint16, []uint32, []int32, []float64, []float32:
		val := reflect.ValueOf(t)

		if val.Len() == 0 {
			return "", nil
		}

		if justFirst == true {
			var valueSuffix string
			if val.Len() > 1 {
				valueSuffix = "..."
			}

			return fmt.Sprintf("%v%s", val.Index(0), valueSuffix), nil
		}

		return fmt.Sprintf("%v", val), nil
	case []Rational:
		if len(t) == 0 {
			return "", nil
		}

		parts := make([]string, len(t))
		for i, r := range t {
			parts[i] = fmt.Sprintf("%d/%d", r.Numerator, r.Denominator)

			if justFirst == true {
				break
			}
		}

		if justFirst == true {
			var valueSuffix string
			if len(t) > 1 {
				valueSuffix = "..."
			}

			return fmt.Sprintf("%v%s", parts[0], valueSuffix), nil
		}

		return fmt.Sprintf("%v", parts), nil
	case []SignedRational:
		if len(t) == 0 {
			return "", nil
		}

		parts := make([]string, len(t))
		for i, r := range t {
			parts[i] = fmt.Sprintf("%d/%d", r.Numerator, r.Denominator)

			if justFirst == true {
				break
			}
		}

		if justFirst == true {
			var valueSuffix string
			if len(t) > 1 {
				valueSuffix = "..."
			}

			return fmt.Sprintf("%v%s", parts[0], valueSuffix), nil
		}

		return fmt.Sprintf("%v", parts), nil
	case fmt.Stringer:
		s := t.String()
		if isPrintableText(s) == false {
			phrase = fmt.Sprintf("stringable with binary data (%d bytes)", len(s))
			return phrase, nil
		}

		// An undefined value that is documented (or that we otherwise support).
		return s, nil
	default:
		// Affects only "unknown" values, in general.
		log.Panicf("type can not be formatted into string: %v", reflect.TypeOf(value).Name())

		// Never called.
		return "", nil
	}
}

// Format returns a stringified value for the given encoding. Automatically
// parses. Automatically calculates count based on type size.
func FormatFromBytes(rawBytes []byte, tagType TagTypePrimitive, justFirst bool, byteOrder binary.ByteOrder) (phrase string, err error) {
	defer func() {
		if state := recover(); state != nil {
			err = log.Wrap(state.(error))
		}
	}()

	// TODO(dustin): !! Add test

	typeSize := tagType.Size()

	if len(rawBytes)%typeSize != 0 {
		log.Panicf("byte-count (%d) does not align for [%s] type with a size of (%d) bytes", len(rawBytes), TypeNames[tagType], typeSize)
	}

	// unitCount is the calculated unit-count. This should equal the original
	// value from the tag (pre-resolution).
	unitCount := uint32(len(rawBytes) / typeSize)

	// Truncate the items if it's not bytes or a string and we just want the first.

	var value interface{}

	switch tagType {
	case TypeByte:
		var err error

		value, err = parser.ParseBytes(rawBytes, unitCount)
		log.PanicIf(err)
	case TypeAscii:
		var err error

		value, err = parser.ParseAscii(rawBytes, unitCount)
		log.PanicIf(err)
	case TypeAsciiNoNul:
		var err error

		value, err = parser.ParseAsciiNoNul(rawBytes, unitCount)
		log.PanicIf(err)
	case TypeShort:
		var err error

		value, err = parser.ParseShorts(rawBytes, unitCount, byteOrder)
		log.PanicIf(err)
	case TypeLong:
		var err error

		value, err = parser.ParseLongs(rawBytes, unitCount, byteOrder)
		log.PanicIf(err)
	case TypeFloat:
		var err error

		value, err = parser.ParseFloats(rawBytes, unitCount, byteOrder)
		log.PanicIf(err)
	case TypeDouble:
		var err error

		value, err = parser.ParseDoubles(rawBytes, unitCount, byteOrder)
		log.PanicIf(err)
	case TypeRational:
		var err error

		value, err = parser.ParseRationals(rawBytes, unitCount, byteOrder)
		log.PanicIf(err)
	case TypeSignedLong:
		var err error

		value, err = parser.ParseSignedLongs(rawBytes, unitCount, byteOrder)
		log.PanicIf(err)
	case TypeSignedRational:
		var err error

		value, err = parser.ParseSignedRationals(rawBytes, unitCount, byteOrder)
		log.PanicIf(err)
	default:
		// Affects only "unknown" values, in general.
		log.Panicf("value of type [%s] can not be formatted into string", tagType.String())

		// Never called.
		return "", nil
	}

	phrase, err = FormatFromType(value, justFirst)
	log.PanicIf(err)

	return phrase, nil
}

// TranslateStringToType converts user-provided strings to properly-typed
// values. If a string, returns a string. Else, assumes that it's a single
// number. If a list needs to be processed, it is the caller's responsibility to
// split it (according to whichever convention has been established).
func TranslateStringToType(tagType TagTypePrimitive, valueString string) (value interface{}, err error) {
	defer func() {
		if state := recover(); state != nil {
			err = log.Wrap(state.(error))
		}
	}()

	if tagType == TypeUndefined {
		// The caller should just call String() on the decoded type.
		log.Panicf("undefined-type values are not supported")
	}

	if tagType == TypeByte {
		wide, err := strconv.ParseInt(valueString, 16, 8)
		log.PanicIf(err)

		return byte(wide), nil
	} else if tagType == TypeAscii || tagType == TypeAsciiNoNul {
		// Whether or not we're putting an NUL on the end is only relevant for
		// byte-level encoding. This function really just supports a user
		// interface.

		return valueString, nil
	} else if tagType == TypeShort {
		n, err := strconv.ParseUint(valueString, 10, 16)
		log.PanicIf(err)

		return uint16(n), nil
	} else if tagType == TypeLong {
		n, err := strconv.ParseUint(valueString, 10, 32)
		log.PanicIf(err)

		return uint32(n), nil
	} else if tagType == TypeRational {
		parts := strings.SplitN(valueString, "/", 2)

		numerator, err := strconv.ParseUint(parts[0], 10, 32)
		log.PanicIf(err)

		denominator, err := strconv.ParseUint(parts[1], 10, 32)
		log.PanicIf(err)

		return Rational{
			Numerator:   uint32(numerator),
			Denominator: uint32(denominator),
		}, nil
	} else if tagType == TypeSignedLong {
		n, err := strconv.ParseInt(valueString, 10, 32)
		log.PanicIf(err)

		return int32(n), nil
	} else if tagType == TypeFloat {
		n, err := strconv.ParseFloat(valueString, 32)
		log.PanicIf(err)

		return float32(n), nil
	} else if tagType == TypeDouble {
		n, err := strconv.ParseFloat(valueString, 64)
		log.PanicIf(err)

		return float64(n), nil
	} else if tagType == TypeSignedRational {
		parts := strings.SplitN(valueString, "/", 2)

		numerator, err := strconv.ParseInt(parts[0], 10, 32)
		log.PanicIf(err)

		denominator, err := strconv.ParseInt(parts[1], 10, 32)
		log.PanicIf(err)

		return SignedRational{
			Numerator:   int32(numerator),
			Denominator: int32(denominator),
		}, nil
	}

	log.Panicf("from-string encoding for type not supported; this shouldn't happen: [%s]", tagType.String())
	return nil, nil
}

// GetTypeByName returns the `TagTypePrimitive` for the given type name.
// Returns (0) if not valid.
func GetTypeByName(typeName string) (tagType TagTypePrimitive, found bool) {
	tagType, found = typeNamesR[typeName]
	return tagType, found
}

// BasicTag describes a single tag for any purpose.
type BasicTag struct {
	// FqIfdPath is the fully-qualified IFD-path.
	FqIfdPath string

	// IfdPath is the unindexed IFD-path.
	IfdPath string

	// TagId is the tag-ID.
	TagId uint16
}

func init() {
	for typeId, typeName := range TypeNames {
		typeNamesR[typeName] = typeId
	}
}
