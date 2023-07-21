package exif

import (
	"fmt"
	"sync"

	"github.com/dsoprea/go-logging"
	"gopkg.in/yaml.v2"

	"github.com/dsoprea/go-exif/v3/common"
)

const (
	// IFD1

	// ThumbnailFqIfdPath is the fully-qualified IFD path that the thumbnail
	// must be found in.
	ThumbnailFqIfdPath = "IFD1"

	// ThumbnailOffsetTagId returns the tag-ID of the thumbnail offset.
	ThumbnailOffsetTagId = 0x0201

	// ThumbnailSizeTagId returns the tag-ID of the thumbnail size.
	ThumbnailSizeTagId = 0x0202
)

const (
	// GPS

	// TagGpsVersionId is the ID of the GPS version tag.
	TagGpsVersionId = 0x0000

	// TagLatitudeId is the ID of the GPS latitude tag.
	TagLatitudeId = 0x0002

	// TagLatitudeRefId is the ID of the GPS latitude orientation tag.
	TagLatitudeRefId = 0x0001

	// TagLongitudeId is the ID of the GPS longitude tag.
	TagLongitudeId = 0x0004

	// TagLongitudeRefId is the ID of the GPS longitude-orientation tag.
	TagLongitudeRefId = 0x0003

	// TagTimestampId is the ID of the GPS time tag.
	TagTimestampId = 0x0007

	// TagDatestampId is the ID of the GPS date tag.
	TagDatestampId = 0x001d

	// TagAltitudeId is the ID of the GPS altitude tag.
	TagAltitudeId = 0x0006

	// TagAltitudeRefId is the ID of the GPS altitude-orientation tag.
	TagAltitudeRefId = 0x0005
)

var (
	// tagsWithoutAlignment is a tag-lookup for tags whose value size won't
	// necessarily be a multiple of its tag-type.
	tagsWithoutAlignment = map[uint16]struct{}{
		// The thumbnail offset is stored as a long, but its data is a binary
		// blob (not a slice of longs).
		ThumbnailOffsetTagId: {},
	}
)

var (
	tagsLogger = log.NewLogger("exif.tags")
)

// File structures.

type encodedTag struct {
	// id is signed, here, because YAML doesn't have enough information to
	// support unsigned.
	Id        int      `yaml:"id"`
	Name      string   `yaml:"name"`
	TypeName  string   `yaml:"type_name"`
	TypeNames []string `yaml:"type_names"`
}

// Indexing structures.

// IndexedTag describes one index lookup result.
type IndexedTag struct {
	// Id is the tag-ID.
	Id uint16

	// Name is the tag name.
	Name string

	// IfdPath is the proper IFD path of this tag. This is not fully-qualified.
	IfdPath string

	// SupportedTypes is an unsorted list of allowed tag-types.
	SupportedTypes []exifcommon.TagTypePrimitive
}

// String returns a descriptive string.
func (it *IndexedTag) String() string {
	return fmt.Sprintf("TAG<ID=(0x%04x) NAME=[%s] IFD=[%s]>", it.Id, it.Name, it.IfdPath)
}

// IsName returns true if this tag matches the given tag name.
func (it *IndexedTag) IsName(ifdPath, name string) bool {
	return it.Name == name && it.IfdPath == ifdPath
}

// Is returns true if this tag matched the given tag ID.
func (it *IndexedTag) Is(ifdPath string, id uint16) bool {
	return it.Id == id && it.IfdPath == ifdPath
}

// GetEncodingType returns the largest type that this tag's value can occupy.
func (it *IndexedTag) GetEncodingType(value interface{}) exifcommon.TagTypePrimitive {
	// For convenience, we handle encoding a `time.Time` directly.
	if exifcommon.IsTime(value) == true {
		// Timestamps are encoded as ASCII.
		value = ""
	}

	if len(it.SupportedTypes) == 0 {
		log.Panicf("IndexedTag [%s] (%d) has no supported types.", it.IfdPath, it.Id)
	} else if len(it.SupportedTypes) == 1 {
		return it.SupportedTypes[0]
	}

	supportsLong := false
	supportsShort := false
	supportsRational := false
	supportsSignedRational := false
	for _, supportedType := range it.SupportedTypes {
		if supportedType == exifcommon.TypeLong {
			supportsLong = true
		} else if supportedType == exifcommon.TypeShort {
			supportsShort = true
		} else if supportedType == exifcommon.TypeRational {
			supportsRational = true
		} else if supportedType == exifcommon.TypeSignedRational {
			supportsSignedRational = true
		}
	}

	// We specifically check for the cases that we know to expect.

	if supportsLong == true && supportsShort == true {
		return exifcommon.TypeLong
	} else if supportsRational == true && supportsSignedRational == true {
		if value == nil {
			log.Panicf("GetEncodingType: require value to be given")
		}

		if _, ok := value.(exifcommon.SignedRational); ok == true {
			return exifcommon.TypeSignedRational
		}

		return exifcommon.TypeRational
	}

	log.Panicf("WidestSupportedType() case is not handled for tag [%s] (0x%04x): %v", it.IfdPath, it.Id, it.SupportedTypes)
	return 0
}

