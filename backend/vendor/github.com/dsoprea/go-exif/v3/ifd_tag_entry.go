package exif

import (
	"fmt"
	"io"

	"encoding/binary"

	"github.com/dsoprea/go-logging"

	"github.com/dsoprea/go-exif/v3/common"
	"github.com/dsoprea/go-exif/v3/undefined"
)

var (
	iteLogger = log.NewLogger("exif.ifd_tag_entry")
)

// IfdTagEntry refers to a tag in the loaded EXIF block.
type IfdTagEntry struct {
	tagId          uint16
	tagIndex       int
	tagType        exifcommon.TagTypePrimitive
	unitCount      uint32
	valueOffset    uint32
	rawValueOffset []byte

	// childIfdName is the right most atom in the IFD-path. We need this to
	// construct the fully-qualified IFD-path.
	childIfdName string

	// childIfdPath is the IFD-path of the child if this tag represents a child
	// IFD.
	childIfdPath string

	// childFqIfdPath is the IFD-path of the child if this tag represents a
	// child IFD. Includes indices.
	childFqIfdPath string

	// TODO(dustin): !! IB's host the child-IBs directly in the tag, but that's not the case here. Refactor to accommodate it for a consistent experience.

	ifdIdentity *exifcommon.IfdIdentity

	isUnhandledUnknown bool

	rs        io.ReadSeeker
	byteOrder binary.ByteOrder

	tagName string
}

func newIfdTagEntry(ii *exifcommon.IfdIdentity, tagId uint16, tagIndex int, tagType exifcommon.TagTypePrimitive, unitCount uint32, valueOffset uint32, rawValueOffset []byte, rs io.ReadSeeker, byteOrder binary.ByteOrder) *IfdTagEntry {
	return &IfdTagEntry{
		ifdIdentity:    ii,
		tagId:          tagId,
		tagIndex:       tagIndex,
		tagType:        tagType,
		unitCount:      unitCount,
		valueOffset:    valueOffset,
		rawValueOffset: rawValueOffset,
		rs:             rs,
		byteOrder:      byteOrder,
	}
}

// String returns a stringified representation of the struct.
func (ite *IfdTagEntry) String() string {
	return fmt.Sprintf("IfdTagEntry<TAG-IFD-PATH=[%s] TAG-ID=(0x%04x) TAG-TYPE=[%s] UNIT-COUNT=(%d)>", ite.ifdIdentity.String(), ite.tagId, ite.tagType.String(), ite.unitCount)
}

// TagName returns the name of the tag. This is determined else and set after
// the parse (since it's not actually stored in the stream). If it's empty, it
// is because it is an unknown tag (nonstandard or otherwise unavailable in the
// tag-index).
func (ite *IfdTagEntry) TagName() string {
	return ite.tagName
}

// setTagName sets the tag-name. This provides the name for convenience and
// efficiency by determining it when most efficient while we're parsing rather
// than delegating it to the caller (or, worse, the user).
func (ite *IfdTagEntry) setTagName(tagName string) {
	ite.tagName = tagName
}

// IfdPath returns the fully-qualified path of the IFD that owns this tag.
func (ite *IfdTagEntry) IfdPath() string {
	return ite.ifdIdentity.String()
}

// TagId returns the ID of the tag that we represent. The combination of
// (IfdPath(), TagId()) is unique.
func (ite *IfdTagEntry) TagId() uint16 {
	return ite.tagId
}

// IsThumbnailOffset returns true if the tag has the IFD and tag-ID of a
// thumbnail offset.
func (ite *IfdTagEntry) IsThumbnailOffset() bool {
	return ite.tagId == ThumbnailOffsetTagId && ite.ifdIdentity.String() == ThumbnailFqIfdPath
}

// IsThumbnailSize returns true if the tag has the IFD and tag-ID of a thumbnail
// size.
func (ite *IfdTagEntry) IsThumbnailSize() bool {
	return ite.tagId == ThumbnailSizeTagId && ite.ifdIdentity.String() == ThumbnailFqIfdPath
}

// TagType is the type of value for this tag.
func (ite *IfdTagEntry) TagType() exifcommon.TagTypePrimitive {
	return ite.tagType
}

// updateTagType sets an alternatively interpreted tag-type.
func (ite *IfdTagEntry) updateTagType(tagType exifcommon.TagTypePrimitive) {
	ite.tagType = tagType
}

// UnitCount returns the unit-count of the tag's value.
func (ite *IfdTagEntry) UnitCount() uint32 {
	return ite.unitCount
}

// updateUnitCount sets an alternatively interpreted unit-count.
func (ite *IfdTagEntry) updateUnitCount(unitCount uint32) {
	ite.unitCount = unitCount
}

// getValueOffset is the four-byte offset converted to an integer to point to
// the location of its value in the EXIF block. The "get" parameter is obviously
// used in order to differentiate the naming of the method from the field.
func (ite *IfdTagEntry) getValueOffset() uint32 {
	return ite.valueOffset
}

