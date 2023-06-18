package exifundefined

import (
	"bytes"
	"fmt"

	"encoding/binary"

	"github.com/dsoprea/go-logging"

	"github.com/dsoprea/go-exif/v3/common"
)

var (
	exif9286Logger = log.NewLogger("exifundefined.exif_9286_user_comment")
)

const (
	TagUndefinedType_9286_UserComment_Encoding_ASCII     = iota
	TagUndefinedType_9286_UserComment_Encoding_JIS       = iota
	TagUndefinedType_9286_UserComment_Encoding_UNICODE   = iota
	TagUndefinedType_9286_UserComment_Encoding_UNDEFINED = iota
)

var (
	TagUndefinedType_9286_UserComment_Encoding_Names = map[int]string{
		TagUndefinedType_9286_UserComment_Encoding_ASCII:     "ASCII",
		TagUndefinedType_9286_UserComment_Encoding_JIS:       "JIS",
		TagUndefinedType_9286_UserComment_Encoding_UNICODE:   "UNICODE",
		TagUndefinedType_9286_UserComment_Encoding_UNDEFINED: "UNDEFINED",
	}

	TagUndefinedType_9286_UserComment_Encodings = map[int][]byte{
		TagUndefinedType_9286_UserComment_Encoding_ASCII:     {'A', 'S', 'C', 'I', 'I', 0, 0, 0},
		TagUndefinedType_9286_UserComment_Encoding_JIS:       {'J', 'I', 'S', 0, 0, 0, 0, 0},
		TagUndefinedType_9286_UserComment_Encoding_UNICODE:   {'U', 'n', 'i', 'c', 'o', 'd', 'e', 0},
		TagUndefinedType_9286_UserComment_Encoding_UNDEFINED: {0, 0, 0, 0, 0, 0, 0, 0},
	}
)

type Tag9286UserComment struct {
	EncodingType  int
	EncodingBytes []byte
}

func (Tag9286UserComment) EncoderName() string {
	return "Codec9286UserComment"
}

func (uc Tag9286UserComment) String() string {
	var valuePhrase string

	if uc.EncodingType == TagUndefinedType_9286_UserComment_Encoding_ASCII {
		return fmt.Sprintf("[ASCII] %s", string(uc.EncodingBytes))
	} else {
		if len(uc.EncodingBytes) <= 8 {
			valuePhrase = fmt.Sprintf("%v", uc.EncodingBytes)
		} else {
			valuePhrase = fmt.Sprintf("%v...", uc.EncodingBytes[:8])
		}
	}

	return fmt.Sprintf("UserComment<SIZE=(%d) ENCODING=[%s] V=%v LEN=(%d)>", len(uc.EncodingBytes), TagUndefinedType_9286_UserComment_Encoding_Names[uc.EncodingType], valuePhrase, len(uc.EncodingBytes))
}

type Codec9286UserComment struct {
}

func (Codec9286UserComment) Encode(value interface{}, byteOrder binary.ByteOrder) (encoded []byte, unitCount uint32, err error) {
	defer func() {
		if state := recover(); state != nil {
			err = log.Wrap(state.(error))
		}
	}()

	uc, ok := value.(Tag9286UserComment)
	if ok == false {
		log.Panicf("can only encode a Tag9286UserComment")
	}

	encodingTypeBytes, found := TagUndefinedType_9286_UserComment_Encodings[uc.EncodingType]
	if found == false {
		log.Panicf("encoding-type not valid for unknown-type tag 9286 (UserComment): (%d)", uc.EncodingType)
	}

	encoded = make([]byte, len(uc.EncodingBytes)+8)

	copy(encoded[:8], encodingTypeBytes)
	copy(encoded[8:], uc.EncodingBytes)

	// TODO(dustin): Confirm this size against the specification.

	return encoded, uint32(len(encoded)), nil
}

func (Codec9286UserComment) Decode(valueContext *exifcommon.ValueContext) (value EncodeableValue, err error) {
	defer func() {
		if state := recover(); state != nil {
			err = log.Wrap(state.(error))
		}
	}()

	valueContext.SetUndefinedValueType(exifcommon.TypeByte)

	valueBytes, err := valueContext.ReadBytes()
	log.PanicIf(err)

	if len(valueBytes) < 8 {
		return nil, ErrUnparseableValue
	}

	unknownUc := Tag9286UserComment{
		EncodingType:  TagUndefinedType_9286_UserComment_Encoding_UNDEFINED,
		EncodingBytes: []byte{},
	}

	encoding := valueBytes[:8]
	for encodingIndex, encodingBytes := range TagUndefinedType_9286_UserComment_Encodings {
		if bytes.Compare(encoding, encodingBytes) == 0 {
			uc := Tag9286UserComment{
				EncodingType:  encodingIndex,
				EncodingBytes: valueBytes[8:],
			}

			return uc, nil
		}
	}

	exif9286Logger.Warningf(nil, "User-comment encoding not valid. Returning 'unknown' type (the default).")
	return unknownUc, nil
}

func init() {
	registerEncoder(
		Tag9286UserComment{},
		Codec9286UserComment{})

	registerDecoder(
		exifcommon.IfdExifStandardIfdIdentity.UnindexedString(),
		0x9286,
		Codec9286UserComment{})
}
