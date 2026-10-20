//go:build !386 && !arm

package imagemeta

import (
	"bytes"
	"encoding/binary"
	"os"
	"path/filepath"
	"testing"
)

var (
	smallJPEG = []byte{0xff, 0xd8, 0xff, 0xd9}
	largeJPEG = bytes.Repeat([]byte{0xff, 0xd8, 0xff, 0x00}, 512)
)

func TestCollectTIFFPreviewCandidatesPrefersSubIFDLargePreview(t *testing.T) {
	data := buildSyntheticTIFF(tiffFixture{
		thumbnailJPEG: smallJPEG,
		subIFDJPEG:    largeJPEG,
	})
	f := openBytesFile(t, data)

	candidates, err := collectTIFFPreviewCandidates(f)
	if err != nil {
		t.Fatal(err)
	}
	preview, err := readLargestJPEGPreview(f, candidates)
	if err != nil {
		t.Fatal(err)
	}
	if !IsJPEG(preview) {
		t.Fatalf("expected JPEG preview, got %d bytes", len(preview))
	}
	if len(preview) != len(largeJPEG) {
		t.Fatalf("preview size = %d, want %d (largest candidate)", len(preview), len(largeJPEG))
	}
}

func TestCollectTIFFPreviewCandidatesIgnoresMalformedOffsets(t *testing.T) {
	data := buildSyntheticTIFF(tiffFixture{
		thumbnailJPEG:   smallJPEG,
		subIFDJPEG:      largeJPEG,
		badJPEGInterchg: true,
	})
	f := openBytesFile(t, data)

	candidates, err := collectTIFFPreviewCandidates(f)
	if err != nil {
		t.Fatal(err)
	}
	preview, err := readLargestJPEGPreview(f, candidates)
	if err != nil {
		t.Fatal(err)
	}
	if len(preview) != len(largeJPEG) {
		t.Fatalf("preview size = %d, want %d", len(preview), len(largeJPEG))
	}
}

func TestCollectTIFFPreviewCandidatesIgnoresMalformedSubIFDOffset(t *testing.T) {
	data := buildSyntheticTIFF(tiffFixture{
		thumbnailJPEG: smallJPEG,
		subIFDJPEG:    largeJPEG,
		badJpgFromRaw: true,
	})
	f := openBytesFile(t, data)

	candidates, err := collectTIFFPreviewCandidates(f)
	if err != nil {
		t.Fatal(err)
	}
	preview, err := readLargestJPEGPreview(f, candidates)
	if err != nil {
		t.Fatal(err)
	}
	if len(preview) != len(smallJPEG) {
		t.Fatalf("preview size = %d, want %d", len(preview), len(smallJPEG))
	}
}

func TestCollectTIFFPreviewCandidatesBoundsIFDTraversal(t *testing.T) {
	data := buildChainedTIFF(maxTIFFIFDsVisited + 16)
	f := openBytesFile(t, data)

	candidates, err := collectTIFFPreviewCandidates(f)
	if err != nil {
		t.Fatal(err)
	}
	if len(candidates) > maxTIFFPreviewCandidates {
		t.Fatalf("candidate count = %d, want <= %d", len(candidates), maxTIFFPreviewCandidates)
	}
}

func TestTiffTagUint32sSupportsIFDType(t *testing.T) {
	data := buildSyntheticTIFF(tiffFixture{
		thumbnailJPEG: smallJPEG,
		subIFDJPEG:    largeJPEG,
		subIFDType:    tiffTypeIFD,
	})
	f := openBytesFile(t, data)

	candidates, err := collectTIFFPreviewCandidates(f)
	if err != nil {
		t.Fatal(err)
	}
	preview, err := readLargestJPEGPreview(f, candidates)
	if err != nil {
		t.Fatal(err)
	}
	if len(preview) != len(largeJPEG) {
		t.Fatalf("preview size = %d, want %d (SubIFD type IFD)", len(preview), len(largeJPEG))
	}
}

