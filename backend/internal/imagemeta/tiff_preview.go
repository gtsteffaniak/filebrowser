//go:build !386 && !arm

package imagemeta

import (
	"encoding/binary"
	"io"
	"os"
)

const (
	tiffTagSubIFDs            = 0x014a
	tiffTagJpgFromRaw         = 0x002e
	tiffTagJPEGInterchange    = 0x0201
	tiffTagJPEGInterchangeLen = 0x0202
	tiffIFDScanLimit          = 4 * 1024 * 1024
	tiffTypeByte              = 1
	tiffTypeASCII             = 2
	tiffTypeShort             = 3
	tiffTypeLong              = 4
	tiffTypeIFD               = 13
	tiffTypeUndefined         = 7
	maxTIFFIFDsVisited        = 128
	maxTIFFIFDQueueLen        = 128
	maxTIFFSubIFDsPerEntry    = 16
	maxTIFFPreviewCandidates  = 32
)

type tiffIFD struct {
	tags map[uint16]tiffTag
}

type tiffTag struct {
	typ         uint16
	count       uint32
	valueOffset uint32
}

// collectTIFFPreviewCandidates walks TIFF IFDs (including SubIFDs) for embedded
// JPEG preview offset/length pairs that imagemeta does not surface.
func collectTIFFPreviewCandidates(f *os.File) ([]previewCandidate, error) {
	info, err := f.Stat()
	if err != nil {
		return nil, err
	}
	scanSize := info.Size()
	if scanSize > tiffIFDScanLimit {
		scanSize = tiffIFDScanLimit
	}
	if scanSize < 8 {
		return nil, nil
	}
	header := make([]byte, scanSize)
	n, err := f.ReadAt(header, 0)
	if err != nil && err != io.EOF {
		return nil, err
	}
	header = header[:n]

	order, ifd0, ok := tiffHeaderIFD0(header)
	if !ok {
		return nil, nil
	}

	seen := make(map[uint32]struct{})
	var ifds []tiffIFD
	var queue []uint32
	queue = append(queue, ifd0)
	for len(queue) > 0 && len(seen) < maxTIFFIFDsVisited {
		off := queue[0]
		queue = queue[1:]
		if off == 0 {
			continue
		}
		if _, done := seen[off]; done {
			continue
		}
		seen[off] = struct{}{}

		ifd, next, subIFDs, err := parseTIFFIFD(header, order, off)
		if err != nil {
			continue
		}
		ifds = append(ifds, ifd)
		if next != 0 && len(queue) < maxTIFFIFDQueueLen {
			queue = append(queue, next)
		}
		for _, sub := range subIFDs {
			if len(queue) >= maxTIFFIFDQueueLen {
				break
			}
			queue = append(queue, sub)
		}
	}

	var out []previewCandidate
	for _, ifd := range ifds {
		out = appendPreviewCandidates(out, jpgFromRawCandidates(ifd))
		out = appendPreviewCandidates(out, jpegInterchangeCandidates(ifd))
		if len(out) >= maxTIFFPreviewCandidates {
			break
		}
	}
	return out, nil
}

func appendPreviewCandidates(dst, src []previewCandidate) []previewCandidate {
	remaining := maxTIFFPreviewCandidates - len(dst)
	if remaining <= 0 {
		return dst
	}
	if len(src) > remaining {
		src = src[:remaining]
	}
	return append(dst, src...)
}

func jpgFromRawCandidates(ifd tiffIFD) []previewCandidate {
	tag, ok := ifd.tags[tiffTagJpgFromRaw]
	if !ok || tag.count == 0 {
		return nil
	}
	byteLen := uint32(int(tag.count) * tiffTagByteSize(tag.typ))
	if byteLen <= 4 {
		return nil
	}
	return []previewCandidate{{offset: tag.valueOffset, length: byteLen}}
}

