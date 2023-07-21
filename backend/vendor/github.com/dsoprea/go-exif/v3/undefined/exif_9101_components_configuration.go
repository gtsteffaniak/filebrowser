package exifundefined

import (
	"bytes"
	"fmt"

	"encoding/binary"

	"github.com/dsoprea/go-logging"

	"github.com/dsoprea/go-exif/v3/common"
)

const (
	TagUndefinedType_9101_ComponentsConfiguration_Channel_Y  = 0x1
	TagUndefinedType_9101_ComponentsConfiguration_Channel_Cb = 0x2
	TagUndefinedType_9101_ComponentsConfiguration_Channel_Cr = 0x3
	TagUndefinedType_9101_ComponentsConfiguration_Channel_R  = 0x4
	TagUndefinedType_9101_ComponentsConfiguration_Channel_G  = 0x5
	TagUndefinedType_9101_ComponentsConfiguration_Channel_B  = 0x6
)

const (
	TagUndefinedType_9101_ComponentsConfiguration_OTHER = iota
	TagUndefinedType_9101_ComponentsConfiguration_RGB   = iota
	TagUndefinedType_9101_ComponentsConfiguration_YCBCR = iota
)

var (
	TagUndefinedType_9101_ComponentsConfiguration_Names = map[int]string{
		TagUndefinedType_9101_ComponentsConfiguration_OTHER: "OTHER",
		TagUndefinedType_9101_ComponentsConfiguration_RGB:   "RGB",
		TagUndefinedType_9101_ComponentsConfiguration_YCBCR: "YCBCR",
	}

	TagUndefinedType_9101_ComponentsConfiguration_Configurations = map[int][]byte{
		TagUndefinedType_9101_ComponentsConfiguration_RGB: {
			TagUndefinedType_9101_ComponentsConfiguration_Channel_R,
			TagUndefinedType_9101_ComponentsConfiguration_Channel_G,
			TagUndefinedType_9101_ComponentsConfiguration_Channel_B,
			0,
		},

		TagUndefinedType_9101_ComponentsConfiguration_YCBCR: {
			TagUndefinedType_9101_ComponentsConfiguration_Channel_Y,
			TagUndefinedType_9101_ComponentsConfiguration_Channel_Cb,
			TagUndefinedType_9101_ComponentsConfiguration_Channel_Cr,
			0,
		},
	}
)

type TagExif9101ComponentsConfiguration struct {
	ConfigurationId    int
	ConfigurationBytes []byte
}

func (TagExif9101ComponentsConfiguration) EncoderName() string {
	return "CodecExif9101ComponentsConfiguration"
}

func (cc TagExif9101ComponentsConfiguration) String() string {
	return fmt.Sprintf("Exif9101ComponentsConfiguration<ID=[%s] BYTES=%v>", TagUndefinedType_9101_ComponentsConfiguration_Names[cc.ConfigurationId], cc.ConfigurationBytes)
}

type CodecExif9101ComponentsConfiguration struct {
}

func (CodecExif9101ComponentsConfiguration) Encode(value interface{}, byteOrder binary.ByteOrder) (encoded []byte, unitCount uint32, err error) {
	defer func() {
		if state := recover(); state != nil {
			err = log.Wrap(state.(error))
		}
	}()

	cc, ok := value.(TagExif9101ComponentsConfiguration)
	if ok == false {
		log.Panicf("can only encode a TagExif9101ComponentsConfiguration")
	}

	return cc.ConfigurationBytes, uint32(len(cc.ConfigurationBytes)), nil
}

func (CodecExif9101ComponentsConfiguration) Decode(valueContext *exifcommon.ValueContext) (value EncodeableValue, err error) {
	defer func() {
		if state := recover(); state != nil {
			err = log.Wrap(state.(error))
		}
	}()

	valueContext.SetUndefinedValueType(exifcommon.TypeByte)

	valueBytes, err := valueContext.ReadBytes()
	log.PanicIf(err)

	for configurationId, configurationBytes := range TagUndefinedType_9101_ComponentsConfiguration_Configurations {
		if bytes.Equal(configurationBytes, valueBytes) == true {
			cc := TagExif9101ComponentsConfiguration{
				ConfigurationId:    configurationId,
				ConfigurationBytes: valueBytes,
			}

			return cc, nil
		}
	}

	cc := TagExif9101ComponentsConfiguration{
		ConfigurationId:    TagUndefinedType_9101_ComponentsConfiguration_OTHER,
		ConfigurationBytes: valueBytes,
	}

	return cc, nil
}

func init() {
	registerEncoder(
		TagExif9101ComponentsConfiguration{},
		CodecExif9101ComponentsConfiguration{})

	registerDecoder(
		exifcommon.IfdExifStandardIfdIdentity.UnindexedString(),
		0x9101,
		CodecExif9101ComponentsConfiguration{})
}
