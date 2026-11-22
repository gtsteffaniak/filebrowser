// Package opensw reads SolidWorks documents the way OpenSW (the sldprt
// Python library) does, ported to pure Go so the same code runs on the
// server and compiled to WebAssembly in the browser.
//
// A document is one of two containers: the classic OLE2 compound file
// (CFB) or the newer "block stream" format whose named blobs are
// raw-DEFLATE blocks found by a marker.
package opensw

import (
	"bytes"
	"compress/flate"
	"encoding/binary"
	"errors"
	"fmt"
	"hash/crc32"
	"io"
	"unicode/utf16"
)

// ole2Magic starts every classic compound document; blockMarker starts each
// block of the newer container.
var ole2Magic = []byte{0xD0, 0xCF, 0x11, 0xE0, 0xA1, 0xB1, 0x1A, 0xE1}
var blockMarker = []byte{0x14, 0x00, 0x06, 0x00, 0x08, 0x00}

// cfbFreeSector and friends are the special sector marks of a compound file.
const (
	cfbFreeSector  = 0xFFFFFFFF
	cfbEndOfChain  = 0xFFFFFFFE
	cfbDIFATSector = 0xFFFFFFFC
)

type cfbEntry struct {
	name      string
	entryType byte // 0 unallocated, 1 storage, 2 stream, 5 root
	left      uint32
	right     uint32
	child     uint32
	start     uint32
	size      int64
}

type cfbFile struct {
	data       []byte
	sectorSize int
	miniSize   int
	fat        []uint32
	miniFat    []uint32
	miniStream []byte
	entries    []cfbEntry
	paths      map[string]int // full path -> entry id
}

// Container is a SolidWorks document opened for reading, whatever its
// physical format: named streams, as OpenSW's Container exposes blobs.
type Container struct {
	// Streams maps every stream name (e.g. "PreviewPNG",
	// "FaceTessellations/Directory") to its payload.
	Streams map[string][]byte
}

// OpenContainer detects the container kind and reads every named stream.
func OpenContainer(data []byte) (*Container, error) {
	switch {
	case len(data) >= len(ole2Magic) && bytes.Equal(data[:8], ole2Magic):
		f, err := readCFB(data)
		if err != nil {
			return nil, err
		}
		out := map[string][]byte{}
		for path, id := range f.paths {
			if b := f.stream(path, id); b != nil {
				out[path] = b
			}
		}
		return &Container{Streams: out}, nil
	case blockMarkerIn(data):
		blobs, err := readBlockBlobs(data)
		if err != nil {
			return nil, err
		}
		return &Container{Streams: blobs}, nil
	default:
		return nil, errors.New("opensw: unknown SolidWorks container")
	}
}

// Get returns the stream payload, or nil when absent.
func (c *Container) Get(name string) []byte {
	return c.Streams[name]
}

func blockMarkerIn(data []byte) bool {
	head := data
	if len(head) > 4096 {
		head = head[:4096]
	}
	return bytes.Contains(head, blockMarker)
}

