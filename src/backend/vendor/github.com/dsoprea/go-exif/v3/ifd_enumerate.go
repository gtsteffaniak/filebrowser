package exif

import (
	"bytes"
	"errors"
	"fmt"
	"io"
	"strconv"
	"strings"
	"time"

	"encoding/binary"

	"github.com/dsoprea/go-logging"

	"github.com/dsoprea/go-exif/v3/common"
	"github.com/dsoprea/go-exif/v3/undefined"
)

var (
	ifdEnumerateLogger = log.NewLogger("exif.ifd_enumerate")
)

var (
	// ErrNoThumbnail means that no thumbnail was found.
	ErrNoThumbnail = errors.New("no thumbnail")

	// ErrNoGpsTags means that no GPS info was found.
	ErrNoGpsTags = errors.New("no gps tags")

	// ErrTagTypeNotValid means that the tag-type is not valid.
	ErrTagTypeNotValid = errors.New("tag type invalid")

	// ErrOffsetInvalid means that the file offset is not valid.
	ErrOffsetInvalid = errors.New("file offset invalid")
)

var (
	// ValidGpsVersions is the list of recognized EXIF GPS versions/signatures.
	ValidGpsVersions = [][4]byte{
		// 2.0.0.0 appears to have a very similar format to 2.2.0.0, so enabling
		// it under that assumption.
		//
		// IFD-PATH=[IFD] ID=(0x8825) NAME=[GPSTag] COUNT=(1) TYPE=[LONG] VALUE=[114]
		// IFD-PATH=[IFD/GPSInfo] ID=(0x0000) NAME=[GPSVersionID] COUNT=(4) TYPE=[BYTE] VALUE=[02 00 00 00]
		// IFD-PATH=[IFD/GPSInfo] ID=(0x0001) NAME=[GPSLatitudeRef] COUNT=(2) TYPE=[ASCII] VALUE=[S]
		// IFD-PATH=[IFD/GPSInfo] ID=(0x0002) NAME=[GPSLatitude] COUNT=(3) TYPE=[RATIONAL] VALUE=[38/1...]
		// IFD-PATH=[IFD/GPSInfo] ID=(0x0003) NAME=[GPSLongitudeRef] COUNT=(2) TYPE=[ASCII] VALUE=[E]
		// IFD-PATH=[IFD/GPSInfo] ID=(0x0004) NAME=[GPSLongitude] COUNT=(3) TYPE=[RATIONAL] VALUE=[144/1...]
		// IFD-PATH=[IFD/GPSInfo] ID=(0x0012) NAME=[GPSMapDatum] COUNT=(7) TYPE=[ASCII] VALUE=[WGS-84]
		//
		{2, 0, 0, 0},

		{2, 2, 0, 0},

		// Suddenly appeared at the default in 2.31: https://home.jeita.or.jp/tsc/std-pdf/CP-3451D.pdf
		//
		// Note that the presence of 2.3.0.0 doesn't seem to guarantee
		// coordinates. In some cases, we seen just the following:
		//
		// GPS Tag Version     |2.3.0.0
		// GPS Receiver Status |V
		// Geodetic Survey Data|WGS-84
		// GPS Differential Cor|0
		//
		{2, 3, 0, 0},
	}
)

// byteParser knows how to decode an IFD and all of the tags it
// describes.
//
// The IFDs and the actual values can float throughout the EXIF block, but the
// IFD itself is just a minor header followed by a set of repeating,
// statically-sized records. So, the tags (though notnecessarily their values)
// are fairly simple to enumerate.
type byteParser struct {
	byteOrder     binary.ByteOrder
	rs            io.ReadSeeker
	ifdOffset     uint32
	currentOffset uint32
}

// newByteParser returns a new byteParser struct.
//
// initialOffset is for arithmetic-based tracking of where we should be at in
// the stream.
func newByteParser(rs io.ReadSeeker, byteOrder binary.ByteOrder, initialOffset uint32) (bp *byteParser, err error) {
	// TODO(dustin): Add test

	bp = &byteParser{
		rs:            rs,
		byteOrder:     byteOrder,
		currentOffset: initialOffset,
	}

	return bp, nil
}

// getUint16 reads a uint16 and advances both our current and our current
// accumulator (which allows us to know how far to seek to the beginning of the
// next IFD when it's time to jump).
func (bp *byteParser) getUint16() (value uint16, raw []byte, err error) {
	defer func() {
		if state := recover(); state != nil {
			err = log.Wrap(state.(error))
		}
	}()

	// TODO(dustin): Add test

	needBytes := 2

	raw = make([]byte, needBytes)

	_, err = io.ReadFull(bp.rs, raw)
	log.PanicIf(err)

	value = bp.byteOrder.Uint16(raw)

	bp.currentOffset += uint32(needBytes)

	return value, raw, nil
}

// getUint32 reads a uint32 and advances both our current and our current
// accumulator (which allows us to know how far to seek to the beginning of the
// next IFD when it's time to jump).
func (bp *byteParser) getUint32() (value uint32, raw []byte, err error) {
	defer func() {
		if state := recover(); state != nil {
			err = log.Wrap(state.(error))
		}
	}()

	// TODO(dustin): Add test

	needBytes := 4

	raw = make([]byte, needBytes)

	_, err = io.ReadFull(bp.rs, raw)
	log.PanicIf(err)

	value = bp.byteOrder.Uint32(raw)

	bp.currentOffset += uint32(needBytes)

	return value, raw, nil
}

// CurrentOffset returns the starting offset but the number of bytes that we
// have parsed. This is arithmetic-based tracking, not a seek(0) operation.
func (bp *byteParser) CurrentOffset() uint32 {
	return bp.currentOffset
}

// IfdEnumerate is the main enumeration type. It knows how to parse the IFD
// containers in the EXIF blob.
type IfdEnumerate struct {
	ebs            ExifBlobSeeker
	byteOrder      binary.ByteOrder
	tagIndex       *TagIndex
	ifdMapping     *exifcommon.IfdMapping
	furthestOffset uint32

	visitedIfdOffsets map[uint32]struct{}
}

// NewIfdEnumerate returns a new instance of IfdEnumerate.
func NewIfdEnumerate(ifdMapping *exifcommon.IfdMapping, tagIndex *TagIndex, ebs ExifBlobSeeker, byteOrder binary.ByteOrder) *IfdEnumerate {
	return &IfdEnumerate{
		ebs:        ebs,
		byteOrder:  byteOrder,
		ifdMapping: ifdMapping,
		tagIndex:   tagIndex,

		visitedIfdOffsets: make(map[uint32]struct{}),
	}
}

func (ie *IfdEnumerate) getByteParser(ifdOffset uint32) (bp *byteParser, err error) {
	defer func() {
		if state := recover(); state != nil {
			err = log.Wrap(state.(error))
		}
	}()

	initialOffset := ExifAddressableAreaStart + ifdOffset

	rs, err := ie.ebs.GetReadSeeker(int64(initialOffset))
	log.PanicIf(err)

	bp, err =
		newByteParser(
			rs,
			ie.byteOrder,
			initialOffset)

	if err != nil {
		if err == ErrOffsetInvalid {
			return nil, err
		}

		log.Panic(err)
	}

	return bp, nil
}

