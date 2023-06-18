package exifcommon

import (
	"errors"
	"fmt"
	"strings"

	"github.com/dsoprea/go-logging"
)

var (
	ifdLogger = log.NewLogger("exifcommon.ifd")
)

var (
	ErrChildIfdNotMapped = errors.New("no child-IFD for that tag-ID under parent")
)

// MappedIfd is one node in the IFD-mapping.
type MappedIfd struct {
	ParentTagId uint16
	Placement   []uint16
	Path        []string

	Name     string
	TagId    uint16
	Children map[uint16]*MappedIfd
}

// String returns a descriptive string.
func (mi *MappedIfd) String() string {
	pathPhrase := mi.PathPhrase()
	return fmt.Sprintf("MappedIfd<(0x%04X) [%s] PATH=[%s]>", mi.TagId, mi.Name, pathPhrase)
}

// PathPhrase returns a non-fully-qualified IFD path.
func (mi *MappedIfd) PathPhrase() string {
	return strings.Join(mi.Path, "/")
}

// TODO(dustin): Refactor this to use IfdIdentity structs.

// IfdMapping describes all of the IFDs that we currently recognize.
type IfdMapping struct {
	rootNode *MappedIfd
}

// NewIfdMapping returns a new IfdMapping struct.
func NewIfdMapping() (ifdMapping *IfdMapping) {
	rootNode := &MappedIfd{
		Path:     make([]string, 0),
		Children: make(map[uint16]*MappedIfd),
	}

	return &IfdMapping{
		rootNode: rootNode,
	}
}

// NewIfdMappingWithStandard retruns a new IfdMapping struct preloaded with the
// standard IFDs.
func NewIfdMappingWithStandard() (ifdMapping *IfdMapping, err error) {
	defer func() {
		if state := recover(); state != nil {
			err = log.Wrap(state.(error))
		}
	}()

	im := NewIfdMapping()

	err = LoadStandardIfds(im)
	log.PanicIf(err)

	return im, nil
}

// Get returns the node given the path slice.
func (im *IfdMapping) Get(parentPlacement []uint16) (childIfd *MappedIfd, err error) {
	defer func() {
		if state := recover(); state != nil {
			err = log.Wrap(state.(error))
		}
	}()

	ptr := im.rootNode
	for _, tagId := range parentPlacement {
		if descendantPtr, found := ptr.Children[tagId]; found == false {
			log.Panicf("ifd child with tag-ID (%04x) not registered: [%s]", tagId, ptr.PathPhrase())
		} else {
			ptr = descendantPtr
		}
	}

	return ptr, nil
}

// GetWithPath returns the node given the path string.
func (im *IfdMapping) GetWithPath(pathPhrase string) (mi *MappedIfd, err error) {
	defer func() {
		if state := recover(); state != nil {
			err = log.Wrap(state.(error))
		}
	}()

	if pathPhrase == "" {
		log.Panicf("path-phrase is empty")
	}

	path := strings.Split(pathPhrase, "/")
	ptr := im.rootNode

	for _, name := range path {
		var hit *MappedIfd
		for _, mi := range ptr.Children {
			if mi.Name == name {
				hit = mi
				break
			}
		}

		if hit == nil {
			log.Panicf("ifd child with name [%s] not registered: [%s]", name, ptr.PathPhrase())
		}

		ptr = hit
	}

	return ptr, nil
}

// GetChild is a convenience function to get the child path for a given parent
// placement and child tag-ID.
func (im *IfdMapping) GetChild(parentPathPhrase string, tagId uint16) (mi *MappedIfd, err error) {
	defer func() {
		if state := recover(); state != nil {
			err = log.Wrap(state.(error))
		}
	}()

	mi, err = im.GetWithPath(parentPathPhrase)
	log.PanicIf(err)

	for _, childMi := range mi.Children {
		if childMi.TagId == tagId {
			return childMi, nil
		}
	}

	// Whether or not an IFD is defined in data, such an IFD is not registered
	// and would be unknown.
	log.Panic(ErrChildIfdNotMapped)
	return nil, nil
}