// readCFB parses the structure of a Compound File Binary (OLE2) document.
// Only what stream reading needs is parsed: FAT, mini FAT, the directory
// tree and the root's mini stream.
func readCFB(data []byte) (*cfbFile, error) {
	if len(data) < 512 {
		return nil, errors.New("opensw: file too small to be a compound document")
	}
	if !bytes.Equal(data[:8], ole2Magic) {
		return nil, errors.New("opensw: not a compound document")
	}
	sectorShift := binary.LittleEndian.Uint16(data[30:32])
	miniShift := binary.LittleEndian.Uint16(data[32:34])
	if sectorShift < 6 || sectorShift > 20 || miniShift != 6 {
		return nil, errors.New("opensw: unexpected sector size")
	}
	f := &cfbFile{
		data:       data,
		sectorSize: 1 << sectorShift,
		miniSize:   1 << miniShift,
		paths:      map[string]int{},
	}
	firstDirSector := binary.LittleEndian.Uint32(data[48:52])
	firstMiniFAT := binary.LittleEndian.Uint32(data[60:64])
	numMiniFAT := binary.LittleEndian.Uint32(data[64:68])
	firstDIFAT := binary.LittleEndian.Uint32(data[68:72])
	numDIFAT := binary.LittleEndian.Uint32(data[72:76])

	// The header occupies one whole sector; sector 0 starts after it. Bound
	// table counts by the sectors the file can actually hold so a crafted
	// header cannot trigger an oversized allocation.
	headerSpan := int64(f.sectorSize)
	maxSectors := int64(0)
	if int64(len(data)) > headerSpan {
		maxSectors = (int64(len(data)) - headerSpan + headerSpan - 1) / headerSpan
	}
	numFatSectors := binary.LittleEndian.Uint32(data[44:48])
	if int64(numFatSectors) > maxSectors {
		return nil, errors.New("opensw: FAT sector count beyond file size")
	}
	if int64(numDIFAT) > maxSectors || int64(numMiniFAT) > maxSectors {
		return nil, errors.New("opensw: DIFAT/miniFAT sector count beyond file size")
	}

	// DIFAT: 109 entries in the header, then a chain of DIFAT sectors.
	difat := make([]uint32, 0, 109+int(numDIFAT)*f.sectorSize/4)
	for i := 0; i < 109; i++ {
		difat = append(difat, binary.LittleEndian.Uint32(data[76+4*i:80+4*i]))
	}
	difat = append(difat, difatEntries(f, firstDIFAT, int(numDIFAT))...)

	// FAT: only ordinary, unique, in-range sectors may back it. Validate them
	// before allocating so a crafted DIFAT cannot inflate the capacity.
	seen := make(map[uint32]bool, len(difat))
	validDIFAT := make([]uint32, 0, len(difat))
	for _, sid := range difat {
		if sid == cfbFreeSector {
			continue
		}
		if sid == cfbEndOfChain || sid == cfbDIFATSector || int64(sid) >= maxSectors || seen[sid] {
			return nil, fmt.Errorf("opensw: invalid FAT sector id %d", sid)
		}
		seen[sid] = true
		validDIFAT = append(validDIFAT, sid)
	}
	difat = validDIFAT
	fat := make([]uint32, 0, int(numFatSectors)*f.sectorSize/4)
	for _, sid := range difat {
		sect, err := f.sector(sid)
		if err != nil {
			return nil, err
		}
		for off := 0; off+4 <= len(sect); off += 4 {
			fat = append(fat, binary.LittleEndian.Uint32(sect[off:off+4]))
		}
	}
	if len(fat) == 0 {
		return nil, errors.New("opensw: empty FAT")
	}
	f.fat = fat

	// Directory entries, chained from the first directory sector.
	dirBytes, err := f.readChain(fat, firstDirSector, 1<<62)
	if err != nil {
		return nil, err
	}
	for off := 0; off+128 <= len(dirBytes); off += 128 {
		entry, parseErr := parseEntry(dirBytes[off : off+128])
		if parseErr != nil {
			return nil, parseErr
		}
		f.entries = append(f.entries, entry)
	}
	if len(f.entries) == 0 || f.entries[0].entryType != 5 {
		return nil, errors.New("opensw: missing root directory entry")
	}

	// Mini FAT and the root's mini stream (backing store of small streams).
	f.miniFat, err = f.readChainSlice(fat, firstMiniFAT, int64(numMiniFAT)*int64(f.sectorSize))
	if err != nil {
		return nil, err
	}
	f.miniStream, err = f.readChain(fat, f.entries[0].start, f.entries[0].size)
	if err != nil {
		return nil, err
	}

	// Full paths, walking storage trees from the root's children. The root
	// entry itself has no name in stream paths ("PreviewPNG", not
	// "Root Entry/PreviewPNG").
	if f.entries[0].child != cfbFreeSector {
		if err := f.walk(int(f.entries[0].child), "", map[int]bool{0: true}); err != nil {
			return nil, err
		}
	}
	return f, nil
}

func difatEntries(f *cfbFile, first uint32, count int) []uint32 {
	var out []uint32
	sid := first
	perSector := f.sectorSize/4 - 1
	for i := 0; i < count && sid != cfbFreeSector && sid != cfbEndOfChain; i++ {
		sect, err := f.sector(sid)
		if err != nil {
			return out
		}
		for j := 0; j < perSector; j++ {
			out = append(out, binary.LittleEndian.Uint32(sect[4*j:4*j+4]))
		}
		sid = binary.LittleEndian.Uint32(sect[4*perSector : 4*perSector+4])
	}
	return out
}

func (f *cfbFile) sector(sid uint32) ([]byte, error) {
	off := (int64(sid) + 1) * int64(f.sectorSize)
	if off+int64(f.sectorSize) > int64(len(f.data)) {
		return nil, fmt.Errorf("opensw: sector %d beyond end of file", sid)
	}
	return f.data[off : off+int64(f.sectorSize)], nil
}

