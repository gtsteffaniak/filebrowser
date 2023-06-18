package exifundefined

import (
	"bytes"
	"fmt"

	"encoding/binary"

	"github.com/dsoprea/go-logging"

	"github.com/dsoprea/go-exif/v3/common"
)

type Tag8828Oecf struct {
	Columns     uint16
	Rows        uint16
	ColumnNames []string
	Values      []exifcommon.SignedRational
}

func (oecf Tag8828Oecf) String() string {
	return fmt.Sprintf("Tag8828Oecf<COLUMNS=(%d) ROWS=(%d)>", oecf.Columns, oecf.Rows)
}

func (oecf Tag8828Oecf) EncoderName() string {
	return "Codec8828Oecf"
}

type Codec8828Oecf struct {
}

func (Codec8828Oecf) Encode(value interface{}, byteOrder binary.ByteOrder) (encoded []byte, unitCount uint32, err error) {
	defer func() {
		if state := recover(); state != nil {
			err = log.Wrap(state.(error))
		}
	}()

	// TODO(dustin): Add test

	oecf, ok := value.(Tag8828Oecf)
	if ok == false {
		log.Panicf("can only encode a Tag8828Oecf")
	}

	b := new(bytes.Buffer)

	err = binary.Write(b, byteOrder, oecf.Columns)
	log.PanicIf(err)

	err = binary.Write(b, byteOrder, oecf.Rows)
	log.PanicIf(err)

	for _, name := range oecf.ColumnNames {
		_, err := b.Write([]byte(name))
		log.PanicIf(err)

		_, err = b.Write([]byte{0})
		log.PanicIf(err)
	}

	ve := exifcommon.NewValueEncoder(byteOrder)

	ed, err := ve.Encode(oecf.Values)
	log.PanicIf(err)

	_, err = b.Write(ed.Encoded)
	log.PanicIf(err)

	return b.Bytes(), uint32(b.Len()), nil
}

func (Codec8828Oecf) Decode(valueContext *exifcommon.ValueContext) (value EncodeableValue, err error) {
	defer func() {
		if state := recover(); state != nil {
			err = log.Wrap(state.(error))
		}
	}()

	// TODO(dustin): Add test using known good data.

	valueContext.SetUndefinedValueType(exifcommon.TypeByte)

	valueBytes, err := valueContext.ReadBytes()
	log.PanicIf(err)

	oecf := Tag8828Oecf{}

	oecf.Columns = valueContext.ByteOrder().Uint16(valueBytes[0:2])
	oecf.Rows = valueContext.ByteOrder().Uint16(valueBytes[2:4])

	columnNames := make([]string, oecf.Columns)

	// startAt is where the current column name starts.
	startAt := 4

	// offset is our current position.
	offset := startAt

	currentColumnNumber := uint16(0)

	for currentColumnNumber < oecf.Columns {
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

	oecf.ColumnNames = columnNames

	rawRationalBytes := valueBytes[offset:]

	rationalSize := exifcommon.TypeSignedRational.Size()
	if len(rawRationalBytes)%rationalSize > 0 {
		log.Panicf("OECF signed-rationals not aligned: (%d) %% (%d) > 0", len(rawRationalBytes), rationalSize)
	}

	rationalCount := len(rawRationalBytes) / rationalSize

	parser := new(exifcommon.Parser)

	byteOrder := valueContext.ByteOrder()

	items, err := parser.ParseSignedRationals(rawRationalBytes, uint32(rationalCount), byteOrder)
	log.PanicIf(err)

	oecf.Values = items

	return oecf, nil
}

func init() {
	registerDecoder(
		exifcommon.IfdExifStandardIfdIdentity.UnindexedString(),
		0x8828,
		Codec8828Oecf{})
}