// GetRawBytes renders a specific list of bytes from the value in this tag.
func (ite *IfdTagEntry) GetRawBytes() (rawBytes []byte, err error) {
	defer func() {
		if state := recover(); state != nil {
			err = log.Wrap(state.(error))
		}
	}()

	valueContext := ite.getValueContext()

	if ite.tagType == exifcommon.TypeUndefined {
		value, err := exifundefined.Decode(valueContext)
		if err != nil {
			if err == exifcommon.ErrUnhandledUndefinedTypedTag {
				ite.setIsUnhandledUnknown(true)
				return nil, exifundefined.ErrUnparseableValue
			} else if err == exifundefined.ErrUnparseableValue {
				return nil, err
			} else {
				log.Panic(err)
			}
		}

		// Encode it back, in order to get the raw bytes. This is the best,
		// general way to do it with an undefined tag.

		rawBytes, _, err := exifundefined.Encode(value, ite.byteOrder)
		log.PanicIf(err)

		return rawBytes, nil
	}

	rawBytes, err = valueContext.ReadRawEncoded()
	log.PanicIf(err)

	return rawBytes, nil
}

// Value returns the specific, parsed, typed value from the tag.
func (ite *IfdTagEntry) Value() (value interface{}, err error) {
	defer func() {
		if state := recover(); state != nil {
			err = log.Wrap(state.(error))
		}
	}()

	valueContext := ite.getValueContext()

	if ite.tagType == exifcommon.TypeUndefined {
		var err error

		value, err = exifundefined.Decode(valueContext)
		if err != nil {
			if err == exifcommon.ErrUnhandledUndefinedTypedTag || err == exifundefined.ErrUnparseableValue {
				return nil, err
			}

			log.Panic(err)
		}
	} else {
		var err error

		value, err = valueContext.Values()
		log.PanicIf(err)
	}

	return value, nil
}

// Format returns the tag's value as a string.
func (ite *IfdTagEntry) Format() (phrase string, err error) {
	defer func() {
		if state := recover(); state != nil {
			err = log.Wrap(state.(error))
		}
	}()

	value, err := ite.Value()
	if err != nil {
		if err == exifcommon.ErrUnhandledUndefinedTypedTag {
			return exifundefined.UnparseableUnknownTagValuePlaceholder, nil
		} else if err == exifundefined.ErrUnparseableValue {
			return exifundefined.UnparseableHandledTagValuePlaceholder, nil
		}

		log.Panic(err)
	}

	phrase, err = exifcommon.FormatFromType(value, false)
	log.PanicIf(err)

	return phrase, nil
}

// FormatFirst returns the same as Format() but only the first item.
func (ite *IfdTagEntry) FormatFirst() (phrase string, err error) {
	defer func() {
		if state := recover(); state != nil {
			err = log.Wrap(state.(error))
		}
	}()

	// TODO(dustin): We should add a convenience type "timestamp", to simplify translating to and from the physical ASCII and provide validation.

	value, err := ite.Value()
	if err != nil {
		if err == exifcommon.ErrUnhandledUndefinedTypedTag {
			return exifundefined.UnparseableUnknownTagValuePlaceholder, nil
		}

		log.Panic(err)
	}

	phrase, err = exifcommon.FormatFromType(value, true)
	log.PanicIf(err)

	return phrase, nil
}

func (ite *IfdTagEntry) setIsUnhandledUnknown(isUnhandledUnknown bool) {
	ite.isUnhandledUnknown = isUnhandledUnknown
}

// SetChildIfd sets child-IFD information (if we represent a child IFD).
func (ite *IfdTagEntry) SetChildIfd(ii *exifcommon.IfdIdentity) {
	ite.childFqIfdPath = ii.String()
	ite.childIfdPath = ii.UnindexedString()
	ite.childIfdName = ii.Name()
}

// ChildIfdName returns the name of the child IFD
func (ite *IfdTagEntry) ChildIfdName() string {
	return ite.childIfdName
}

// ChildIfdPath returns the path of the child IFD.
func (ite *IfdTagEntry) ChildIfdPath() string {
	return ite.childIfdPath
}

// ChildFqIfdPath returns the complete path of the child IFD along with the
// numeric suffixes differentiating sibling occurrences of the same type. "0"
// indices are omitted.
func (ite *IfdTagEntry) ChildFqIfdPath() string {
	return ite.childFqIfdPath
}

// IfdIdentity returns the IfdIdentity associated with this tag.
func (ite *IfdTagEntry) IfdIdentity() *exifcommon.IfdIdentity {
	return ite.ifdIdentity
}

func (ite *IfdTagEntry) getValueContext() *exifcommon.ValueContext {
	return exifcommon.NewValueContext(
		ite.ifdIdentity.String(),
		ite.tagId,
		ite.unitCount,
		ite.valueOffset,
		ite.rawValueOffset,
		ite.rs,
		ite.tagType,
		ite.byteOrder)
}
