package exifundefined

import (
	"encoding/binary"

	"github.com/dsoprea/go-logging"

	"github.com/dsoprea/go-exif/v3/common"
)

// Encode encodes the given encodeable undefined value to bytes.
func Encode(value EncodeableValue, byteOrder binary.ByteOrder) (encoded []byte, unitCount uint32, err error) {
	defer func() {
		if state := recover(); state != nil {
			err = log.Wrap(state.(error))
		}
	}()

	encoderName := value.EncoderName()

	encoder, found := encoders[encoderName]
	if found == false {
		log.Panicf("no encoder registered for type [%s]", encoderName)
	}

	encoded, unitCount, err = encoder.Encode(value, byteOrder)
	log.PanicIf(err)

	return encoded, unitCount, nil
}

// Decode constructs a value from raw encoded bytes
func Decode(valueContext *exifcommon.ValueContext) (value EncodeableValue, err error) {
	defer func() {
		if state := recover(); state != nil {
			err = log.Wrap(state.(error))
		}
	}()

	uth := UndefinedTagHandle{
		IfdPath: valueContext.IfdPath(),
		TagId:   valueContext.TagId(),
	}

	decoder, found := decoders[uth]
	if found == false {
		// We have no choice but to return the error. We have no way of knowing how
		// much data there is without already knowing what data-type this tag is.
		return nil, exifcommon.ErrUnhandledUndefinedTypedTag
	}

	value, err = decoder.Decode(valueContext)
	if err != nil {
		if err == ErrUnparseableValue {
			return nil, err
		}

		log.Panic(err)
	}

	return value, nil
}