func (ie *IfdEnumerate) parseTag(ii *exifcommon.IfdIdentity, tagPosition int, bp *byteParser) (ite *IfdTagEntry, err error) {
	defer func() {
		if state := recover(); state != nil {
			err = log.Wrap(state.(error))
		}
	}()

	tagId, _, err := bp.getUint16()
	log.PanicIf(err)

	tagTypeRaw, _, err := bp.getUint16()
	log.PanicIf(err)

	tagType := exifcommon.TagTypePrimitive(tagTypeRaw)

	unitCount, _, err := bp.getUint32()
	log.PanicIf(err)

	valueOffset, rawValueOffset, err := bp.getUint32()
	log.PanicIf(err)

	// Check whether the embedded type indicator is valid.

	if tagType.IsValid() == false {
		// Technically, we have the type on-file in the tags-index, but
		// if the type stored alongside the data disagrees with it,
		// which it apparently does, all bets are off.
		ifdEnumerateLogger.Warningf(nil,
			"Tag (0x%04x) in IFD [%s] at position (%d) has invalid type (0x%04x) and will be skipped.",
			tagId, ii, tagPosition, int(tagType))

		ite = &IfdTagEntry{
			tagId:   tagId,
			tagType: tagType,
		}

		return ite, ErrTagTypeNotValid
	}

	// Check whether the embedded type is listed among the supported types for
	// the registered tag. If not, skip processing the tag.

	it, err := ie.tagIndex.Get(ii, tagId)
	if err != nil {
		if log.Is(err, ErrTagNotFound) == true {
			ifdEnumerateLogger.Warningf(nil, "Tag (0x%04x) is not known and will be skipped.", tagId)

			ite = &IfdTagEntry{
				tagId: tagId,
			}

			return ite, ErrTagNotFound
		}

		log.Panic(err)
	}

	// If we're trying to be as forgiving as possible then use whatever type was
	// reported in the format. Otherwise, only accept a type that's expected for
	// this tag.
	if ie.tagIndex.UniversalSearch() == false && it.DoesSupportType(tagType) == false {
		// The type in the stream disagrees with the type that this tag is
		// expected to have. This can present issues with how we handle the
		// special-case tags (e.g. thumbnails, GPS, etc..) when those tags
		// suddenly have data that we no longer manipulate correctly/
		// accurately.
		ifdEnumerateLogger.Warningf(nil,
			"Tag (0x%04x) in IFD [%s] at position (%d) has unsupported type (0x%02x) and will be skipped.",
			tagId, ii, tagPosition, int(tagType))

		return nil, ErrTagTypeNotValid
	}

	// Construct tag struct.

	rs, err := ie.ebs.GetReadSeeker(0)
	log.PanicIf(err)

	ite = newIfdTagEntry(
		ii,
		tagId,
		tagPosition,
		tagType,
		unitCount,
		valueOffset,
		rawValueOffset,
		rs,
		ie.byteOrder)

	ifdPath := ii.UnindexedString()

	// If it's an IFD but not a standard one, it'll just be seen as a LONG
	// (the standard IFD tag type), later, unless we skip it because it's
	// [likely] not even in the standard list of known tags.
	mi, err := ie.ifdMapping.GetChild(ifdPath, tagId)
	if err == nil {
		currentIfdTag := ii.IfdTag()

		childIt := exifcommon.NewIfdTag(&currentIfdTag, tagId, mi.Name)
		iiChild := ii.NewChild(childIt, 0)
		ite.SetChildIfd(iiChild)

		// We also need to set `tag.ChildFqIfdPath` but can't do it here
		// because we don't have the IFD index.
	} else if log.Is(err, exifcommon.ErrChildIfdNotMapped) == false {
		log.Panic(err)
	}

	return ite, nil
}

// TagVisitorFn is called for each tag when enumerating through the EXIF.
type TagVisitorFn func(ite *IfdTagEntry) (err error)

// tagPostParse do some tag-level processing here following the parse of each.
func (ie *IfdEnumerate) tagPostParse(ite *IfdTagEntry, med *MiscellaneousExifData) (err error) {
	defer func() {
		if state := recover(); state != nil {
			err = log.Wrap(state.(error))
		}
	}()

	// TODO(dustin): Add test

	ii := ite.IfdIdentity()

	tagId := ite.TagId()
	tagType := ite.TagType()

	it, err := ie.tagIndex.Get(ii, tagId)
	if err == nil {
		ite.setTagName(it.Name)
	} else {
		if err != ErrTagNotFound {
			log.Panic(err)
		}

		// This is an unknown tag.

		originalBt := exifcommon.BasicTag{
			FqIfdPath: ii.String(),
			IfdPath:   ii.UnindexedString(),
			TagId:     tagId,
		}

		if med != nil {
			med.unknownTags[originalBt] = exifcommon.BasicTag{}
		}

		utilityLogger.Debugf(nil,
			"Tag (0x%04x) is not valid for IFD [%s]. Attempting secondary "+
				"lookup.", tagId, ii.String())

		// This will overwrite the existing `it` and `err`. Since `FindFirst()`
		// might generate different Errors than `Get()`, the log message above
		// is import to try and mitigate confusion in that case.
		it, err = ie.tagIndex.FindFirst(tagId, tagType, nil)
		if err != nil {
			if err != ErrTagNotFound {
				log.Panic(err)
			}

			// This is supposed to be a convenience function and if we were
			// to keep the name empty or set it to some placeholder, it
			// might be mismanaged by the package that is calling us. If
			// they want to specifically manage these types of tags, they
			// can use more advanced functionality to specifically -handle
			// unknown tags.
			utilityLogger.Warningf(nil,
				"Tag with ID (0x%04x) in IFD [%s] is not recognized and "+
					"will be ignored.", tagId, ii.String())

			return ErrTagNotFound
		}

		ite.setTagName(it.Name)

		utilityLogger.Warningf(nil,
			"Tag with ID (0x%04x) is not valid for IFD [%s], but it *is* "+
				"valid as tag [%s] under IFD [%s] and has the same type "+
				"[%s], so we will use that. This EXIF blob was probably "+
				"written by a buggy implementation.",
			tagId, ii.UnindexedString(), it.Name, it.IfdPath,
			tagType)

		if med != nil {
			med.unknownTags[originalBt] = exifcommon.BasicTag{
				IfdPath: it.IfdPath,
				TagId:   tagId,
			}
		}
	}

	// This is a known tag (from the standard, unless the user did
	// something different).

	// Skip any tags that have a type that doesn't match the type in the
	// index (which is loaded with the standard and accept tag
	// information unless configured otherwise).
	//
	// We've run into multiple instances of the same tag, where a) no
	// tag should ever be repeated, and b) all but one had an incorrect
	// type and caused parsing/conversion woes. So, this is a quick fix
	// for those scenarios.
	if ie.tagIndex.UniversalSearch() == false && it.DoesSupportType(tagType) == false {
		ifdEnumerateLogger.Warningf(nil,
			"Skipping tag [%s] (0x%04x) [%s] with an unexpected type: %v ∉ %v",
			ii.UnindexedString(), tagId, it.Name,
			tagType, it.SupportedTypes)

		return ErrTagNotFound
	}

	return nil
}