// DoesSupportType returns true if this tag can be found/decoded with this type.
func (it *IndexedTag) DoesSupportType(tagType exifcommon.TagTypePrimitive) bool {
	// This is always a very small collection. So, we keep it unsorted.
	for _, thisTagType := range it.SupportedTypes {
		if thisTagType == tagType {
			return true
		}
	}

	return false
}

// TagIndex is a tag-lookup facility.
type TagIndex struct {
	tagsByIfd  map[string]map[uint16]*IndexedTag
	tagsByIfdR map[string]map[string]*IndexedTag

	mutex sync.Mutex

	doUniversalSearch bool
}

// NewTagIndex returns a new TagIndex struct.
func NewTagIndex() *TagIndex {
	ti := new(TagIndex)

	ti.tagsByIfd = make(map[string]map[uint16]*IndexedTag)
	ti.tagsByIfdR = make(map[string]map[string]*IndexedTag)

	return ti
}

// SetUniversalSearch enables a fallback to matching tags under *any* IFD.
func (ti *TagIndex) SetUniversalSearch(flag bool) {
	ti.doUniversalSearch = flag
}

// UniversalSearch enables a fallback to matching tags under *any* IFD.
func (ti *TagIndex) UniversalSearch() bool {
	return ti.doUniversalSearch
}

// Add registers a new tag to be recognized during the parse.
func (ti *TagIndex) Add(it *IndexedTag) (err error) {
	defer func() {
		if state := recover(); state != nil {
			err = log.Wrap(state.(error))
		}
	}()

	ti.mutex.Lock()
	defer ti.mutex.Unlock()

	// Store by ID.

	family, found := ti.tagsByIfd[it.IfdPath]
	if found == false {
		family = make(map[uint16]*IndexedTag)
		ti.tagsByIfd[it.IfdPath] = family
	}

	if _, found := family[it.Id]; found == true {
		log.Panicf("tag-ID defined more than once for IFD [%s]: (%02x)", it.IfdPath, it.Id)
	}

	family[it.Id] = it

	// Store by name.

	familyR, found := ti.tagsByIfdR[it.IfdPath]
	if found == false {
		familyR = make(map[string]*IndexedTag)
		ti.tagsByIfdR[it.IfdPath] = familyR
	}

	if _, found := familyR[it.Name]; found == true {
		log.Panicf("tag-name defined more than once for IFD [%s]: (%s)", it.IfdPath, it.Name)
	}

	familyR[it.Name] = it

	return nil
}

func (ti *TagIndex) getOne(ifdPath string, id uint16) (it *IndexedTag, err error) {
	defer func() {
		if state := recover(); state != nil {
			err = log.Wrap(state.(error))
		}
	}()

	if len(ti.tagsByIfd) == 0 {
		err := LoadStandardTags(ti)
		log.PanicIf(err)
	}

	ti.mutex.Lock()
	defer ti.mutex.Unlock()

	family, found := ti.tagsByIfd[ifdPath]
	if found == false {
		return nil, ErrTagNotFound
	}

	it, found = family[id]
	if found == false {
		return nil, ErrTagNotFound
	}

	return it, nil
}

// Get returns information about the non-IFD tag given a tag ID. `ifdPath` must
// not be fully-qualified.
func (ti *TagIndex) Get(ii *exifcommon.IfdIdentity, id uint16) (it *IndexedTag, err error) {
	defer func() {
		if state := recover(); state != nil {
			err = log.Wrap(state.(error))
		}
	}()

	ifdPath := ii.UnindexedString()

	it, err = ti.getOne(ifdPath, id)
	if err == nil {
		return it, nil
	} else if err != ErrTagNotFound {
		log.Panic(err)
	}

	if ti.doUniversalSearch == false {
		return nil, ErrTagNotFound
	}

	// We've been told to fallback to look for the tag in other IFDs.

	skipIfdPath := ii.UnindexedString()

	for currentIfdPath, _ := range ti.tagsByIfd {
		if currentIfdPath == skipIfdPath {
			// Skip the primary IFD, which has already been checked.
			continue
		}

		it, err = ti.getOne(currentIfdPath, id)
		if err == nil {
			tagsLogger.Warningf(nil,
				"Found tag (0x%02x) in the wrong IFD: [%s] != [%s]",
				id, currentIfdPath, ifdPath)

			return it, nil
		} else if err != ErrTagNotFound {
			log.Panic(err)
		}
	}

	return nil, ErrTagNotFound
}