func TestExtractEmbeddedPreviewFallbackWhenDecodeFails(t *testing.T) {
	// Synthetic ARW where SubIFD offset is placed after IFD2, and a SubIFD tag
	// points past EOF so extimagemeta.Decode fails with EOF, while IFD2 holds a
	// valid JPEGInterchangeFormat preview.
	order := binary.LittleEndian
	buf := make([]byte, 32*1024)
	copy(buf[0:2], []byte("II"))
	order.PutUint16(buf[2:4], tiffMagicStandard)
	ifd0Off := uint32(8)
	order.PutUint32(buf[4:8], ifd0Off)

	thumbOff := uint32(4096)
	copy(buf[thumbOff:], smallJPEG)
	largeOff := uint32(8192)
	copy(buf[largeOff:], largeJPEG)

	ifd1Off := uint32(256)
	ifd2Off := uint32(512)
	subIFDOff := uint32(768)

	writeIFD := func(offset uint32, entries []tiffEntry, next uint32) {
		off := int(offset)
		order.PutUint16(buf[off:off+2], uint16(len(entries)))
		entryBase := off + 2
		for i, entry := range entries {
			pos := entryBase + i*12
			order.PutUint16(buf[pos:pos+2], entry.tag)
			order.PutUint16(buf[pos+2:pos+4], entry.typ)
			order.PutUint32(buf[pos+4:pos+8], entry.count)
			order.PutUint32(buf[pos+8:pos+12], entry.value)
		}
		order.PutUint32(buf[entryBase+len(entries)*12:entryBase+len(entries)*12+4], next)
	}

	writeIFD(ifd0Off, []tiffEntry{
		{tag: tiffTagOrientation, typ: tiffTypeShort, count: 1, value: 6},
		{tag: tiffTagSubIFDs, typ: tiffTypeLong, count: 1, value: subIFDOff},
		{tag: tiffTagJPEGInterchange, typ: tiffTypeLong, count: 1, value: thumbOff},
		{tag: tiffTagJPEGInterchangeLen, typ: tiffTypeLong, count: 1, value: uint32(len(smallJPEG))},
	}, ifd1Off)

	writeIFD(ifd1Off, []tiffEntry{
		{tag: tiffTagJPEGInterchange, typ: tiffTypeLong, count: 1, value: thumbOff},
		{tag: tiffTagJPEGInterchangeLen, typ: tiffTypeLong, count: 1, value: uint32(len(smallJPEG))},
	}, ifd2Off)

	writeIFD(ifd2Off, []tiffEntry{
		{tag: tiffTagJPEGInterchange, typ: tiffTypeLong, count: 1, value: largeOff},
		{tag: tiffTagJPEGInterchangeLen, typ: tiffTypeLong, count: 1, value: uint32(len(largeJPEG))},
	}, 0)

	// SubIFD has a StripOffsets tag with count=2 pointing beyond file end so Decode fails with EOF.
	writeIFD(subIFDOff, []tiffEntry{
		{tag: tiffTagStripOffsets, typ: tiffTypeLong, count: 2, value: uint32(len(buf) + 4096)},
	}, 0)

	path := filepath.Join(t.TempDir(), "sample.arw")
	if err := os.WriteFile(path, buf[:int(largeOff)+len(largeJPEG)], 0o600); err != nil {
		t.Fatal(err)
	}

	preview, err := ExtractEmbeddedPreview(t.Context(), path)
	if err != nil {
		t.Fatalf("ExtractEmbeddedPreview() err = %v", err)
	}
	if len(preview) != len(largeJPEG) {
		t.Fatalf("ExtractEmbeddedPreview() len = %d, want %d", len(preview), len(largeJPEG))
	}
	if got := GetOrientation(t.Context(), path); got != "Rotate 90 CW" {
		t.Fatalf("GetOrientation() = %q, want %q", got, "Rotate 90 CW")
	}
}

func TestExtractEmbeddedPreviewSupportsRW2AndStripPreview(t *testing.T) {
	order := binary.LittleEndian
	buf := make([]byte, 16*1024)
	copy(buf[0:2], []byte("II"))
	order.PutUint16(buf[2:4], tiffMagicPanasonicRW2)
	ifd0Off := uint32(8)
	order.PutUint32(buf[4:8], ifd0Off)

	largeOff := uint32(4096)
	copy(buf[largeOff:], largeJPEG)

	entries := []tiffEntry{
		{tag: tiffTagStripOffsets, typ: tiffTypeLong, count: 1, value: largeOff},
		{tag: tiffTagStripByteCounts, typ: tiffTypeLong, count: 1, value: uint32(len(largeJPEG))},
	}
	order.PutUint16(buf[ifd0Off:ifd0Off+2], uint16(len(entries)))
	for i, entry := range entries {
		pos := int(ifd0Off) + 2 + i*12
		order.PutUint16(buf[pos:pos+2], entry.tag)
		order.PutUint16(buf[pos+2:pos+4], entry.typ)
		order.PutUint32(buf[pos+4:pos+8], entry.count)
		order.PutUint32(buf[pos+8:pos+12], entry.value)
	}

	path := filepath.Join(t.TempDir(), "sample.rw2")
	if err := os.WriteFile(path, buf[:int(largeOff)+len(largeJPEG)], 0o600); err != nil {
		t.Fatal(err)
	}

	preview, err := ExtractEmbeddedPreview(t.Context(), path)
	if err != nil {
		t.Fatalf("ExtractEmbeddedPreview() err = %v", err)
	}
	if len(preview) != len(largeJPEG) {
		t.Fatalf("ExtractEmbeddedPreview() len = %d, want %d", len(preview), len(largeJPEG))
	}
}