// parseIfd decodes the IFD block that we're currently sitting on the first
// byte of.
func (ie *IfdEnumerate) parseIfd(ii *exifcommon.IfdIdentity, bp *byteParser, visitor TagVisitorFn, doDescend bool, med *MiscellaneousExifData) (nextIfdOffset uint32, entries []*IfdTagEntry, thumbnailData []byte, err error) {
	defer func() {
		if state := recover(); state != nil {
			err = log.Wrap(state.(error))
		}
	}()

	tagCount, _, err := bp.getUint16()
	log.PanicIf(err)

	ifdEnumerateLogger.Debugf(nil, "IFD [%s] tag-count: (%d)", ii.String(), tagCount)

	entries = make([]*IfdTagEntry, 0)

	var enumeratorThumbnailOffset *IfdTagEntry
	var enumeratorThumbnailSize *IfdTagEntry

	for i := 0; i < int(tagCount); i++ {
		ite, err := ie.parseTag(ii, i, bp)
		if err != nil {
			if log.Is(err, ErrTagNotFound) == true || log.Is(err, ErrTagTypeNotValid) == true {
				// These tags should've been fully logged in parseTag(). The
				// ITE returned is nil so we can't print anything about them, now.
				continue
			}

			log.Panic(err)
		}

		err = ie.tagPostParse(ite, med)
		if err == nil {
			if err == ErrTagNotFound {
				continue
			}

			log.PanicIf(err)
		}

		tagId := ite.TagId()

		if visitor != nil {
			err := visitor(ite)
			log.PanicIf(err)
		}

		if ite.IsThumbnailOffset() == true {
			ifdEnumerateLogger.Debugf(nil, "Skipping the thumbnail offset tag (0x%04x). Use accessors to get it or set it.", tagId)

			enumeratorThumbnailOffset = ite
			entries = append(entries, ite)

			continue
		} else if ite.IsThumbnailSize() == true {
			ifdEnumerateLogger.Debugf(nil, "Skipping the thumbnail size tag (0x%04x). Use accessors to get it or set it.", tagId)

			enumeratorThumbnailSize = ite
			entries = append(entries, ite)

			continue
		}

		if ite.TagType() != exifcommon.TypeUndefined {
			// If this tag's value is an offset, bump our max-offset value to
			// what that offset is plus however large that value is.

			vc := ite.getValueContext()

			farOffset, err := vc.GetFarOffset()
			if err == nil {
				candidateOffset := farOffset + uint32(vc.SizeInBytes())
				if candidateOffset > ie.furthestOffset {
					ie.furthestOffset = candidateOffset
				}
			} else if err != exifcommon.ErrNotFarValue {
				log.PanicIf(err)
			}
		}

		// If it's an IFD but not a standard one, it'll just be seen as a LONG
		// (the standard IFD tag type), later, unless we skip it because it's
		// [likely] not even in the standard list of known tags.
		if ite.ChildIfdPath() != "" {
			if doDescend == true {
				ifdEnumerateLogger.Debugf(nil, "Descending from IFD [%s] to IFD [%s].", ii, ite.ChildIfdPath())

				currentIfdTag := ii.IfdTag()

				childIfdTag :=
					exifcommon.NewIfdTag(
						&currentIfdTag,
						ite.TagId(),
						ite.ChildIfdName())

				iiChild := ii.NewChild(childIfdTag, 0)

				err := ie.scan(iiChild, ite.getValueOffset(), visitor, med)
				log.PanicIf(err)

				ifdEnumerateLogger.Debugf(nil, "Ascending from IFD [%s] to IFD [%s].", ite.ChildIfdPath(), ii)
			}
		}

		entries = append(entries, ite)
	}

	if enumeratorThumbnailOffset != nil && enumeratorThumbnailSize != nil {
		thumbnailData, err = ie.parseThumbnail(enumeratorThumbnailOffset, enumeratorThumbnailSize)
		if err != nil {
			ifdEnumerateLogger.Errorf(
				nil, err,
				"We tried to bump our furthest-offset counter but there was an issue first seeking past the thumbnail.")
		} else {
			// In this case, the value is always an offset.
			offset := enumeratorThumbnailOffset.getValueOffset()

			// This this case, the value is always a length.
			length := enumeratorThumbnailSize.getValueOffset()

			ifdEnumerateLogger.Debugf(nil, "Found thumbnail in IFD [%s]. Its offset is (%d) and is (%d) bytes.", ii, offset, length)

			furthestOffset := offset + length

			if furthestOffset > ie.furthestOffset {
				ie.furthestOffset = furthestOffset
			}
		}
	}

	nextIfdOffset, _, err = bp.getUint32()
	log.PanicIf(err)

	_, alreadyVisited := ie.visitedIfdOffsets[nextIfdOffset]

	if alreadyVisited == true {
		ifdEnumerateLogger.Warningf(nil, "IFD at offset (0x%08x) has been linked-to more than once. There might be a cycle in the IFD chain. Not reparsing.", nextIfdOffset)
		nextIfdOffset = 0
	}

	if nextIfdOffset != 0 {
		ie.visitedIfdOffsets[nextIfdOffset] = struct{}{}
		ifdEnumerateLogger.Debugf(nil, "[%s] Next IFD at offset: (0x%08x)", ii.String(), nextIfdOffset)
	} else {
		ifdEnumerateLogger.Debugf(nil, "[%s] IFD chain has terminated.", ii.String())
	}

	return nextIfdOffset, entries, thumbnailData, nil
}

func (ie *IfdEnumerate) parseThumbnail(offsetIte, lengthIte *IfdTagEntry) (thumbnailData []byte, err error) {
	defer func() {
		if state := recover(); state != nil {
			err = log.Wrap(state.(error))
		}
	}()

	vRaw, err := lengthIte.Value()
	log.PanicIf(err)

	vList := vRaw.([]uint32)
	if len(vList) != 1 {
		log.Panicf("not exactly one long: (%d)", len(vList))
	}

	length := vList[0]

	// The tag is official a LONG type, but it's actually an offset to a blob of bytes.
	offsetIte.updateTagType(exifcommon.TypeByte)
	offsetIte.updateUnitCount(length)

	thumbnailData, err = offsetIte.GetRawBytes()
	log.PanicIf(err)

	return thumbnailData, nil
}

