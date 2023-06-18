package exifundefined

import (
	"github.com/dsoprea/go-logging"
)

// UndefinedTagHandle defines one undefined-type tag with a corresponding
// decoder.
type UndefinedTagHandle struct {
	IfdPath string
	TagId   uint16
}

func registerEncoder(entity EncodeableValue, encoder UndefinedValueEncoder) {
	typeName := entity.EncoderName()

	_, found := encoders[typeName]
	if found == true {
		log.Panicf("encoder already registered: %v", typeName)
	}

	encoders[typeName] = encoder
}

func registerDecoder(ifdPath string, tagId uint16, decoder UndefinedValueDecoder) {
	uth := UndefinedTagHandle{
		IfdPath: ifdPath,
		TagId:   tagId,
	}

	_, found := decoders[uth]
	if found == true {
		log.Panicf("decoder already registered: %v", uth)
	}

	decoders[uth] = decoder
}

var (
	encoders = make(map[string]UndefinedValueEncoder)
	decoders = make(map[UndefinedTagHandle]UndefinedValueDecoder)
)
