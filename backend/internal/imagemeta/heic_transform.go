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

// parseHEICTransform reads the native irot property from a HEIC/HEIF file.
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
	return scanHEICTransformBoxes(buf[:n])
}

// scanHEICTransformBoxes locates standard 9-byte irot property boxes in BMFF metadata.
func scanHEICTransformBoxes(data []byte) heicTransform {
	var t heicTransform
	for i := 4; i+8 <= len(data); i++ {
		if binary.BigEndian.Uint32(data[i-4:i]) != 9 {
			continue
		}
		if string(data[i:i+4]) != "irot" {
			continue
		}
		t.irot = data[i+4] & 0x03
		t.found = true
	}
	return t
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
