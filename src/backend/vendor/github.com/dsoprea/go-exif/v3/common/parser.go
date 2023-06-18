package exifcommon

import (
	"bytes"
	"errors"
	"math"

	"encoding/binary"

	"github.com/dsoprea/go-logging"
)

var (
	parserLogger = log.NewLogger("exifcommon.parser")
)

var (
	ErrParseFail = errors.New("parse failure")
)

// Parser knows how to parse all well-defined, encoded EXIF types.
type Parser struct {
}

// ParseBytesknows how to parse a byte-type value.
func (p *Parser) ParseBytes(data []byte, unitCount uint32) (value []uint8, err error) {
	defer func() {
		if state := recover(); state != nil {
			err = log.Wrap(state.(error))
		}
	}()

	// TODO(dustin): Add test

	count := int(unitCount)

	if len(data) < (TypeByte.Size() * count) {
		log.Panic(ErrNotEnoughData)
	}

	value = []uint8(data[:count])

	return value, nil
}

// ParseAscii returns a string and auto-strips the trailing NUL character that
// should be at the end of the encoding.
func (p *Parser) ParseAscii(data []byte, unitCount uint32) (value string, err error) {
	defer func() {
		if state := recover(); state != nil {
			err = log.Wrap(state.(error))
		}
	}()

	// TODO(dustin): Add test

	count := int(unitCount)

	if len(data) < (TypeAscii.Size() * count) {
		log.Panic(ErrNotEnoughData)
	}

	if len(data) == 0 || data[count-1] != 0 {
		s := string(data[:count])
		parserLogger.Warningf(nil, "ASCII not terminated with NUL as expected: [%v]", s)

		for i, c := range s {
			if c > 127 {
				// Binary

				t := s[:i]
				parserLogger.Warningf(nil, "ASCII also had binary characters. Truncating: [%v]->[%s]", s, t)

				return t, nil
			}
		}

		return s, nil
	}

	// Auto-strip the NUL from the end. It serves no purpose outside of
	// encoding semantics.

	return string(data[:count-1]), nil
}

// ParseAsciiNoNul returns a string without any consideration for a trailing NUL
// character.
func (p *Parser) ParseAsciiNoNul(data []byte, unitCount uint32) (value string, err error) {
	defer func() {
		if state := recover(); state != nil {
			err = log.Wrap(state.(error))
		}
	}()

	// TODO(dustin): Add test

	count := int(unitCount)

	if len(data) < (TypeAscii.Size() * count) {
		log.Panic(ErrNotEnoughData)
	}

	return string(data[:count]), nil
}

// ParseShorts knows how to parse an encoded list of shorts.
func (p *Parser) ParseShorts(data []byte, unitCount uint32, byteOrder binary.ByteOrder) (value []uint16, err error) {
	defer func() {
		if state := recover(); state != nil {
			err = log.Wrap(state.(error))
		}
	}()

	// TODO(dustin): Add test

	count := int(unitCount)

	if len(data) < (TypeShort.Size() * count) {
		log.Panic(ErrNotEnoughData)
	}

	value = make([]uint16, count)
	for i := 0; i < count; i++ {
		value[i] = byteOrder.Uint16(data[i*2:])
	}

	return value, nil
}

// ParseLongs knows how to encode an encoded list of unsigned longs.
func (p *Parser) ParseLongs(data []byte, unitCount uint32, byteOrder binary.ByteOrder) (value []uint32, err error) {
	defer func() {
		if state := recover(); state != nil {
			err = log.Wrap(state.(error))
		}
	}()

	// TODO(dustin): Add test

	count := int(unitCount)

	if len(data) < (TypeLong.Size() * count) {
		log.Panic(ErrNotEnoughData)
	}

	value = make([]uint32, count)
	for i := 0; i < count; i++ {
		value[i] = byteOrder.Uint32(data[i*4:])
	}

	return value, nil
}

// ParseFloats knows how to encode an encoded list of floats.
func (p *Parser) ParseFloats(data []byte, unitCount uint32, byteOrder binary.ByteOrder) (value []float32, err error) {
	defer func() {
		if state := recover(); state != nil {
			err = log.Wrap(state.(error))
		}
	}()

	count := int(unitCount)

	if len(data) != (TypeFloat.Size() * count) {
		log.Panic(ErrNotEnoughData)
	}

	value = make([]float32, count)
	for i := 0; i < count; i++ {
		value[i] = math.Float32frombits(byteOrder.Uint32(data[i*4 : (i+1)*4]))
	}

	return value, nil
}

// ParseDoubles knows how to encode an encoded list of doubles.
func (p *Parser) ParseDoubles(data []byte, unitCount uint32, byteOrder binary.ByteOrder) (value []float64, err error) {
	defer func() {
		if state := recover(); state != nil {
			err = log.Wrap(state.(error))
		}
	}()

	count := int(unitCount)

	if len(data) != (TypeDouble.Size() * count) {
		log.Panic(ErrNotEnoughData)
	}

	value = make([]float64, count)
	for i := 0; i < count; i++ {
		value[i] = math.Float64frombits(byteOrder.Uint64(data[i*8 : (i+1)*8]))
	}

	return value, nil
}

// ParseRationals knows how to parse an encoded list of unsigned rationals.
func (p *Parser) ParseRationals(data []byte, unitCount uint32, byteOrder binary.ByteOrder) (value []Rational, err error) {
	defer func() {
		if state := recover(); state != nil {
			err = log.Wrap(state.(error))
		}
	}()

	// TODO(dustin): Add test

	count := int(unitCount)

	if len(data) < (TypeRational.Size() * count) {
		log.Panic(ErrNotEnoughData)
	}

	value = make([]Rational, count)
	for i := 0; i < count; i++ {
		value[i].Numerator = byteOrder.Uint32(data[i*8:])
		value[i].Denominator = byteOrder.Uint32(data[i*8+4:])
	}

	return value, nil
}

// ParseSignedLongs knows how to parse an encoded list of signed longs.
func (p *Parser) ParseSignedLongs(data []byte, unitCount uint32, byteOrder binary.ByteOrder) (value []int32, err error) {
	defer func() {
		if state := recover(); state != nil {
			err = log.Wrap(state.(error))
		}
	}()

	// TODO(dustin): Add test

	count := int(unitCount)

	if len(data) < (TypeSignedLong.Size() * count) {
		log.Panic(ErrNotEnoughData)
	}

	b := bytes.NewBuffer(data)

	value = make([]int32, count)
	for i := 0; i < count; i++ {
		err := binary.Read(b, byteOrder, &value[i])
		log.PanicIf(err)
	}

	return value, nil
}

// ParseSignedRationals knows how to parse an encoded list of signed
// rationals.
func (p *Parser) ParseSignedRationals(data []byte, unitCount uint32, byteOrder binary.ByteOrder) (value []SignedRational, err error) {
	defer func() {
		if state := recover(); state != nil {
			err = log.Wrap(state.(error))
		}
	}()

	// TODO(dustin): Add test

	count := int(unitCount)

	if len(data) < (TypeSignedRational.Size() * count) {
		log.Panic(ErrNotEnoughData)
	}

	b := bytes.NewBuffer(data)

	value = make([]SignedRational, count)
	for i := 0; i < count; i++ {
		err = binary.Read(b, byteOrder, &value[i].Numerator)
		log.PanicIf(err)

		err = binary.Read(b, byteOrder, &value[i].Denominator)
		log.PanicIf(err)
	}

	return value, nil
}