// IfdTagIdAndIndex represents a specific part of the IFD path.
//
// This is a legacy type.
type IfdTagIdAndIndex struct {
	Name  string
	TagId uint16
	Index int
}

// String returns a descriptive string.
func (itii IfdTagIdAndIndex) String() string {
	return fmt.Sprintf("IfdTagIdAndIndex<NAME=[%s] ID=(%04x) INDEX=(%d)>", itii.Name, itii.TagId, itii.Index)
}

// ResolvePath takes a list of names, which can also be suffixed with indices
// (to identify the second, third, etc.. sibling IFD) and returns a list of
// tag-IDs and those indices.
//
// Example:
//
// - IFD/Exif/Iop
// - IFD0/Exif/Iop
//
// This is the only call that supports adding the numeric indices.
func (im *IfdMapping) ResolvePath(pathPhrase string) (lineage []IfdTagIdAndIndex, err error) {
	defer func() {
		if state := recover(); state != nil {
			err = log.Wrap(state.(error))
		}
	}()

	pathPhrase = strings.TrimSpace(pathPhrase)

	if pathPhrase == "" {
		log.Panicf("can not resolve empty path-phrase")
	}

	path := strings.Split(pathPhrase, "/")
	lineage = make([]IfdTagIdAndIndex, len(path))

	ptr := im.rootNode
	empty := IfdTagIdAndIndex{}
	for i, name := range path {
		indexByte := name[len(name)-1]
		index := 0
		if indexByte >= '0' && indexByte <= '9' {
			index = int(indexByte - '0')
			name = name[:len(name)-1]
		}

		itii := IfdTagIdAndIndex{}
		for _, mi := range ptr.Children {
			if mi.Name != name {
				continue
			}

			itii.Name = name
			itii.TagId = mi.TagId
			itii.Index = index

			ptr = mi

			break
		}

		if itii == empty {
			log.Panicf("ifd child with name [%s] not registered: [%s]", name, pathPhrase)
		}

		lineage[i] = itii
	}

	return lineage, nil
}

// FqPathPhraseFromLineage returns the fully-qualified IFD path from the slice.
func (im *IfdMapping) FqPathPhraseFromLineage(lineage []IfdTagIdAndIndex) (fqPathPhrase string) {
	fqPathParts := make([]string, len(lineage))
	for i, itii := range lineage {
		if itii.Index > 0 {
			fqPathParts[i] = fmt.Sprintf("%s%d", itii.Name, itii.Index)
		} else {
			fqPathParts[i] = itii.Name
		}
	}

	return strings.Join(fqPathParts, "/")
}

// PathPhraseFromLineage returns the non-fully-qualified IFD path from the
// slice.
func (im *IfdMapping) PathPhraseFromLineage(lineage []IfdTagIdAndIndex) (pathPhrase string) {
	pathParts := make([]string, len(lineage))
	for i, itii := range lineage {
		pathParts[i] = itii.Name
	}

	return strings.Join(pathParts, "/")
}

// StripPathPhraseIndices returns a non-fully-qualified path-phrase (no
// indices).
func (im *IfdMapping) StripPathPhraseIndices(pathPhrase string) (strippedPathPhrase string, err error) {
	defer func() {
		if state := recover(); state != nil {
			err = log.Wrap(state.(error))
		}
	}()

	lineage, err := im.ResolvePath(pathPhrase)
	log.PanicIf(err)

	strippedPathPhrase = im.PathPhraseFromLineage(lineage)
	return strippedPathPhrase, nil
}