// scan parses and enumerates the different IFD blocks and invokes a visitor
// callback for each tag. No information is kept or returned.
func (ie *IfdEnumerate) scan(iiGeneral *exifcommon.IfdIdentity, ifdOffset uint32, visitor TagVisitorFn, med *MiscellaneousExifData) (err error) {
	defer func() {
		if state := recover(); state != nil {
			err = log.Wrap(state.(error))
		}
	}()

	// TODO(dustin): Add test

	for ifdIndex := 0; ; ifdIndex++ {
		iiSibling := iiGeneral.NewSibling(ifdIndex)

		ifdEnumerateLogger.Debugf(nil, "Parsing IFD [%s] at offset (0x%04x) (scan).", iiSibling.String(), ifdOffset)

		bp, err := ie.getByteParser(ifdOffset)
		if err != nil {
			if err == ErrOffsetInvalid {
				ifdEnumerateLogger.Errorf(nil, nil, "IFD [%s] at offset (0x%04x) is unreachable. Terminating scan.", iiSibling.String(), ifdOffset)
				break
			}

			log.Panic(err)
		}

		nextIfdOffset, _, _, err := ie.parseIfd(iiSibling, bp, visitor, true, med)
		log.PanicIf(err)

		currentOffset := bp.CurrentOffset()
		if currentOffset > ie.furthestOffset {
			ie.furthestOffset = currentOffset
		}

		if nextIfdOffset == 0 {
			break
		}

		ifdOffset = nextIfdOffset
	}

	return nil
}

// MiscellaneousExifData is reports additional data collected during the parse.
type MiscellaneousExifData struct {
	// UnknownTags contains all tags that were invalid for their containing
	// IFDs. The values represent alternative IFDs that were correctly matched
	// to those tags and used instead.
	unknownTags map[exifcommon.BasicTag]exifcommon.BasicTag
}

// UnknownTags returns the unknown tags encountered during the scan.
func (med *MiscellaneousExifData) UnknownTags() map[exifcommon.BasicTag]exifcommon.BasicTag {
	return med.unknownTags
}

// ScanOptions tweaks parser behavior/choices.
type ScanOptions struct {
	// NOTE(dustin): Reserved for future usage.
}

// Scan enumerates the different EXIF blocks (called IFDs). `rootIfdName` will
// be "IFD" in the TIFF standard.
func (ie *IfdEnumerate) Scan(iiRoot *exifcommon.IfdIdentity, ifdOffset uint32, visitor TagVisitorFn, so *ScanOptions) (med *MiscellaneousExifData, err error) {
	defer func() {
		if state := recover(); state != nil {
			err = log.Wrap(state.(error))
		}
	}()

	// TODO(dustin): Add test

	med = &MiscellaneousExifData{
		unknownTags: make(map[exifcommon.BasicTag]exifcommon.BasicTag),
	}

	err = ie.scan(iiRoot, ifdOffset, visitor, med)
	log.PanicIf(err)

	ifdEnumerateLogger.Debugf(nil, "Scan: It looks like the furthest offset that contained EXIF data in the EXIF blob was (%d) (Scan).", ie.FurthestOffset())

	return med, nil
}

// Ifd represents a single, parsed IFD.
type Ifd struct {
	ifdIdentity *exifcommon.IfdIdentity

	ifdMapping *exifcommon.IfdMapping
	tagIndex   *TagIndex

	offset    uint32
	byteOrder binary.ByteOrder
	id        int

	parentIfd *Ifd

	// ParentTagIndex is our tag position in the parent IFD, if we had a parent
	// (if `ParentIfd` is not nil and we weren't an IFD referenced as a sibling
	// instead of as a child).
	parentTagIndex int

	entries        []*IfdTagEntry
	entriesByTagId map[uint16][]*IfdTagEntry

	children      []*Ifd
	childIfdIndex map[string]*Ifd

	thumbnailData []byte

	nextIfdOffset uint32
	nextIfd       *Ifd
}

// IfdIdentity returns IFD identity that this struct represents.
func (ifd *Ifd) IfdIdentity() *exifcommon.IfdIdentity {
	return ifd.ifdIdentity
}

// Entries returns a flat list of all tags for this IFD.
func (ifd *Ifd) Entries() []*IfdTagEntry {

	// TODO(dustin): Add test

	return ifd.entries
}

// EntriesByTagId returns a map of all tags for this IFD.
func (ifd *Ifd) EntriesByTagId() map[uint16][]*IfdTagEntry {

	// TODO(dustin): Add test

	return ifd.entriesByTagId
}

// Children returns a flat list of all child IFDs of this IFD.
func (ifd *Ifd) Children() []*Ifd {

	// TODO(dustin): Add test

	return ifd.children
}

// ChildWithIfdPath returns a map of all child IFDs of this IFD.
func (ifd *Ifd) ChildIfdIndex() map[string]*Ifd {

	// TODO(dustin): Add test

	return ifd.childIfdIndex
}

// ParentTagIndex returns the position of this IFD's tag in its parent IFD (*if*
// there is a parent).
func (ifd *Ifd) ParentTagIndex() int {

	// TODO(dustin): Add test

	return ifd.parentTagIndex
}

// Offset returns the offset of the IFD in the stream.
func (ifd *Ifd) Offset() uint32 {

	// TODO(dustin): Add test

	return ifd.offset
}

// Offset returns the offset of the IFD in the stream.
func (ifd *Ifd) ByteOrder() binary.ByteOrder {

	// TODO(dustin): Add test

	return ifd.byteOrder
}

// NextIfd returns the Ifd struct for the next IFD in the chain.
func (ifd *Ifd) NextIfd() *Ifd {

	// TODO(dustin): Add test

	return ifd.nextIfd
}

// ChildWithIfdPath returns an `Ifd` struct for the given child of the current
// IFD.
func (ifd *Ifd) ChildWithIfdPath(iiChild *exifcommon.IfdIdentity) (childIfd *Ifd, err error) {
	defer func() {
		if state := recover(); state != nil {
			err = log.Wrap(state.(error))
		}
	}()

	// TODO(dustin): This is a bridge while we're introducing the IFD type-system. We should be able to use the (IfdIdentity).Equals() method for this.
	ifdPath := iiChild.UnindexedString()

	for _, childIfd := range ifd.children {
		if childIfd.ifdIdentity.UnindexedString() == ifdPath {
			return childIfd, nil
		}
	}

	log.Panic(ErrTagNotFound)
	return nil, nil
}

// FindTagWithId returns a list of tags (usually just zero or one) that match
// the given tag ID. This is efficient.
func (ifd *Ifd) FindTagWithId(tagId uint16) (results []*IfdTagEntry, err error) {
	defer func() {
		if state := recover(); state != nil {
			err = log.Wrap(state.(error))
		}
	}()

	results, found := ifd.entriesByTagId[tagId]
	if found != true {
		log.Panic(ErrTagNotFound)
	}

	return results, nil
}

