package exifundefined

import (
	"bytes"
	"fmt"

	"encoding/binary"

	"github.com/dsoprea/go-logging"

	"github.com/dsoprea/go-exif/v3/common"
)

type TagA20CSpatialFrequencyResponse struct {
	Columns     uint16
	Rows        uint16
	ColumnNames []string
	Values      []exifcommon.Rational
}

func (TagA20CSpatialFrequencyResponse) EncoderName() string {
	return "CodecA20CSpatialFrequencyResponse"
}

func (sfr TagA20CSpatialFrequencyResponse) String() string {
	return fmt.Sprintf("CodecA20CSpatialFrequencyResponse<COLUMNS=(%d) ROWS=(%d)>", sfr.Columns, sfr.Rows)
}

type CodecA20CSpatialFrequencyResponse struct {
}

func (CodecA20CSpatialFrequencyResponse) Encode(value interface{}, byteOrder binary.ByteOrder) (encoded []byte, unitCount uint32, err error) {
	defer func() {
		if state := recover(); state != nil {
			err = log.Wrap(state.(error))
		}
	}()

	// TODO(dustin): Add test.

	sfr, ok := value.(TagA20CSpatialFrequencyResponse)
	if ok == false {
		log.Panicf("can only encode a TagA20CSpatialFrequencyResponse")
	}

	b := new(bytes.Buffer)

	err = binary.Write(b, byteOrder, sfr.Columns)
	log.PanicIf(err)

	err = binary.Write(b, byteOrder, sfr.Rows)
	log.PanicIf(err)

	// Write columns.

	for _, name := range sfr.ColumnNames {
		_, err := b.WriteString(name)
		log.PanicIf(err)

		err = b.WriteByte(0)
		log.PanicIf(err)
	}

	// Write values.

	ve := exifcommon.NewValueEncoder(byteOrder)

	ed, err := ve.Encode(sfr.Values)
	log.PanicIf(err)

	_, err = b.Write(ed.Encoded)
	log.PanicIf(err)

	encoded = b.Bytes()

	// TODO(dustin): Confirm this size against the specification.

	return encoded, uint32(len(encoded)), nil
}

func (CodecA20CSpatialFrequencyResponse) Decode(valueContext *exifcommon.ValueContext) (value EncodeableValue, err error) {
	defer func() {
		if state := recover(); state != nil {
			err = log.Wrap(state.(error))
		}
	}()

	// TODO(dustin): Add test using known good data.

	byteOrder := valueContext.ByteOrder()

	valueContext.SetUndefinedValueType(exifcommon.TypeByte)

	valueBytes, err := valueContext.ReadBytes()
	log.PanicIf(err)

	sfr := TagA20CSpatialFrequencyResponse{}

	sfr.Columns = byteOrder.Uint16(valueBytes[0:2])
	sfr.Rows = byteOrder.Uint16(valueBytes[2:4])

	columnNames := make([]string, sfr.Columns)

	// startAt is where the current column name starts.
	startAt := 4

	// offset is our current position.
	offset := 4

	currentColumnNumber := uint16(0)

	for currentColumnNumber < sfr.Columns {
		if valueBytes[offset] == 0 {
			columnName := string(valueBytes[startAt:offset])
			if len(columnName) == 0 {
				log.Panicf("SFR column (%d) has zero length", currentColumnNumber)
			}

			columnNames[currentColumnNumber] = columnName
			currentColumnNumber++

			offset++
			startAt = offset
			continue
		}

		offset++
	}

	sfr.ColumnNames = columnNames

	rawRationalBytes := valueBytes[offset:]

	rationalSize := exifcommon.TypeRational.Size()
	if len(rawRationalBytes)%rationalSize > 0 {
		log.Panicf("SFR rationals not aligned: (%d) %% (%d) > 0", len(rawRationalBytes), rationalSize)
	}

	rationalCount := len(rawRationalBytes) / rationalSize

	parser := new(exifcommon.Parser)

	items, err := parser.ParseRationals(rawRationalBytes, uint32(rationalCount), byteOrder)
	log.PanicIf(err)

	sfr.Values = items

	return sfr, nil
}

func init() {
	registerEncoder(
		TagA20CSpatialFrequencyResponse{},
		CodecA20CSpatialFrequencyResponse{})

	registerDecoder(
		exifcommon.IfdExifStandardIfdIdentity.UnindexedString(),
		0xa20c,
		CodecA20CSpatialFrequencyResponse{})
}