// readChain follows fat from start and returns at most limit bytes of
// chained sector payloads. A valid chain never repeats a sector, so more
// steps than FAT entries means the chain loops.
func (f *cfbFile) readChain(fat []uint32, start uint32, limit int64) ([]byte, error) {
	var out []byte
	for sid, steps := start, 0; sid != cfbEndOfChain && sid != cfbFreeSector; steps++ {
		if int(sid) >= len(fat) {
			return nil, errors.New("opensw: FAT chain out of range")
		}
		if steps > len(fat) {
			return nil, errors.New("opensw: FAT chain loops")
		}
		sect, err := f.sector(sid)
		if err != nil {
			return nil, err
		}
		out = append(out, sect...)
		if int64(len(out)) >= limit {
			break
		}
		sid = fat[sid]
	}
	return out, nil
}

// readChainSlice is readChain returning the raw link words (used for mini FAT).
func (f *cfbFile) readChainSlice(fat []uint32, start uint32, limit int64) ([]uint32, error) {
	raw, err := f.readChain(fat, start, limit)
	if err != nil {
		return nil, err
	}
	out := make([]uint32, 0, len(raw)/4)
	for off := 0; off+4 <= len(raw); off += 4 {
		out = append(out, binary.LittleEndian.Uint32(raw[off : off+4]))
	}
	return out, nil
}

func parseEntry(e []byte) (cfbEntry, error) {
	nameLen := int(binary.LittleEndian.Uint16(e[64:66]))
	// The name field is 64 bytes; nameLen counts UTF-16 units including the
	// terminator, so it must fit the field and be even.
	if nameLen > 64 || nameLen%2 != 0 {
		return cfbEntry{}, fmt.Errorf("opensw: bad directory name length %d", nameLen)
	}
	name := ""
	if nameLen >= 2 {
		raw := e[:nameLen-2] // trailing NUL excluded
		u16 := make([]uint16, len(raw)/2)
		for i := range u16 {
			u16[i] = binary.LittleEndian.Uint16(raw[2*i:])
		}
		name = string(utf16.Decode(u16))
	}
	size := binary.LittleEndian.Uint64(e[120:128])
	if size > 1<<63-1 {
		return cfbEntry{}, fmt.Errorf("opensw: directory stream size %d out of range", size)
	}
	return cfbEntry{
		name:      name,
		entryType: e[66],
		left:      binary.LittleEndian.Uint32(e[68:72]),
		right:     binary.LittleEndian.Uint32(e[72:76]),
		child:     binary.LittleEndian.Uint32(e[76:80]),
		start:     binary.LittleEndian.Uint32(e[116:120]),
		size:      int64(size),
	}, nil
}

// walk records full storage paths, visiting siblings and children. visited
// holds every directory ID seen so far; a revisit means a malformed (cyclic
// or shared) tree, which would otherwise recurse forever or blow up.
func (f *cfbFile) walk(id int, prefix string, visited map[int]bool) error {
	if id < 0 || id >= len(f.entries) {
		return nil
	}
	if visited[id] {
		return fmt.Errorf("opensw: cyclic directory entry %d", id)
	}
	visited[id] = true
	e := f.entries[id]
	if e.entryType == 0 {
		return nil
	}
	path := e.name
	if prefix != "" {
		path = prefix + "/" + e.name
	}
	if e.entryType == 2 { // streams are the leaves reading needs
		if _, dup := f.paths[path]; !dup {
			f.paths[path] = id
		}
	}
	if e.child != cfbFreeSector {
		if err := f.walk(int(e.child), path, visited); err != nil {
			return err
		}
	}
	if err := f.walk(int(e.left), prefix, visited); err != nil {
		return err
	}
	return f.walk(int(e.right), prefix, visited)
}

// stream returns the payload of the stream entry, or nil.
func (f *cfbFile) stream(path string, id int) []byte {
	e := f.entries[id]
	if e.size >= 4096 {
		b, err := f.readChain(f.fat, e.start, e.size)
		if err != nil {
			return nil
		}
		return b[:min64(int64(len(b)), e.size)]
	}
	// Small streams live in the root's mini stream, chained through mini FAT.
	var out []byte
	sid := e.start
	for sid != cfbEndOfChain && sid != cfbFreeSector && int64(len(out)) < e.size {
		if int(sid) >= len(f.miniFat) {
			return nil
		}
		off := int64(sid) * int64(f.miniSize)
		if off+int64(f.miniSize) > int64(len(f.miniStream)) {
			return nil
		}
		out = append(out, f.miniStream[off:off+int64(f.miniSize)]...)
		sid = f.miniFat[sid]
	}
	if int64(len(out)) < e.size {
		return nil
	}
	return out[:e.size]
}