var (
	// tagGuessDefaultIfdIdentities describes which IFDs we'll look for a given
	// tag-ID in, if it's not found where it's supposed to be. We suppose that
	// Exif-IFD tags might be found in IFD0 or IFD1, or IFD0/IFD1 tags might be
	// found in the Exif IFD. This is the only thing we've seen so far. So, this
	// is the limit of our guessing.
	tagGuessDefaultIfdIdentities = []*exifcommon.IfdIdentity{
		exifcommon.IfdExifStandardIfdIdentity,
		exifcommon.IfdStandardIfdIdentity,
	}
)

// FindFirst looks for the given tag-ID in each of the given IFDs in the given
// order. If `fqIfdPaths` is `nil` then use a default search order. This defies
// the standard, which requires each tag to exist in certain IFDs. This is a
// contingency to make recommendations for malformed data.
//
// Things *can* end badly here, in that the same tag-ID in different IFDs might
// describe different data and different ata-types, and our decode might then
// produce binary and non-printable data.
func (ti *TagIndex) FindFirst(id uint16, typeId exifcommon.TagTypePrimitive, ifdIdentities []*exifcommon.IfdIdentity) (it *IndexedTag, err error) {
	defer func() {
		if state := recover(); state != nil {
			err = log.Wrap(state.(error))
		}
	}()

	if ifdIdentities == nil {
		ifdIdentities = tagGuessDefaultIfdIdentities
	}

	for _, ii := range ifdIdentities {
		it, err := ti.Get(ii, id)
		if err != nil {
			if err == ErrTagNotFound {
				continue
			}

			log.Panic(err)
		}

		// Even though the tag might be mislocated, the type should still be the
		// same. Check this so we don't accidentally end-up on a complete
		// irrelevant tag with a totally different data type. This attempts to
		// mitigate producing garbage.
		for _, supportedType := range it.SupportedTypes {
			if supportedType == typeId {
				return it, nil
			}
		}
	}

	return nil, ErrTagNotFound
}

// GetWithName returns information about the non-IFD tag given a tag name.
func (ti *TagIndex) GetWithName(ii *exifcommon.IfdIdentity, name string) (it *IndexedTag, err error) {
	defer func() {
		if state := recover(); state != nil {
			err = log.Wrap(state.(error))
		}
	}()

	if len(ti.tagsByIfdR) == 0 {
		err := LoadStandardTags(ti)
		log.PanicIf(err)
	}

	ifdPath := ii.UnindexedString()

	it, found := ti.tagsByIfdR[ifdPath][name]
	if found != true {
		log.Panic(ErrTagNotFound)
	}

	return it, nil
}

// LoadStandardTags registers the tags that all devices/applications should
// support.
func LoadStandardTags(ti *TagIndex) (err error) {
	defer func() {
		if state := recover(); state != nil {
			err = log.Wrap(state.(error))
		}
	}()

	// Read static data.

	encodedIfds := make(map[string][]encodedTag)

	err = yaml.Unmarshal([]byte(tagsYaml), encodedIfds)
	log.PanicIf(err)

	// Load structure.

	count := 0
	for ifdPath, tags := range encodedIfds {
		for _, tagInfo := range tags {
			tagId := uint16(tagInfo.Id)
			tagName := tagInfo.Name
			tagTypeName := tagInfo.TypeName
			tagTypeNames := tagInfo.TypeNames

			if tagTypeNames == nil {
				if tagTypeName == "" {
					log.Panicf("no tag-types were given when registering standard tag [%s] (0x%04x) [%s]", ifdPath, tagId, tagName)
				}

				tagTypeNames = []string{
					tagTypeName,
				}
			} else if tagTypeName != "" {
				log.Panicf("both 'type_names' and 'type_name' were given when registering standard tag [%s] (0x%04x) [%s]", ifdPath, tagId, tagName)
			}

			tagTypes := make([]exifcommon.TagTypePrimitive, 0)
			for _, tagTypeName := range tagTypeNames {

				// TODO(dustin): Discard unsupported types. This helps us with non-standard types that have actually been found in real data, that we ignore for right now. e.g. SSHORT, FLOAT, DOUBLE
				tagTypeId, found := exifcommon.GetTypeByName(tagTypeName)
				if found == false {
					tagsLogger.Warningf(nil, "Type [%s] for tag [%s] being loaded is not valid and is being ignored.", tagTypeName, tagName)
					continue
				}

				tagTypes = append(tagTypes, tagTypeId)
			}

			if len(tagTypes) == 0 {
				tagsLogger.Warningf(nil, "Tag [%s] (0x%04x) [%s] being loaded does not have any supported types and will not be registered.", ifdPath, tagId, tagName)
				continue
			}

			it := &IndexedTag{
				IfdPath:        ifdPath,
				Id:             tagId,
				Name:           tagName,
				SupportedTypes: tagTypes,
			}

			err = ti.Add(it)
			log.PanicIf(err)

			count++
		}
	}

	tagsLogger.Debugf(nil, "(%d) tags loaded.", count)

	return nil
}