func TestExtractEmbeddedPreviewDSC02672(t *testing.T) {
	path := "/Users/steffag/Downloads/DSC02672-first-4MB.ARW"
	if _, err := os.Stat(path); err != nil {
		t.Skip(path)
	}

	got, err := ExtractEmbeddedPreview(t.Context(), path)
	if err != nil {
		t.Fatal(err)
	}
	if !IsJPEG(got) {
		t.Fatalf("expected JPEG preview, got %d bytes", len(got))
	}
	if len(got) != 2954147 {
		t.Fatalf("preview size = %d, want 2954147", len(got))
	}
	if orient := GetOrientation(t.Context(), path); orient != "Horizontal (normal)" {
		t.Fatalf("GetOrientation() = %q, want Horizontal (normal)", orient)
	}
}

type tiffFixture struct {
	thumbnailJPEG   []byte
	subIFDJPEG      []byte
	subIFDType      uint16
	badJpgFromRaw   bool
	badJPEGInterchg bool
}

func openBytesFile(t *testing.T, data []byte) *os.File {
	t.Helper()
	path := filepath.Join(t.TempDir(), "sample.tiff")
	if err := os.WriteFile(path, data, 0o600); err != nil {
		t.Fatal(err)
	}
	f, err := os.Open(path)
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = f.Close() })
	return f
}

func buildSyntheticTIFF(fix tiffFixture) []byte {
	order := binary.LittleEndian
	if fix.subIFDType == 0 {
		fix.subIFDType = tiffTypeLong
	}
	buf := make([]byte, 64*1024)

	thumbOff := uint32(4096)
	copy(buf[thumbOff:], fix.thumbnailJPEG)
	largeOff := uint32(8192)
	copy(buf[largeOff:], fix.subIFDJPEG)

	ifd1Off := uint32(256)
	ifd0Off := uint32(8)

	writeIFD := func(offset uint32, entries []tiffEntry, next uint32) {
		off := int(offset)
		order.PutUint16(buf[off:off+2], uint16(len(entries)))
		entryBase := off + 2
		for i, entry := range entries {
			pos := entryBase + i*12
			order.PutUint16(buf[pos:pos+2], entry.tag)
			order.PutUint16(buf[pos+2:pos+4], entry.typ)
			order.PutUint32(buf[pos+4:pos+8], entry.count)
			order.PutUint32(buf[pos+8:pos+12], entry.value)
		}
		order.PutUint32(buf[entryBase+len(entries)*12:entryBase+len(entries)*12+4], next)
	}

	ifd1Entries := []tiffEntry{
		{tag: tiffTagJpgFromRaw, typ: tiffTypeUndefined, count: uint32(len(fix.subIFDJPEG)), value: largeOff},
	}
	if fix.badJpgFromRaw {
		ifd1Entries[0].value = 1_000_000
	}
	writeIFD(ifd1Off, ifd1Entries, 0)

	ifd0Entries := []tiffEntry{
		{tag: tiffTagSubIFDs, typ: fix.subIFDType, count: 1, value: ifd1Off},
		{tag: tiffTagJPEGInterchange, typ: tiffTypeLong, count: 1, value: thumbOff},
		{tag: tiffTagJPEGInterchangeLen, typ: tiffTypeLong, count: 1, value: uint32(len(fix.thumbnailJPEG))},
	}
	if fix.badJPEGInterchg {
		ifd0Entries[1].value = 2_000_000
	}
	writeIFD(ifd0Off, ifd0Entries, 0)

	copy(buf[0:2], []byte("II"))
	order.PutUint16(buf[2:4], 42)
	order.PutUint32(buf[4:8], ifd0Off)

	end := int(largeOff) + len(fix.subIFDJPEG)
	if end < 1024 {
		end = 1024
	}
	return buf[:end]
}

type tiffEntry struct {
	tag, typ     uint16
	count, value uint32
}

func buildChainedTIFF(chainLen int) []byte {
	order := binary.LittleEndian
	const ifdSize = 6 // 2-byte entry count + 4-byte next IFD pointer
	buf := make([]byte, 8+chainLen*ifdSize)
	copy(buf[0:2], []byte("II"))
	order.PutUint16(buf[2:4], 42)
	order.PutUint32(buf[4:8], 8)

	for i := 0; i < chainLen; i++ {
		off := 8 + i*ifdSize
		order.PutUint16(buf[off:off+2], 0)
		var next uint32
		if i+1 < chainLen {
			next = uint32(8 + (i+1)*ifdSize)
		}
		order.PutUint32(buf[off+2:off+6], next)
	}
	return buf
}
