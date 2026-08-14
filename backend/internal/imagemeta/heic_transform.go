//go:build !386 && !arm

package imagemeta

import (
	"encoding/binary"
	"io"
	"os"
)

const heicMetadataScanLimit = 32 * 1024 * 1024

type heicTransform struct {
	irot  uint8
	found bool
}

// parseHEICTransform reads the irot property associated with the primary image item.
// Per MIAF, irot takes precedence over informational EXIF Orientation (same as
// ExifTool QuickTime:Rotation, which the previous exiftool integration used).
func parseHEICTransform(f *os.File) heicTransform {
	info, err := f.Stat()
	if err != nil {
		return heicTransform{}
	}
	size := info.Size()
	if size > heicMetadataScanLimit {
		size = heicMetadataScanLimit
	}
	if size < 12 {
		return heicTransform{}
	}
	buf := make([]byte, size)
	n, err := f.ReadAt(buf, 0)
	if err != nil && err != io.EOF {
		return heicTransform{}
	}
	return parseHEICPrimaryIrot(buf[:n])
}

func parseHEICPrimaryIrot(data []byte) heicTransform {
	meta := findBMFFBox(data, "meta")
	if meta == nil {
		return heicTransform{}
	}

	primaryID := parsePITMItemID(findBMFFChildBox(meta, "pitm"))
	if primaryID == 0 {
		return heicTransform{}
	}

	iprp := findBMFFChildBox(meta, "iprp")
	if iprp == nil {
		return heicTransform{}
	}

	ipco := findBMFFChildBox(iprp, "ipco")
	ipma := findBMFFChildBox(iprp, "ipma")
	if ipco == nil || ipma == nil {
		return heicTransform{}
	}

	properties := parseIPCOProperties(ipco)
	indices := parseIPMAPropertyIndices(ipma, primaryID)
	for _, idx := range indices {
		if idx == 0 || int(idx) > len(properties) {
			continue
		}
		prop := properties[idx-1]
		if prop.typ == "irot" {
			return heicTransform{irot: prop.irot & 0x03, found: true}
		}
	}
	return heicTransform{}
}

func findBMFFBox(data []byte, typ string) []byte {
	return findBMFFBoxRange(data, 0, len(data), typ)
}

func findBMFFBoxRange(data []byte, start, end int, typ string) []byte {
	pos := start
	for pos+8 <= end {
		size, header, ok := bmffBoxHeader(data, pos, end)
		if !ok || size < header {
			break
		}
		boxEnd := pos + size
		if boxEnd > end {
			break
		}
		if string(data[pos+4:pos+8]) == typ {
			return data[pos:boxEnd]
		}
		if isBMFFContainerType(data[pos+4 : pos+8]) {
			if child := findBMFFBoxRange(data, pos+header, boxEnd, typ); child != nil {
				return child
			}
		}
		pos = boxEnd
	}
	return nil
}

func findBMFFChildBox(container []byte, typ string) []byte {
	payload := bmffBoxPayload(container)
	if payload == nil {
		return nil
	}
	return findBMFFBoxRange(payload, 0, len(payload), typ)
}

func isBMFFContainerType(typ []byte) bool {
	switch string(typ) {
	case "meta", "iprp", "ipco", "moov", "trak", "mdia", "minf", "stbl", "dinf":
		return true
	default:
		return false
	}
}

func bmffBoxHeader(data []byte, pos, end int) (size int, header int, ok bool) {
	if pos+8 > end {
		return 0, 0, false
	}
	size32 := binary.BigEndian.Uint32(data[pos : pos+4])
	if size32 == 0 {
		return 0, 0, false
	}
	if size32 == 1 {
		if pos+16 > end {
			return 0, 0, false
		}
		return int(binary.BigEndian.Uint64(data[pos+8 : pos+16])), 16, true
	}
	return int(size32), 8, true
}

func bmffBoxPayload(box []byte) []byte {
	if len(box) < 8 {
		return nil
	}
	typ := string(box[4:8])
	header := 8
	if binary.BigEndian.Uint32(box[0:4]) == 1 {
		header = 16
	}
	if isBMFFFullBoxType(typ) {
		header += 4
	}
	if header > len(box) {
		return nil
	}
	return box[header:]
}

func isBMFFFullBoxType(typ string) bool {
	switch typ {
	case "meta", "pitm", "irot", "imir", "ispe", "ipma", "hdlr", "iloc", "iinf", "infe":
		return true
	default:
		return false
	}
}

func parsePITMItemID(pitm []byte) uint32 {
	payload := bmffBoxPayload(pitm)
	if len(pitm) < 12 || payload == nil {
		return 0
	}
	version := pitm[8]
	switch version {
	case 0:
		if len(payload) < 2 {
			return 0
		}
		return uint32(binary.BigEndian.Uint16(payload[0:2]))
	default:
		if len(payload) < 4 {
			return 0
		}
		return binary.BigEndian.Uint32(payload[0:4])
	}
}

type ipcoProperty struct {
	typ  string
	irot uint8
}

func parseIPCOProperties(ipco []byte) []ipcoProperty {
	payload := bmffBoxPayload(ipco)
	if payload == nil {
		return nil
	}
	var props []ipcoProperty
	pos := 0
	for pos+8 <= len(payload) {
		size, header, ok := bmffBoxHeader(payload, pos, len(payload))
		if !ok || size < header || pos+size > len(payload) {
			break
		}
		child := payload[pos : pos+size]
		prop := ipcoProperty{typ: string(child[4:8])}
		if prop.typ == "irot" && len(child) >= 13 {
			prop.irot = child[12] & 0x03
		}
		props = append(props, prop)
		pos += size
	}
	return props
}

func parseIPMAPropertyIndices(ipma []byte, itemID uint32) []uint8 {
	payload := bmffBoxPayload(ipma)
	if len(ipma) < 12 || payload == nil || len(payload) < 4 {
		return nil
	}
	version := ipma[8]
	entryCount := int(binary.BigEndian.Uint32(payload[0:4]))
	pos := 4
	for i := 0; i < entryCount && pos < len(payload); i++ {
		var id uint32
		switch version {
		case 0:
			if pos+2 > len(payload) {
				return nil
			}
			id = uint32(binary.BigEndian.Uint16(payload[pos : pos+2]))
			pos += 2
		default:
			if pos+4 > len(payload) {
				return nil
			}
			id = binary.BigEndian.Uint32(payload[pos : pos+4])
			pos += 4
		}
		if pos >= len(payload) {
			return nil
		}
		assocCount := int(payload[pos])
		pos++
		indices := make([]uint8, 0, assocCount)
		for j := 0; j < assocCount && pos < len(payload); j++ {
			indices = append(indices, payload[pos]&0x7f)
			pos++
		}
		if id == itemID {
			return indices
		}
	}
	return nil
}

// heicTransformOrientation maps irot to EXIF-style orientation labels used elsewhere.
func heicTransformOrientation(t heicTransform) string {
	if !t.found {
		return ""
	}
	switch t.irot & 0x03 {
	case 0:
		return "Horizontal (normal)"
	case 1:
		return "Rotate 270 CW"
	case 2:
		return "Rotate 180"
	case 3:
		return "Rotate 90 CW"
	default:
		return ""
	}
}
