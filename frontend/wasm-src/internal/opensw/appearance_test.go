package opensw

import (
	"encoding/binary"
	"math"
	"testing"
)

func TestDisplayFaceColorsUsePartAndFaceAppearances(t *testing.T) {
	var data []byte
	data = appendMFCColor(data, "defaultplastic", 2, [3]byte{51, 102, 153})
	data = append(data, testDisplayFace(0)...)
	data = appendAppearance(data, 0x8001, [3]byte{204, 153, 102})
	data = append(data, testDisplayFace(2)...)
	data = appendAppearance(data, 1, [3]byte{})
	data = append(data, testDisplayFace(4)...)

	faces := blockFaceBlocks(data, 0, len(data))
	if len(faces) != 3 {
		t.Fatalf("expected 3 display faces, got %d", len(faces))
	}
	colors := displayFaceColors(data, faces)
	assertColor(t, colors[0], [3]float64{204.0 / 255, 153.0 / 255, 102.0 / 255})
	assertColor(t, colors[1], [3]float64{204.0 / 255, 153.0 / 255, 102.0 / 255})
	assertColor(t, colors[2], [3]float64{51.0 / 255, 102.0 / 255, 153.0 / 255})
}

func TestDisplayFaceColorsLeaveMissingAppearanceUnset(t *testing.T) {
	data := testDisplayFace(0)
	faces := blockFaceBlocks(data, 0, len(data))
	colors := displayFaceColors(data, faces)
	if len(colors) != 1 || colors[0] != nil {
		t.Fatalf("expected no color for a part without appearance metadata, got %v", colors)
	}
}

func appendMFCColor(data []byte, name string, kind byte, rgb [3]byte) []byte {
	data = append(data, utf16Mark...)
	data = append(data, byte(len(name)))
	for _, character := range name {
		data = append(data, byte(character), 0)
	}
	data = append(data, kind, rgb[0], rgb[1], rgb[2])
	return data
}

func appendAppearance(data []byte, tag uint16, rgb [3]byte) []byte {
	data = append(data, appearanceField...)
	var encoded [2]byte
	binary.LittleEndian.PutUint16(encoded[:], tag)
	data = append(data, encoded[:]...)
	if tag&0x8000 != 0 && tag != 0xffff {
		data = append(data, rgb[:]...)
	}
	return data
}

func testDisplayFace(x float32) []byte {
	data := make([]byte, 20+16+36)
	binary.LittleEndian.PutUint32(data[0:4], 4)
	binary.LittleEndian.PutUint32(data[4:8], 8)
	binary.LittleEndian.PutUint32(data[8:12], 2)
	binary.LittleEndian.PutUint32(data[12:16], 1)
	binary.LittleEndian.PutUint32(data[16:20], 3)
	binary.LittleEndian.PutUint32(data[20:24], 12)
	binary.LittleEndian.PutUint32(data[24:28], 0x64)
	binary.LittleEndian.PutUint32(data[28:32], 2)
	binary.LittleEndian.PutUint32(data[32:36], 3)
	for i, value := range []float32{x, 0, 0, x + 1, 0, 0, x, 1, 0} {
		binary.LittleEndian.PutUint32(data[36+i*4:40+i*4], math.Float32bits(value))
	}
	return data
}

func assertColor(t *testing.T, got []float64, want [3]float64) {
	t.Helper()
	if len(got) != 3 {
		t.Fatalf("expected RGB color, got %v", got)
	}
	for i := range want {
		if math.Abs(got[i]-want[i]) > 1e-9 {
			t.Fatalf("color channel %d = %g, want %g", i, got[i], want[i])
		}
	}
}