func min64(a, b int64) int64 {
	if a < b {
		return a
	}
	return b
}

// --- block stream container -------------------------------------------------

const blockHeaderSize = 26

// nibbleSwap rotates every byte left by key bits (key 4 swaps nibbles): the
// block container's name obfuscation.
func nibbleSwap(data []byte, key int) []byte {
	shift := key & 7
	if shift == 0 {
		return data
	}
	out := make([]byte, len(data))
	for i, b := range data {
		out[i] = (b << shift) | (b >> (8 - shift))
	}
	return out
}

// nameKey reads the rotation key from the block container's header.
func nameKey(data []byte) int {
	if len(data) > 7 && data[4] == 0 && data[5] == 0 && data[6] == 0 && data[7] != 0 {
		return int(data[7])
	}
	return 4
}

func printableName(name string) bool {
	if name == "" {
		return false
	}
	for _, c := range name {
		if c < 0x20 || c >= 0x7F {
			return false
		}
	}
	return true
}

const (
	maxBlockInflated = 64 << 20  // per-stream inflated limit
	maxDecodedPerDoc = 256 << 20 // all retained decoded streams in one document
)

type decodeBudget struct {
	remaining int64
	exceeded  bool
}

// tryBlock decodes the block at off: marker, type id, crc32, compressed and
// uncompressed sizes, name length, then the rotated name and the DEFLATE
// payload. ok is false unless every check passes.
func tryBlock(data []byte, off, key int, b *decodeBudget) (name string, raw []byte, end int, ok bool) {
	if off+blockHeaderSize > len(data) {
		return "", nil, 0, false
	}
	crc := binary.LittleEndian.Uint32(data[off+10 : off+14])
	compSz := binary.LittleEndian.Uint32(data[off+14 : off+18])
	uncompSz := binary.LittleEndian.Uint32(data[off+18 : off+22])
	preSz := binary.LittleEndian.Uint32(data[off+22 : off+26])
	if preSz > 4096 || uint64(compSz) > uint64(len(data)) || uncompSz > maxBlockInflated {
		return "", nil, 0, false
	}
	// Check before inflating: a stream may use only the remaining document budget.
	if int64(uncompSz)+1 > b.remaining {
		b.exceeded = true
		return "", nil, 0, false
	}
	body := off + blockHeaderSize
	end = body + int(preSz) + int(compSz)
	if end > len(data) {
		return "", nil, 0, false
	}
	payload := data[body+int(preSz) : end]
	fr := flate.NewReader(bytes.NewReader(payload))
	defer fr.Close()
	// One byte past uncompSz: a stream that inflates larger must be rejected.
	limit := int64(uncompSz) + 1
	if limit > b.remaining {
		limit = b.remaining
	}
	raw, err := io.ReadAll(io.LimitReader(fr, limit))
	if err != nil {
		return "", nil, 0, false
	}
	if uint32(len(raw)) != uncompSz || crc32.ChecksumIEEE(raw) != crc {
		return "", nil, 0, false
	}
	name = string(nibbleSwap(data[body:body+int(preSz)], key))
	if !printableName(name) {
		return "", nil, 0, false
	}
	return name, raw, end, true
}

// readBlockBlobs scans for marker-prefixed blocks and returns the first
// payload per name, bounded by a document-wide decoded budget.
func readBlockBlobs(data []byte) (map[string][]byte, error) {
	key := nameKey(data)
	out := map[string][]byte{}
	b := &decodeBudget{remaining: maxDecodedPerDoc}
	searchFrom := 0
	for {
		rel := bytes.Index(data[searchFrom:], blockMarker)
		if rel < 0 {
			break
		}
		off := searchFrom + rel
		name, raw, end, ok := tryBlock(data, off, key, b)
		if b.exceeded {
			return nil, errors.New("opensw: decoded block budget exceeded")
		}
		if ok {
			if _, exists := out[name]; !exists {
				out[name] = raw
				b.remaining -= int64(len(raw))
			}
			searchFrom = end
		} else {
			searchFrom = off + len(blockMarker)
		}
	}
	return out, nil
}