// FindTagWithName returns a list of tags (usually just zero or one) that match
// the given tag name. This is not efficient (though the labor is trivial).
func (ifd *Ifd) FindTagWithName(tagName string) (results []*IfdTagEntry, err error) {
	defer func() {
		if state := recover(); state != nil {
			err = log.Wrap(state.(error))
		}
	}()

	it, err := ifd.tagIndex.GetWithName(ifd.ifdIdentity, tagName)
	if log.Is(err, ErrTagNotFound) == true {
		log.Panic(ErrTagNotKnown)
	} else if err != nil {
		log.Panic(err)
	}

	results = make([]*IfdTagEntry, 0)
	for _, ite := range ifd.entries {
		if ite.TagId() == it.Id {
			results = append(results, ite)
		}
	}

	if len(results) == 0 {
		log.Panic(ErrTagNotFound)
	}

	return results, nil
}

// String returns a description string.
func (ifd *Ifd) String() string {
	parentOffset := uint32(0)
	if ifd.parentIfd != nil {
		parentOffset = ifd.parentIfd.offset
	}

	return fmt.Sprintf("Ifd<ID=(%d) IFD-PATH=[%s] INDEX=(%d) COUNT=(%d) OFF=(0x%04x) CHILDREN=(%d) PARENT=(0x%04x) NEXT-IFD=(0x%04x)>", ifd.id, ifd.ifdIdentity.UnindexedString(), ifd.ifdIdentity.Index(), len(ifd.entries), ifd.offset, len(ifd.children), parentOffset, ifd.nextIfdOffset)
}

// Thumbnail returns the raw thumbnail bytes. This is typically directly
// readable by any standard image viewer.
func (ifd *Ifd) Thumbnail() (data []byte, err error) {

	if ifd.thumbnailData == nil {
		return nil, ErrNoThumbnail
	}

	return ifd.thumbnailData, nil
}

// dumpTags recursively builds a list of tags from an IFD.
func (ifd *Ifd) dumpTags(tags []*IfdTagEntry) []*IfdTagEntry {
	if tags == nil {
		tags = make([]*IfdTagEntry, 0)
	}

	// Now, print the tags while also descending to child-IFDS as we encounter them.

	ifdsFoundCount := 0

	for _, ite := range ifd.entries {
		tags = append(tags, ite)

		childIfdPath := ite.ChildIfdPath()
		if childIfdPath != "" {
			ifdsFoundCount++

			childIfd, found := ifd.childIfdIndex[childIfdPath]
			if found != true {
				log.Panicf("alien child IFD referenced by a tag: [%s]", childIfdPath)
			}

			tags = childIfd.dumpTags(tags)
		}
	}

	if len(ifd.children) != ifdsFoundCount {
		log.Panicf("have one or more dangling child IFDs: (%d) != (%d)", len(ifd.children), ifdsFoundCount)
	}

	if ifd.nextIfd != nil {
		tags = ifd.nextIfd.dumpTags(tags)
	}

	return tags
}

// DumpTags prints the IFD hierarchy.
func (ifd *Ifd) DumpTags() []*IfdTagEntry {
	return ifd.dumpTags(nil)
}

func (ifd *Ifd) printTagTree(populateValues bool, index, level int, nextLink bool) {
	indent := strings.Repeat(" ", level*2)

	prefix := " "
	if nextLink {
		prefix = ">"
	}

	fmt.Printf("%s%sIFD: %s\n", indent, prefix, ifd)

	// Now, print the tags while also descending to child-IFDS as we encounter them.

	ifdsFoundCount := 0

	for _, ite := range ifd.entries {
		if ite.ChildIfdPath() != "" {
			fmt.Printf("%s - TAG: %s\n", indent, ite)
		} else {
			// This will just add noise to the output (byte-tags are fully
			// dumped).
			if ite.IsThumbnailOffset() == true || ite.IsThumbnailSize() == true {
				continue
			}

			it, err := ifd.tagIndex.Get(ifd.ifdIdentity, ite.TagId())

			tagName := ""
			if err == nil {
				tagName = it.Name
			}

			var valuePhrase string
			if populateValues == true {
				var err error

				valuePhrase, err = ite.Format()
				if err != nil {
					if log.Is(err, exifcommon.ErrUnhandledUndefinedTypedTag) == true {
						ifdEnumerateLogger.Warningf(nil, "Skipping non-standard undefined tag: [%s] (%04x)", ifd.ifdIdentity.UnindexedString(), ite.TagId())
						continue
					} else if err == exifundefined.ErrUnparseableValue {
						ifdEnumerateLogger.Warningf(nil, "Skipping unparseable undefined tag: [%s] (%04x) [%s]", ifd.ifdIdentity.UnindexedString(), ite.TagId(), it.Name)
						continue
					}

					log.Panic(err)
				}
			} else {
				valuePhrase = "!UNRESOLVED"
			}

			fmt.Printf("%s - TAG: %s NAME=[%s] VALUE=[%v]\n", indent, ite, tagName, valuePhrase)
		}

		childIfdPath := ite.ChildIfdPath()
		if childIfdPath != "" {
			ifdsFoundCount++

			childIfd, found := ifd.childIfdIndex[childIfdPath]
			if found != true {
				log.Panicf("alien child IFD referenced by a tag: [%s]", childIfdPath)
			}

			childIfd.printTagTree(populateValues, 0, level+1, false)
		}
	}

	if len(ifd.children) != ifdsFoundCount {
		log.Panicf("have one or more dangling child IFDs: (%d) != (%d)", len(ifd.children), ifdsFoundCount)
	}

	if ifd.nextIfd != nil {
		ifd.nextIfd.printTagTree(populateValues, index+1, level, true)
	}
}

// PrintTagTree prints the IFD hierarchy.
func (ifd *Ifd) PrintTagTree(populateValues bool) {
	ifd.printTagTree(populateValues, 0, 0, false)
}

func (ifd *Ifd) printIfdTree(level int, nextLink bool) {
	indent := strings.Repeat(" ", level*2)

	prefix := " "
	if nextLink {
		prefix = ">"
	}

	fmt.Printf("%s%s%s\n", indent, prefix, ifd)

	// Now, print the tags while also descending to child-IFDS as we encounter them.

	ifdsFoundCount := 0

	for _, ite := range ifd.entries {
		childIfdPath := ite.ChildIfdPath()
		if childIfdPath != "" {
			ifdsFoundCount++

			childIfd, found := ifd.childIfdIndex[childIfdPath]
			if found != true {
				log.Panicf("alien child IFD referenced by a tag: [%s]", childIfdPath)
			}

			childIfd.printIfdTree(level+1, false)
		}
	}

	if len(ifd.children) != ifdsFoundCount {
		log.Panicf("have one or more dangling child IFDs: (%d) != (%d)", len(ifd.children), ifdsFoundCount)
	}

	if ifd.nextIfd != nil {
		ifd.nextIfd.printIfdTree(level, true)
	}
}

// PrintIfdTree prints the IFD hierarchy.
func (ifd *Ifd) PrintIfdTree() {
	ifd.printIfdTree(0, false)
}