// Add puts the given IFD at the given position of the tree. The position of the
// tree is referred to as the placement and is represented by a set of tag-IDs,
// where the leftmost is the root tag and the tags going to the right are
// progressive descendants.
func (im *IfdMapping) Add(parentPlacement []uint16, tagId uint16, name string) (err error) {
	defer func() {
		if state := recover(); state != nil {
			err = log.Wrap(state.(error))
		}
	}()

	// TODO(dustin): !! It would be nicer to provide a list of names in the placement rather than tag-IDs.

	ptr, err := im.Get(parentPlacement)
	log.PanicIf(err)

	path := make([]string, len(parentPlacement)+1)
	if len(parentPlacement) > 0 {
		copy(path, ptr.Path)
	}

	path[len(path)-1] = name

	placement := make([]uint16, len(parentPlacement)+1)
	if len(placement) > 0 {
		copy(placement, ptr.Placement)
	}

	placement[len(placement)-1] = tagId

	childIfd := &MappedIfd{
		ParentTagId: ptr.TagId,
		Path:        path,
		Placement:   placement,
		Name:        name,
		TagId:       tagId,
		Children:    make(map[uint16]*MappedIfd),
	}

	if _, found := ptr.Children[tagId]; found == true {
		log.Panicf("child IFD with tag-ID (%04x) already registered under IFD [%s] with tag-ID (%04x)", tagId, ptr.Name, ptr.TagId)
	}

	ptr.Children[tagId] = childIfd

	return nil
}

func (im *IfdMapping) dumpLineages(stack []*MappedIfd, input []string) (output []string, err error) {
	defer func() {
		if state := recover(); state != nil {
			err = log.Wrap(state.(error))
		}
	}()

	currentIfd := stack[len(stack)-1]

	output = input
	for _, childIfd := range currentIfd.Children {
		stackCopy := make([]*MappedIfd, len(stack)+1)

		copy(stackCopy, stack)
		stackCopy[len(stack)] = childIfd

		// Add to output, but don't include the obligatory root node.
		parts := make([]string, len(stackCopy)-1)
		for i, mi := range stackCopy[1:] {
			parts[i] = mi.Name
		}

		output = append(output, strings.Join(parts, "/"))

		output, err = im.dumpLineages(stackCopy, output)
		log.PanicIf(err)
	}

	return output, nil
}

// DumpLineages returns a slice of strings representing all mappings.
func (im *IfdMapping) DumpLineages() (output []string, err error) {
	defer func() {
		if state := recover(); state != nil {
			err = log.Wrap(state.(error))
		}
	}()

	stack := []*MappedIfd{im.rootNode}
	output = make([]string, 0)

	output, err = im.dumpLineages(stack, output)
	log.PanicIf(err)

	return output, nil
}

// LoadStandardIfds loads the standard IFDs into the mapping.
func LoadStandardIfds(im *IfdMapping) (err error) {
	defer func() {
		if state := recover(); state != nil {
			err = log.Wrap(state.(error))
		}
	}()

	err = im.Add(
		[]uint16{},
		IfdStandardIfdIdentity.TagId(), IfdStandardIfdIdentity.Name())

	log.PanicIf(err)

	err = im.Add(
		[]uint16{IfdStandardIfdIdentity.TagId()},
		IfdExifStandardIfdIdentity.TagId(), IfdExifStandardIfdIdentity.Name())

	log.PanicIf(err)

	err = im.Add(
		[]uint16{IfdStandardIfdIdentity.TagId(), IfdExifStandardIfdIdentity.TagId()},
		IfdExifIopStandardIfdIdentity.TagId(), IfdExifIopStandardIfdIdentity.Name())

	log.PanicIf(err)

	err = im.Add(
		[]uint16{IfdStandardIfdIdentity.TagId()},
		IfdGpsInfoStandardIfdIdentity.TagId(), IfdGpsInfoStandardIfdIdentity.Name())

	log.PanicIf(err)

	return nil
}

// IfdTag describes a single IFD tag and its parent (if any).
type IfdTag struct {
	parentIfdTag *IfdTag
	tagId        uint16
	name         string
}

func NewIfdTag(parentIfdTag *IfdTag, tagId uint16, name string) IfdTag {
	return IfdTag{
		parentIfdTag: parentIfdTag,
		tagId:        tagId,
		name:         name,
	}
}

// ParentIfd returns the IfdTag of this IFD's parent.
func (it IfdTag) ParentIfd() *IfdTag {
	return it.parentIfdTag
}

// TagId returns the tag-ID of this IFD.
func (it IfdTag) TagId() uint16 {
	return it.tagId
}

// Name returns the simple name of this IFD.
func (it IfdTag) Name() string {
	return it.name
}

