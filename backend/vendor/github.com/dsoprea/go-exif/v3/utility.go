package exif

import (
	"fmt"
	"io"
	"math"

	"github.com/dsoprea/go-logging"
	"github.com/dsoprea/go-utility/v2/filesystem"

	"github.com/dsoprea/go-exif/v3/common"
	"github.com/dsoprea/go-exif/v3/undefined"
)

var (
	utilityLogger = log.NewLogger("exif.utility")
)

// ExifTag is one simple representation of a tag in a flat list of all of them.
type ExifTag struct {
	// IfdPath is the fully-qualified IFD path (even though it is not named as
	// such).
	IfdPath string `json:"ifd_path"`

	// TagId is the tag-ID.
	TagId uint16 `json:"id"`

	// TagName is the tag-name. This is never empty.
	TagName string `json:"name"`

	// UnitCount is the recorded number of units constution of the value.
	UnitCount uint32 `json:"unit_count"`

	// TagTypeId is the type-ID.
	TagTypeId exifcommon.TagTypePrimitive `json:"type_id"`

	// TagTypeName is the type name.
	TagTypeName string `json:"type_name"`

	// Value is the decoded value.
	Value interface{} `json:"value"`

	// ValueBytes is the raw, encoded value.
	ValueBytes []byte `json:"value_bytes"`

	// Formatted is the human representation of the first value (tag values are
	// always an array).
	FormattedFirst string `json:"formatted_first"`

	// Formatted is the human representation of the complete value.
	Formatted string `json:"formatted"`

	// ChildIfdPath is the name of the child IFD this tag represents (if it
	// represents any). Otherwise, this is empty.
	ChildIfdPath string `json:"child_ifd_path"`
}

// String returns a string representation.
func (et ExifTag) String() string {
	return fmt.Sprintf(
		"ExifTag<"+
			"IFD-PATH=[%s] "+
			"TAG-ID=(0x%02x) "+
			"TAG-NAME=[%s] "+
			"TAG-TYPE=[%s] "+
			"VALUE=[%v] "+
			"VALUE-BYTES=(%d) "+
			"CHILD-IFD-PATH=[%s]",
		et.IfdPath, et.TagId, et.TagName, et.TagTypeName, et.FormattedFirst,
		len(et.ValueBytes), et.ChildIfdPath)
}

// GetFlatExifData returns a simple, flat representation of all tags.
func GetFlatExifData(exifData []byte, so *ScanOptions) (exifTags []ExifTag, med *MiscellaneousExifData, err error) {
	defer func() {
		if state := recover(); state != nil {
			err = log.Wrap(state.(error))
		}
	}()

	sb := rifs.NewSeekableBufferWithBytes(exifData)

	exifTags, med, err = getFlatExifDataUniversalSearchWithReadSeeker(sb, so, false)
	log.PanicIf(err)

	return exifTags, med, nil
}

// RELEASE(dustin): GetFlatExifDataUniversalSearch is a kludge to allow univeral tag searching in a backwards-compatible manner. For the next release, undo this and simply add the flag to GetFlatExifData.

// GetFlatExifDataUniversalSearch returns a simple, flat representation of all
// tags.
func GetFlatExifDataUniversalSearch(exifData []byte, so *ScanOptions, doUniversalSearch bool) (exifTags []ExifTag, med *MiscellaneousExifData, err error) {
	defer func() {
		if state := recover(); state != nil {
			err = log.Wrap(state.(error))
		}
	}()

	sb := rifs.NewSeekableBufferWithBytes(exifData)

	exifTags, med, err = getFlatExifDataUniversalSearchWithReadSeeker(sb, so, doUniversalSearch)
	log.PanicIf(err)

	return exifTags, med, nil
}

// RELEASE(dustin): GetFlatExifDataUniversalSearchWithReadSeeker is a kludge to allow using a ReadSeeker in a backwards-compatible manner. For the next release, drop this and refactor GetFlatExifDataUniversalSearch to take a ReadSeeker.