func (ifd *Ifd) dumpTree(tagsDump []string, level int) []string {
	if tagsDump == nil {
		tagsDump = make([]string, 0)
	}

	indent := strings.Repeat(" ", level*2)

	var ifdPhrase string
	if ifd.parentIfd != nil {
		ifdPhrase = fmt.Sprintf("[%s]->[%s]:(%d)", ifd.parentIfd.ifdIdentity.UnindexedString(), ifd.ifdIdentity.UnindexedString(), ifd.ifdIdentity.Index())
	} else {
		ifdPhrase = fmt.Sprintf("[ROOT]->[%s]:(%d)", ifd.ifdIdentity.UnindexedString(), ifd.ifdIdentity.Index())
	}

	startBlurb := fmt.Sprintf("%s> IFD %s TOP", indent, ifdPhrase)
	tagsDump = append(tagsDump, startBlurb)

	ifdsFoundCount := 0
	for _, ite := range ifd.entries {
		tagsDump = append(tagsDump, fmt.Sprintf("%s  - (0x%04x)", indent, ite.TagId()))

		childIfdPath := ite.ChildIfdPath()
		if childIfdPath != "" {
			ifdsFoundCount++

			childIfd, found := ifd.childIfdIndex[childIfdPath]
			if found != true {
				log.Panicf("alien child IFD referenced by a tag: [%s]", childIfdPath)
			}

			tagsDump = childIfd.dumpTree(tagsDump, level+1)
		}
	}

	if len(ifd.children) != ifdsFoundCount {
		log.Panicf("have one or more dangling child IFDs: (%d) != (%d)", len(ifd.children), ifdsFoundCount)
	}

	finishBlurb := fmt.Sprintf("%s< IFD %s BOTTOM", indent, ifdPhrase)
	tagsDump = append(tagsDump, finishBlurb)

	if ifd.nextIfd != nil {
		siblingBlurb := fmt.Sprintf("%s* LINKING TO SIBLING IFD [%s]:(%d)", indent, ifd.nextIfd.ifdIdentity.UnindexedString(), ifd.nextIfd.ifdIdentity.Index())
		tagsDump = append(tagsDump, siblingBlurb)

		tagsDump = ifd.nextIfd.dumpTree(tagsDump, level)
	}

	return tagsDump
}

// DumpTree returns a list of strings describing the IFD hierarchy.
func (ifd *Ifd) DumpTree() []string {
	return ifd.dumpTree(nil, 0)
}

// GpsInfo parses and consolidates the GPS info. This can only be called on the
// GPS IFD.
func (ifd *Ifd) GpsInfo() (gi *GpsInfo, err error) {
	defer func() {
		if state := recover(); state != nil {
			err = log.Wrap(state.(error))
		}
	}()

	gi = new(GpsInfo)

	if ifd.ifdIdentity.Equals(exifcommon.IfdGpsInfoStandardIfdIdentity) == false {
		log.Panicf("GPS can only be read on GPS IFD: [%s]", ifd.ifdIdentity.UnindexedString())
	}

	if tags, found := ifd.entriesByTagId[TagGpsVersionId]; found == false {
		// We've seen this. We'll just have to default to assuming we're in a
		// 2.2.0.0 format.
		ifdEnumerateLogger.Warningf(nil, "No GPS version tag (0x%04x) found.", TagGpsVersionId)
	} else {
		versionBytes, err := tags[0].GetRawBytes()
		log.PanicIf(err)

		hit := false
		for _, acceptedGpsVersion := range ValidGpsVersions {
			if bytes.Compare(versionBytes, acceptedGpsVersion[:]) == 0 {
				hit = true
				break
			}
		}

		if hit != true {
			ifdEnumerateLogger.Warningf(nil, "GPS version not supported: %v", versionBytes)
			log.Panic(ErrNoGpsTags)
		}
	}

	tags, found := ifd.entriesByTagId[TagLatitudeId]
	if found == false {
		ifdEnumerateLogger.Warningf(nil, "latitude not found")
		log.Panic(ErrNoGpsTags)
	}

	latitudeValue, err := tags[0].Value()
	log.PanicIf(err)

	// Look for whether North or South.
	tags, found = ifd.entriesByTagId[TagLatitudeRefId]
	if found == false {
		ifdEnumerateLogger.Warningf(nil, "latitude-ref not found")
		log.Panic(ErrNoGpsTags)
	}

	latitudeRefValue, err := tags[0].Value()
	log.PanicIf(err)

	tags, found = ifd.entriesByTagId[TagLongitudeId]
	if found == false {
		ifdEnumerateLogger.Warningf(nil, "longitude not found")
		log.Panic(ErrNoGpsTags)
	}

	longitudeValue, err := tags[0].Value()
	log.PanicIf(err)

	// Look for whether West or East.
	tags, found = ifd.entriesByTagId[TagLongitudeRefId]
	if found == false {
		ifdEnumerateLogger.Warningf(nil, "longitude-ref not found")
		log.Panic(ErrNoGpsTags)
	}

	longitudeRefValue, err := tags[0].Value()
	log.PanicIf(err)

	// Parse location.

	latitudeRaw := latitudeValue.([]exifcommon.Rational)

	gi.Latitude, err = NewGpsDegreesFromRationals(latitudeRefValue.(string), latitudeRaw)
	log.PanicIf(err)

	longitudeRaw := longitudeValue.([]exifcommon.Rational)

	gi.Longitude, err = NewGpsDegreesFromRationals(longitudeRefValue.(string), longitudeRaw)
	log.PanicIf(err)

	// Parse altitude.

	altitudeTags, foundAltitude := ifd.entriesByTagId[TagAltitudeId]
	altitudeRefTags, foundAltitudeRef := ifd.entriesByTagId[TagAltitudeRefId]

	if foundAltitude == true && foundAltitudeRef == true {
		altitudePhrase, err := altitudeTags[0].Format()
		log.PanicIf(err)

		ifdEnumerateLogger.Debugf(nil, "Altitude is [%s].", altitudePhrase)

		altitudeValue, err := altitudeTags[0].Value()
		log.PanicIf(err)

		altitudeRefPhrase, err := altitudeRefTags[0].Format()
		log.PanicIf(err)

		ifdEnumerateLogger.Debugf(nil, "Altitude-reference is [%s].", altitudeRefPhrase)

		altitudeRefValue, err := altitudeRefTags[0].Value()
		log.PanicIf(err)

		altitudeRaw := altitudeValue.([]exifcommon.Rational)
		if altitudeRaw[0].Denominator > 0 {
			altitude := int(altitudeRaw[0].Numerator / altitudeRaw[0].Denominator)

			if altitudeRefValue.([]byte)[0] == 1 {
				altitude *= -1
			}

			gi.Altitude = altitude
		}
	}

	// Parse timestamp from separate date and time tags.

	timestampTags, foundTimestamp := ifd.entriesByTagId[TagTimestampId]
	datestampTags, foundDatestamp := ifd.entriesByTagId[TagDatestampId]

	if foundTimestamp == true && foundDatestamp == true {
		datestampValue, err := datestampTags[0].Value()
		log.PanicIf(err)

		datePhrase := datestampValue.(string)
		ifdEnumerateLogger.Debugf(nil, "Date tag value is [%s].", datePhrase)

		// Normalize the separators.
		datePhrase = strings.ReplaceAll(datePhrase, "-", ":")

		dateParts := strings.Split(datePhrase, ":")

		year, err1 := strconv.ParseUint(dateParts[0], 10, 16)
		month, err2 := strconv.ParseUint(dateParts[1], 10, 8)
		day, err3 := strconv.ParseUint(dateParts[2], 10, 8)

		if err1 == nil && err2 == nil && err3 == nil {
			timestampValue, err := timestampTags[0].Value()
			log.PanicIf(err)

			timePhrase, err := timestampTags[0].Format()
			log.PanicIf(err)

			ifdEnumerateLogger.Debugf(nil, "Time tag value is [%s].", timePhrase)

			timestampRaw := timestampValue.([]exifcommon.Rational)

			hour := int(timestampRaw[0].Numerator / timestampRaw[0].Denominator)
			minute := int(timestampRaw[1].Numerator / timestampRaw[1].Denominator)
			second := int(timestampRaw[2].Numerator / timestampRaw[2].Denominator)

			gi.Timestamp = time.Date(int(year), time.Month(month), int(day), hour, minute, second, 0, time.UTC)
		}
	}

	return gi, nil
}