func jpegInterchangeCandidates(ifd tiffIFD) []previewCandidate {
	offTag, okOff := ifd.tags[tiffTagJPEGInterchange]
	lenTag, okLen := ifd.tags[tiffTagJPEGInterchangeLen]
	if !okOff || !okLen || offTag.count == 0 || lenTag.count == 0 {
		return nil
	}
	if offTag.typ != tiffTypeLong || lenTag.typ != tiffTypeLong {
		return nil
	}
	offset := offTag.valueOffset
	length := lenTag.valueOffset
	if length == 0 || length > maxPreviewReadSize {
		return nil
	}
	return []previewCandidate{{offset: offset, length: length}}
}

func tiffHeaderIFD0(data []byte) (binary.ByteOrder, uint32, bool) {
	if len(data) < 8 {
		return nil, 0, false
	}
	var order binary.ByteOrder
	switch string(data[:2]) {
	case "II":
		order = binary.LittleEndian
	case "MM":
		order = binary.BigEndian
	default:
		return nil, 0, false
	}
	if order.Uint16(data[2:4]) != 42 {
		return nil, 0, false
	}
	return order, order.Uint32(data[4:8]), true
}

func parseTIFFIFD(data []byte, order binary.ByteOrder, offset uint32) (ifd tiffIFD, next uint32, subIFDs []uint32, err error) {
	ifd.tags = make(map[uint16]tiffTag)
	off := int(offset)
	if off < 0 || off+2 > len(data) {
		return ifd, 0, nil, io.ErrUnexpectedEOF
	}
	count := int(order.Uint16(data[off : off+2]))
	entryBase := off + 2
	for i := 0; i < count; i++ {
		entry := entryBase + i*12
		if entry+12 > len(data) {
			return ifd, 0, nil, io.ErrUnexpectedEOF
		}
		tagID := order.Uint16(data[entry : entry+2])
		typ := order.Uint16(data[entry+2 : entry+4])
		cnt := order.Uint32(data[entry+4 : entry+8])
		val := order.Uint32(data[entry+8 : entry+12])
		ifd.tags[tagID] = tiffTag{typ: typ, count: cnt, valueOffset: val}
		if tagID == tiffTagSubIFDs {
			subIFDs = limitSubIFDs(tiffTagUint32s(data, order, typ, cnt, val))
		}
	}
	nextOff := entryBase + count*12
	if nextOff+4 <= len(data) {
		next = order.Uint32(data[nextOff : nextOff+4])
	}
	return ifd, next, subIFDs, nil
}

func limitSubIFDs(offsets []uint32) []uint32 {
	if len(offsets) > maxTIFFSubIFDsPerEntry {
		return offsets[:maxTIFFSubIFDsPerEntry]
	}
	return offsets
}

func tiffTagUint32s(data []byte, order binary.ByteOrder, typ uint16, count uint32, valueOffset uint32) []uint32 {
	if count == 0 {
		return nil
	}
	if count > maxTIFFSubIFDsPerEntry {
		count = maxTIFFSubIFDsPerEntry
	}
	elemSize := tiffTagByteSize(typ)
	if elemSize <= 0 {
		return nil
	}
	var raw []byte
	switch {
	case typ == tiffTypeLong && count == 1:
		var b [4]byte
		order.PutUint32(b[:], valueOffset)
		raw = b[:]
	case elemSize*int(count) <= 4:
		var b [4]byte
		order.PutUint32(b[:], valueOffset)
		raw = b[:count*uint32(elemSize)]
	default:
		off := int(valueOffset)
		n := int(count) * elemSize
		if off < 0 || off+n > len(data) {
			return nil
		}
		raw = data[off : off+n]
	}
	out := make([]uint32, 0, count)
	for i := 0; i < int(count); i++ {
		pos := i * elemSize
		if pos+elemSize > len(raw) {
			break
		}
		switch typ {
		case tiffTypeShort:
			out = append(out, uint32(order.Uint16(raw[pos:pos+2])))
		case tiffTypeLong, tiffTypeIFD:
			out = append(out, order.Uint32(raw[pos:pos+4]))
		default:
			return nil
		}
	}
	return out
}

func tiffTagByteSize(typ uint16) int {
	switch typ {
	case tiffTypeByte, tiffTypeASCII, tiffTypeUndefined:
		return 1
	case tiffTypeShort:
		return 2
	case tiffTypeLong, tiffTypeIFD:
		return 4
	default:
		return 0
	}
}
