package opensw

import (
	"bytes"
	"encoding/binary"
	"errors"
	"fmt"
	"image"
	"image/color"
	"image/png"
	"os"
)

// Embedded preview extraction, ported from OpenSW (sldprt properties.py):
// the document stores a plain PNG ("PreviewPNG"), a legacy 256-colour DIB
// ("Preview") and, for drawings, one image per sheet ("Images/Sheet_<n>").
// Everything is read straight from the container, no geometry involved.

var pngSignature = []byte{0x89, 'P', 'N', 'G', '\r', '\n', 0x1a, '\n'}

// maxDIBPixels caps the decoded size of the legacy DIB preview so a crafted
// header cannot force a full-image allocation.
const maxDIBPixels = 64 << 20 // 64 MP

// maxSolidWorksDocSize caps how much of a document the preview extraction
// reads into memory; larger files simply have no embedded preview served.
const maxSolidWorksDocSize = 512 << 20 // 512 MB

// ExtractSolidWorksPreview returns the embedded preview PNG of a SolidWorks
// document (.sldprt/.sldasm/.slddrw), in OpenSW's order: PreviewPNG, then the
// legacy "Preview" DIB converted pixel-for-pixel, then the drawing's first
// sheet image.
func ExtractSolidWorksPreview(realPath string) ([]byte, error) {
	info, err := os.Stat(realPath)
	if err != nil {
		return nil, err
	}
	if info.Size() > maxSolidWorksDocSize {
		return nil, fmt.Errorf("opensw: document larger than the preview cap (%d bytes)", info.Size())
	}
	data, err := os.ReadFile(realPath)
	if err != nil {
		return nil, err
	}
	return ExtractSolidWorksPreviewBytes(data)
}

// ExtractSolidWorksPreviewBytes is ExtractSolidWorksPreview on in-memory data.
func ExtractSolidWorksPreviewBytes(data []byte) ([]byte, error) {
	c, err := OpenContainer(data)
	if err != nil {
		return nil, err
	}
	if b := c.Get("PreviewPNG"); bytes.HasPrefix(b, pngSignature) {
		return b, nil
	}
	if b := c.Get("Preview"); len(b) > 0 {
		if png, err := dibToPNG(b); err == nil {
			return png, nil
		}
	}
	if b := c.Get("Images/Sheet_0"); bytes.HasPrefix(b, pngSignature) {
		return b, nil
	}
	return nil, errors.New("opensw: no embedded preview found")
}

// dibToPNG converts the legacy 8-bit paletted "Preview" stream to PNG, pixel
// for pixel (palette kept as the PNG's own). Port of OpenSW's _dib_png:
// uncompressed rows are 4-byte padded; RLE8 runs and deltas outside a row or
// past the end leave palette index 0, as Windows draws them.
func dibToPNG(data []byte) ([]byte, error) {
	if len(data) < 44 || binary.LittleEndian.Uint32(data[4:8]) != 40 {
		return nil, errors.New("opensw: not a 40-byte BITMAPINFOHEADER")
	}
	w := int(int32(binary.LittleEndian.Uint32(data[8:12])))
	height := int(int32(binary.LittleEndian.Uint32(data[12:16])))
	bits := binary.LittleEndian.Uint16(data[18:20])
	compression := binary.LittleEndian.Uint32(data[20:24])
	h := height
	if h < 0 {
		h = -h
	}
	if bits != 8 || (compression != 0 && compression != 1) ||
		w <= 0 || h <= 0 || w > 20_000 || h > 20_000 {
		return nil, errors.New("opensw: unsupported DIB variant")
	}
	// Bound the decoded image before allocating anything: a crafted header
	// must not turn a tiny payload into a full-image allocation.
	if w*h > maxDIBPixels {
		return nil, fmt.Errorf("opensw: DIB dimensions %dx%d exceed the pixel limit", w, h)
	}
	colours := int(binary.LittleEndian.Uint32(data[36:40]))
	if colours == 0 {
		colours = 256
	}
	start := 44 + 4*colours
	if colours > 256 || start > len(data) {
		return nil, errors.New("opensw: bad DIB palette")
	}
	palette := make(color.Palette, colours)
	for k := range palette {
		palette[k] = color.RGBA{
			B: data[44+4*k],
			G: data[45+4*k],
			R: data[46+4*k],
			A: 0xFF,
		}
	}

	rows := make([][]byte, h)
	for y := range rows {
		rows[y] = make([]byte, w)
	}
	pixels := data[start:]
	if compression == 0 {
		step := (w + 3) &^ 3
		if int64(len(pixels)) < int64(step)*int64(h) {
			return nil, errors.New("opensw: short uncompressed DIB")
		}
		for y := 0; y < h; y++ {
			copy(rows[y], pixels[y*step:y*step+w])
		}
	} else {
		x, y, i := 0, 0, 0
		n := len(pixels)
		sawEOF := false
		for i+1 < n {
			count, value := int(pixels[i]), int(pixels[i+1])
			i += 2
			if count > 0 {
				end := min(x+count, w)
				if y < h && end > x {
					for j := x; j < end; j++ {
						rows[y][j] = byte(value)
					}
				}
				x += count
			} else {
				switch value {
				case 0: // end of line
					x, y = 0, y+1
				case 1: // end of bitmap
					sawEOF = true
				case 2: // delta: skip pixels, keeping index 0
					if i+1 >= n {
						return nil, errors.New("opensw: truncated RLE8 delta")
					}
					x, y = x+int(pixels[i]), y+int(pixels[i+1])
					i += 2
				default: // absolute run, padded to a word
					if i+value > n || value+(value&1) > n-i {
						return nil, errors.New("opensw: truncated RLE8 run")
					}
					run := pixels[i : i+value]
					end := min(x+value, w)
					if y < h && end > x {
						copy(rows[y][x:end], run[:end-x])
					}
					x += value
					i += value + (value & 1)
				}
			}
			if sawEOF {
				break
			}
		}
		if !sawEOF {
			return nil, errors.New("opensw: RLE8 stream without end marker")
		}
	}
	if height > 0 { // bottom-up DIB
		for a, b := 0, h-1; a < b; a, b = a+1, b-1 {
			rows[a], rows[b] = rows[b], rows[a]
		}
	}

	img := image.NewPaletted(image.Rect(0, 0, w, h), palette)
	for y := 0; y < h; y++ {
		copy(img.Pix[img.PixOffset(0, y):], rows[y])
	}
	var buf bytes.Buffer
	if err := png.Encode(&buf, img); err != nil {
		return nil, err
	}
	return buf.Bytes(), nil
}