// ParsedTagVisitor is a callback used if wanting to visit through all tags and
// child IFDs from the current IFD and going down.
type ParsedTagVisitor func(*Ifd, *IfdTagEntry) error

// EnumerateTagsRecursively calls the given visitor function for every tag and
// IFD in the current IFD, recursively.
func (ifd *Ifd) EnumerateTagsRecursively(visitor ParsedTagVisitor) (err error) {
	defer func() {
		if state := recover(); state != nil {
			err = log.Wrap(state.(error))
		}
	}()

	for ptr := ifd; ptr != nil; ptr = ptr.nextIfd {
		for _, ite := range ifd.entries {
			childIfdPath := ite.ChildIfdPath()
			if childIfdPath != "" {
				childIfd := ifd.childIfdIndex[childIfdPath]

				err := childIfd.EnumerateTagsRecursively(visitor)
				log.PanicIf(err)
			} else {
				err := visitor(ifd, ite)
				log.PanicIf(err)
			}
		}
	}

	return nil
}

// QueuedIfd is one IFD that has been identified but yet to be processed.
type QueuedIfd struct {
	IfdIdentity *exifcommon.IfdIdentity

	Offset uint32
	Parent *Ifd

	// ParentTagIndex is our tag position in the parent IFD, if we had a parent
	// (if `ParentIfd` is not nil and we weren't an IFD referenced as a sibling
	// instead of as a child).
	ParentTagIndex int
}

// IfdIndex collects a bunch of IFD and tag information stored in several
// different ways in order to provide convenient lookups.
type IfdIndex struct {
	RootIfd *Ifd
	Ifds    []*Ifd
	Tree    map[int]*Ifd
	Lookup  map[string]*Ifd
}

// Collect enumerates the different EXIF blocks (called IFDs) and builds out an
// index struct for referencing all of the parsed data.
func (ie *IfdEnumerate) Collect(rootIfdOffset uint32) (index IfdIndex, err error) {
	defer func() {
		if state := recover(); state != nil {
			err = log.Wrap(state.(error))
		}
	}()

	// TODO(dustin): Add MiscellaneousExifData to IfdIndex

	tree := make(map[int]*Ifd)
	ifds := make([]*Ifd, 0)
	lookup := make(map[string]*Ifd)

	queue := []QueuedIfd{
		{
			IfdIdentity: exifcommon.IfdStandardIfdIdentity,
			Offset:      rootIfdOffset,
		},
	}

	edges := make(map[uint32]*Ifd)

	for {
		if len(queue) == 0 {
			break
		}

		qi := queue[0]
		ii := qi.IfdIdentity

		offset := qi.Offset
		parentIfd := qi.Parent

		queue = queue[1:]

		ifdEnumerateLogger.Debugf(nil, "Parsing IFD [%s] (%d) at offset (0x%04x) (Collect).", ii.String(), ii.Index(), offset)

		bp, err := ie.getByteParser(offset)
		if err != nil {
			if err == ErrOffsetInvalid {
				return index, err
			}

			log.Panic(err)
		}

		// TODO(dustin): We don't need to pass the index in as a separate argument. Get from the II.

		nextIfdOffset, entries, thumbnailData, err := ie.parseIfd(ii, bp, nil, false, nil)
		log.PanicIf(err)

		currentOffset := bp.CurrentOffset()
		if currentOffset > ie.furthestOffset {
			ie.furthestOffset = currentOffset
		}

		id := len(ifds)

		entriesByTagId := make(map[uint16][]*IfdTagEntry)
		for _, ite := range entries {
			tagId := ite.TagId()

			tags, found := entriesByTagId[tagId]
			if found == false {
				tags = make([]*IfdTagEntry, 0)
			}

			entriesByTagId[tagId] = append(tags, ite)
		}

		ifd := &Ifd{
			ifdIdentity: ii,

			byteOrder: ie.byteOrder,

			id: id,

			parentIfd:      parentIfd,
			parentTagIndex: qi.ParentTagIndex,

			offset:         offset,
			entries:        entries,
			entriesByTagId: entriesByTagId,

			// This is populated as each child is processed.
			children: make([]*Ifd, 0),

			nextIfdOffset: nextIfdOffset,
			thumbnailData: thumbnailData,

			ifdMapping: ie.ifdMapping,
			tagIndex:   ie.tagIndex,
		}

		// Add ourselves to a big list of IFDs.
		ifds = append(ifds, ifd)

		// Install ourselves into a by-id lookup table (keys are unique).
		tree[id] = ifd

		// Install into by-name buckets.
		lookup[ii.String()] = ifd

		// Add a link from the previous IFD in the chain to us.
		if previousIfd, found := edges[offset]; found == true {
			previousIfd.nextIfd = ifd
		}

		// Attach as a child to our parent (where we appeared as a tag in
		// that IFD).
		if parentIfd != nil {
			parentIfd.children = append(parentIfd.children, ifd)
		}

		// Determine if any of our entries is a child IFD and queue it.
		for i, ite := range entries {
			if ite.ChildIfdPath() == "" {
				continue
			}

			tagId := ite.TagId()
			childIfdName := ite.ChildIfdName()

			currentIfdTag := ii.IfdTag()

			childIfdTag :=
				exifcommon.NewIfdTag(
					&currentIfdTag,
					tagId,
					childIfdName)

			iiChild := ii.NewChild(childIfdTag, 0)

			qi := QueuedIfd{
				IfdIdentity: iiChild,

				Offset:         ite.getValueOffset(),
				Parent:         ifd,
				ParentTagIndex: i,
			}

			queue = append(queue, qi)
		}

		// If there's another IFD in the chain.
		if nextIfdOffset != 0 {
			iiSibling := ii.NewSibling(ii.Index() + 1)

			// Allow the next link to know what the previous link was.
			edges[nextIfdOffset] = ifd

			qi := QueuedIfd{
				IfdIdentity: iiSibling,
				Offset:      nextIfdOffset,
			}

			queue = append(queue, qi)
		}
	}

	index.RootIfd = tree[0]
	index.Ifds = ifds
	index.Tree = tree
	index.Lookup = lookup

	err = ie.setChildrenIndex(index.RootIfd)
	log.PanicIf(err)

	ifdEnumerateLogger.Debugf(nil, "Collect: It looks like the furthest offset that contained EXIF data in the EXIF blob was (%d).", ie.FurthestOffset())

	return index, nil
}