// String returns a descriptive string.
func (it IfdTag) String() string {
	parentIfdPhrase := ""
	if it.parentIfdTag != nil {
		parentIfdPhrase = fmt.Sprintf(" PARENT=(0x%04x)[%s]", it.parentIfdTag.tagId, it.parentIfdTag.name)
	}

	return fmt.Sprintf("IfdTag<TAG-ID=(0x%04x) NAME=[%s]%s>", it.tagId, it.name, parentIfdPhrase)
}

var (
	// rootStandardIfd is the standard root IFD.
	rootStandardIfd = NewIfdTag(nil, 0x0000, "IFD") // IFD

	// exifStandardIfd is the standard "Exif" IFD.
	exifStandardIfd = NewIfdTag(&rootStandardIfd, 0x8769, "Exif") // IFD/Exif

	// iopStandardIfd is the standard "Iop" IFD.
	iopStandardIfd = NewIfdTag(&exifStandardIfd, 0xA005, "Iop") // IFD/Exif/Iop

	// gpsInfoStandardIfd is the standard "GPS" IFD.
	gpsInfoStandardIfd = NewIfdTag(&rootStandardIfd, 0x8825, "GPSInfo") // IFD/GPSInfo
)

// IfdIdentityPart represents one component in an IFD path.
type IfdIdentityPart struct {
	Name  string
	Index int
}

// String returns a fully-qualified IFD path.
func (iip IfdIdentityPart) String() string {
	if iip.Index > 0 {
		return fmt.Sprintf("%s%d", iip.Name, iip.Index)
	} else {
		return iip.Name
	}
}

// UnindexedString returned a non-fully-qualified IFD path.
func (iip IfdIdentityPart) UnindexedString() string {
	return iip.Name
}

// IfdIdentity represents a single IFD path and provides access to various
// information and representations.
//
// Only global instances can be used for equality checks.
type IfdIdentity struct {
	ifdTag    IfdTag
	parts     []IfdIdentityPart
	ifdPath   string
	fqIfdPath string
}

// NewIfdIdentity returns a new IfdIdentity struct.
func NewIfdIdentity(ifdTag IfdTag, parts ...IfdIdentityPart) (ii *IfdIdentity) {
	ii = &IfdIdentity{
		ifdTag: ifdTag,
		parts:  parts,
	}

	ii.ifdPath = ii.getIfdPath()
	ii.fqIfdPath = ii.getFqIfdPath()

	return ii
}

// NewIfdIdentityFromString parses a string like "IFD/Exif" or "IFD1" or
// something more exotic with custom IFDs ("SomeIFD4/SomeChildIFD6"). Note that
// this will valid the unindexed IFD structure (because the standard tags from
// the specification are unindexed), but not, obviously, any indices (e.g.
// the numbers in "IFD0", "IFD1", "SomeIFD4/SomeChildIFD6"). It is
// required for the caller to check whether these specific instances
// were actually parsed out of the stream.
func NewIfdIdentityFromString(im *IfdMapping, fqIfdPath string) (ii *IfdIdentity, err error) {
	defer func() {
		if state := recover(); state != nil {
			err = log.Wrap(state.(error))
		}
	}()

	lineage, err := im.ResolvePath(fqIfdPath)
	log.PanicIf(err)

	var lastIt *IfdTag
	identityParts := make([]IfdIdentityPart, len(lineage))
	for i, itii := range lineage {
		// Build out the tag that will eventually point to the IFD represented
		// by the right-most part in the IFD path.

		it := &IfdTag{
			parentIfdTag: lastIt,
			tagId:        itii.TagId,
			name:         itii.Name,
		}

		lastIt = it

		// Create the next IfdIdentity part.

		iip := IfdIdentityPart{
			Name:  itii.Name,
			Index: itii.Index,
		}

		identityParts[i] = iip
	}

	ii = NewIfdIdentity(*lastIt, identityParts...)
	return ii, nil
}

func (ii *IfdIdentity) getFqIfdPath() string {
	partPhrases := make([]string, len(ii.parts))
	for i, iip := range ii.parts {
		partPhrases[i] = iip.String()
	}

	return strings.Join(partPhrases, "/")
}

