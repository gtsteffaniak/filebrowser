package opensw

import (
	"bytes"
	"encoding/binary"
)

var appearanceField = []byte{1, 0, 0, 0, 0xff, 0xff, 0xff, 0xff}

func displayFaceColors(data []byte, faces []displayFaceBlock) [][]float64 {
	colors := make([][]float64, len(faces))
	if len(faces) == 0 {
		return colors
	}

	defaultColor := displayAppearanceColor(data, 0, faces[0].start)
	known := make(map[uint16][]float64)
	var expected []float64

	for i, face := range faces {
		if defaultColor != nil {
			colors[i] = defaultColor
		}
		nextStart := len(data)
		if i+1 < len(faces) {
			nextStart = faces[i+1].start
		}
		if face.end < 0 || face.end > nextStart || nextStart > len(data) {
			continue
		}
		offset := bytes.Index(data[face.end:nextStart], appearanceField)
		if offset < 0 {
			continue
		}
		tagOffset := face.end + offset + len(appearanceField)
		if tagOffset+2 > nextStart {
			continue
		}
		tag := binary.LittleEndian.Uint16(data[tagOffset:])
		switch {
		case tag == 0xffff:
			continue
		case tag&0x8000 != 0:
			if tagOffset+5 > nextStart {
				continue
			}
			color := rgbColor(data[tagOffset+2 : tagOffset+5])
			colors[i] = color
			expected = color
		case tag != 0:
			if _, exists := known[tag]; !exists && expected != nil {
				known[tag] = expected
			}
			expected = nil
			if color, exists := known[tag]; exists {
				colors[i] = color
			}
		}
	}
	return colors
}

func displayAppearanceColor(data []byte, start, end int) []float64 {
	if start < 0 || end > len(data) || start >= end {
		return nil
	}
	for pos := start; pos+len(utf16Mark) <= end; {
		relative := bytes.Index(data[pos:end], utf16Mark)
		if relative < 0 {
			break
		}
		offset := pos + relative
		name, afterName, ok := mfcCString(data, offset)
		if ok && name != "" && afterName+4 <= end && (data[afterName] == 2 || data[afterName] == 3) {
			return rgbColor(data[afterName+1 : afterName+4])
		}
		pos = offset + len(utf16Mark)
	}
	return nil
}

func rgbColor(rgb []byte) []float64 {
	return []float64{
		float64(rgb[0]) / 255,
		float64(rgb[1]) / 255,
		float64(rgb[2]) / 255,
	}
}