// GetFlatExifDataUniversalSearchWithReadSeeker returns a simple, flat
// representation of all tags given a ReadSeeker.
func GetFlatExifDataUniversalSearchWithReadSeeker(rs io.ReadSeeker, so *ScanOptions, doUniversalSearch bool) (exifTags []ExifTag, med *MiscellaneousExifData, err error) {
	defer func() {
		if state := recover(); state != nil {
			err = log.Wrap(state.(error))
		}
	}()

	exifTags, med, err = getFlatExifDataUniversalSearchWithReadSeeker(rs, so, doUniversalSearch)
	log.PanicIf(err)

	return exifTags, med, nil
}

// getFlatExifDataUniversalSearchWithReadSeeker returns a simple, flat
// representation of all tags given a ReadSeeker.
func getFlatExifDataUniversalSearchWithReadSeeker(rs io.ReadSeeker, so *ScanOptions, doUniversalSearch bool) (exifTags []ExifTag, med *MiscellaneousExifData, err error) {
	defer func() {
		if state := recover(); state != nil {
			err = log.Wrap(state.(error))
		}
	}()

	headerData := make([]byte, ExifSignatureLength)
	if _, err = io.ReadFull(rs, headerData); err != nil {
		if err == io.EOF {
			return nil, nil, err
		}

		log.Panic(err)
	}

	eh, err := ParseExifHeader(headerData)
	log.PanicIf(err)

	im, err := exifcommon.NewIfdMappingWithStandard()
	log.PanicIf(err)

	ti := NewTagIndex()

	if doUniversalSearch == true {
		ti.SetUniversalSearch(true)
	}

	ebs := NewExifReadSeeker(rs)
	ie := NewIfdEnumerate(im, ti, ebs, eh.ByteOrder)

	exifTags = make([]ExifTag, 0)

	visitor := func(ite *IfdTagEntry) (err error) {
		// This encodes down to base64. Since this an example tool and we do not
		// expect to ever decode the output, we are not worried about
		// specifically base64-encoding it in order to have a measure of
		// control.
		valueBytes, err := ite.GetRawBytes()
		if err != nil {
			if err == exifundefined.ErrUnparseableValue {
				return nil
			}

			log.Panic(err)
		}

		value, err := ite.Value()
		if err != nil {
			if err == exifcommon.ErrUnhandledUndefinedTypedTag {
				value = exifundefined.UnparseableUnknownTagValuePlaceholder
			} else if log.Is(err, exifcommon.ErrParseFail) == true {
				utilityLogger.Warningf(nil,
					"Could not parse value for tag [%s] (%04x) [%s].",
					ite.IfdPath(), ite.TagId(), ite.TagName())

				return nil
			} else {
				log.Panic(err)
			}
		}

		et := ExifTag{
			IfdPath:      ite.IfdPath(),
			TagId:        ite.TagId(),
			TagName:      ite.TagName(),
			UnitCount:    ite.UnitCount(),
			TagTypeId:    ite.TagType(),
			TagTypeName:  ite.TagType().String(),
			Value:        value,
			ValueBytes:   valueBytes,
			ChildIfdPath: ite.ChildIfdPath(),
		}

		et.Formatted, err = ite.Format()
		log.PanicIf(err)

		et.FormattedFirst, err = ite.FormatFirst()
		log.PanicIf(err)

		exifTags = append(exifTags, et)

		return nil
	}

	med, err = ie.Scan(exifcommon.IfdStandardIfdIdentity, eh.FirstIfdOffset, visitor, nil)
	log.PanicIf(err)

	return exifTags, med, nil
}

// GpsDegreesEquals returns true if the two `GpsDegrees` are identical.
func GpsDegreesEquals(gi1, gi2 GpsDegrees) bool {
	if gi2.Orientation != gi1.Orientation {
		return false
	}

	degreesRightBound := math.Nextafter(gi1.Degrees, gi1.Degrees+1)
	minutesRightBound := math.Nextafter(gi1.Minutes, gi1.Minutes+1)
	secondsRightBound := math.Nextafter(gi1.Seconds, gi1.Seconds+1)

	if gi2.Degrees < gi1.Degrees || gi2.Degrees >= degreesRightBound {
		return false
	} else if gi2.Minutes < gi1.Minutes || gi2.Minutes >= minutesRightBound {
		return false
	} else if gi2.Seconds < gi1.Seconds || gi2.Seconds >= secondsRightBound {
		return false
	}

	return true
}