func (ii *IfdIdentity) getIfdPath() string {
	partPhrases := make([]string, len(ii.parts))
	for i, iip := range ii.parts {
		partPhrases[i] = iip.UnindexedString()
	}

	return strings.Join(partPhrases, "/")
}

// String returns a fully-qualified IFD path.
func (ii *IfdIdentity) String() string {
	return ii.fqIfdPath
}

// UnindexedString returns a non-fully-qualified IFD path.
func (ii *IfdIdentity) UnindexedString() string {
	return ii.ifdPath
}

// IfdTag returns the tag struct behind this IFD.
func (ii *IfdIdentity) IfdTag() IfdTag {
	return ii.ifdTag
}

// TagId returns the tag-ID of the IFD.
func (ii *IfdIdentity) TagId() uint16 {
	return ii.ifdTag.TagId()
}

// LeafPathPart returns the last right-most path-part, which represents the
// current IFD.
func (ii *IfdIdentity) LeafPathPart() IfdIdentityPart {
	return ii.parts[len(ii.parts)-1]
}

// Name returns the simple name of this IFD.
func (ii *IfdIdentity) Name() string {
	return ii.LeafPathPart().Name
}

// Index returns the index of this IFD (more then one IFD under a parent IFD
// will be numbered [0..n]).
func (ii *IfdIdentity) Index() int {
	return ii.LeafPathPart().Index
}

// Equals returns true if the two IfdIdentity instances are effectively
// identical.
//
// Since there's no way to get a specific fully-qualified IFD path without a
// certain slice of parts and all other fields are also derived from this,
// checking that the fully-qualified IFD path is equals is sufficient.
func (ii *IfdIdentity) Equals(ii2 *IfdIdentity) bool {
	return ii.String() == ii2.String()
}

// NewChild creates an IfdIdentity for an IFD that is a child of the current
// IFD.
func (ii *IfdIdentity) NewChild(childIfdTag IfdTag, index int) (iiChild *IfdIdentity) {
	if *childIfdTag.parentIfdTag != ii.ifdTag {
		log.Panicf("can not add child; we are not the parent:\nUS=%v\nCHILD=%v", ii.ifdTag, childIfdTag)
	}

	childPart := IfdIdentityPart{childIfdTag.name, index}
	childParts := append(ii.parts, childPart)

	iiChild = NewIfdIdentity(childIfdTag, childParts...)
	return iiChild
}

// NewSibling creates an IfdIdentity for an IFD that is a sibling to the current
// one.
func (ii *IfdIdentity) NewSibling(index int) (iiSibling *IfdIdentity) {
	parts := make([]IfdIdentityPart, len(ii.parts))

	copy(parts, ii.parts)
	parts[len(parts)-1].Index = index

	iiSibling = NewIfdIdentity(ii.ifdTag, parts...)
	return iiSibling
}

var (
	// IfdStandardIfdIdentity represents the IFD path for IFD0.
	IfdStandardIfdIdentity = NewIfdIdentity(rootStandardIfd, IfdIdentityPart{"IFD", 0})

	// IfdExifStandardIfdIdentity represents the IFD path for IFD0/Exif0.
	IfdExifStandardIfdIdentity = IfdStandardIfdIdentity.NewChild(exifStandardIfd, 0)

	// IfdExifIopStandardIfdIdentity represents the IFD path for IFD0/Exif0/Iop0.
	IfdExifIopStandardIfdIdentity = IfdExifStandardIfdIdentity.NewChild(iopStandardIfd, 0)

	// IfdGPSInfoStandardIfdIdentity represents the IFD path for IFD0/GPSInfo0.
	IfdGpsInfoStandardIfdIdentity = IfdStandardIfdIdentity.NewChild(gpsInfoStandardIfd, 0)

	// Ifd1StandardIfdIdentity represents the IFD path for IFD1.
	Ifd1StandardIfdIdentity = NewIfdIdentity(rootStandardIfd, IfdIdentityPart{"IFD", 1})
)