func (ie *IfdEnumerate) setChildrenIndex(ifd *Ifd) (err error) {
	defer func() {
		if state := recover(); state != nil {
			err = log.Wrap(state.(error))
		}
	}()

	childIfdIndex := make(map[string]*Ifd)
	for _, childIfd := range ifd.children {
		childIfdIndex[childIfd.ifdIdentity.UnindexedString()] = childIfd
	}

	ifd.childIfdIndex = childIfdIndex

	for _, childIfd := range ifd.children {
		err := ie.setChildrenIndex(childIfd)
		log.PanicIf(err)
	}

	return nil
}

// FurthestOffset returns the furthest offset visited in the EXIF blob. This
// *does not* account for the locations of any undefined tags since we always
// evaluate the furthest offset, whether or not the user wants to know it.
//
// We are not willing to incur the cost of actually parsing those tags just to
// know their length when there are still undefined tags that are out there
// that we still won't have any idea how to parse, thus making this an
// approximation regardless of how clever we get.
func (ie *IfdEnumerate) FurthestOffset() uint32 {

	// TODO(dustin): Add test

	return ie.furthestOffset
}

// parseOneIfd is a hack to use an IE to parse a raw IFD block. Can be used for
// testing. The fqIfdPath ("fully-qualified IFD path") will be less qualified
// in that the numeric index will always be zero (the zeroth child) rather than
// the proper number (if its actually a sibling to the first child, for
// instance).
func parseOneIfd(ifdMapping *exifcommon.IfdMapping, tagIndex *TagIndex, ii *exifcommon.IfdIdentity, byteOrder binary.ByteOrder, ifdBlock []byte, visitor TagVisitorFn) (nextIfdOffset uint32, entries []*IfdTagEntry, err error) {
	defer func() {
		if state := recover(); state != nil {
			err = log.Wrap(state.(error))
		}
	}()

	// TODO(dustin): Add test

	ebs := NewExifReadSeekerWithBytes(ifdBlock)

	rs, err := ebs.GetReadSeeker(0)
	log.PanicIf(err)

	bp, err := newByteParser(rs, byteOrder, 0)
	if err != nil {
		if err == ErrOffsetInvalid {
			return 0, nil, err
		}

		log.Panic(err)
	}

	dummyEbs := NewExifReadSeekerWithBytes([]byte{})
	ie := NewIfdEnumerate(ifdMapping, tagIndex, dummyEbs, byteOrder)

	nextIfdOffset, entries, _, err = ie.parseIfd(ii, bp, visitor, true, nil)
	log.PanicIf(err)

	return nextIfdOffset, entries, nil
}

// parseOneTag is a hack to use an IE to parse a raw tag block.
func parseOneTag(ifdMapping *exifcommon.IfdMapping, tagIndex *TagIndex, ii *exifcommon.IfdIdentity, byteOrder binary.ByteOrder, tagBlock []byte) (ite *IfdTagEntry, err error) {
	defer func() {
		if state := recover(); state != nil {
			err = log.Wrap(state.(error))
		}
	}()

	// TODO(dustin): Add test

	ebs := NewExifReadSeekerWithBytes(tagBlock)

	rs, err := ebs.GetReadSeeker(0)
	log.PanicIf(err)

	bp, err := newByteParser(rs, byteOrder, 0)
	if err != nil {
		if err == ErrOffsetInvalid {
			return nil, err
		}

		log.Panic(err)
	}

	dummyEbs := NewExifReadSeekerWithBytes([]byte{})
	ie := NewIfdEnumerate(ifdMapping, tagIndex, dummyEbs, byteOrder)

	ite, err = ie.parseTag(ii, 0, bp)
	log.PanicIf(err)

	err = ie.tagPostParse(ite, nil)
	if err != nil {
		if err == ErrTagNotFound {
			return nil, err
		}

		log.Panic(err)
	}

	return ite, nil
}

// FindIfdFromRootIfd returns the given `Ifd` given the root-IFD and path of the
// desired IFD.
func FindIfdFromRootIfd(rootIfd *Ifd, ifdPath string) (ifd *Ifd, err error) {
	defer func() {
		if state := recover(); state != nil {
			err = log.Wrap(state.(error))
		}
	}()

	// TODO(dustin): !! Add test.

	lineage, err := rootIfd.ifdMapping.ResolvePath(ifdPath)
	log.PanicIf(err)

	// Confirm the first IFD is our root IFD type, and then prune it because
	// from then on we'll be searching down through our children.

	if len(lineage) == 0 {
		log.Panicf("IFD path must be non-empty.")
	} else if lineage[0].Name != exifcommon.IfdStandardIfdIdentity.Name() {
		log.Panicf("First IFD path item must be [%s].", exifcommon.IfdStandardIfdIdentity.Name())
	}

	desiredRootIndex := lineage[0].Index
	lineage = lineage[1:]

	// TODO(dustin): !! This is a poorly conceived fix that just doubles the work we already have to do below, which then interacts badly with the indices not being properly represented in the IFD-phrase.
	// TODO(dustin): !! <-- However, we're not sure whether we shouldn't store a secondary IFD-path with the indices. Some IFDs may not necessarily restrict which IFD indices they can be a child of (only the IFD itself matters). Validation should be delegated to the caller.
	thisIfd := rootIfd
	for currentRootIndex := 0; currentRootIndex < desiredRootIndex; currentRootIndex++ {
		if thisIfd.nextIfd == nil {
			log.Panicf("Root-IFD index (%d) does not exist in the data.", currentRootIndex)
		}

		thisIfd = thisIfd.nextIfd
	}

	for _, itii := range lineage {
		var hit *Ifd
		for _, childIfd := range thisIfd.children {
			if childIfd.ifdIdentity.TagId() == itii.TagId {
				hit = childIfd
				break
			}
		}

		// If we didn't find the child, add it.
		if hit == nil {
			log.Panicf("IFD [%s] in [%s] not found: %s", itii.Name, ifdPath, thisIfd.children)
		}

		thisIfd = hit

		// If we didn't find the sibling, add it.
		for i := 0; i < itii.Index; i++ {
			if thisIfd.nextIfd == nil {
				log.Panicf("IFD [%s] does not have (%d) occurrences/siblings", thisIfd.ifdIdentity.UnindexedString(), itii.Index)
			}

			thisIfd = thisIfd.nextIfd
		}
	}

	return thisIfd, nil
}
