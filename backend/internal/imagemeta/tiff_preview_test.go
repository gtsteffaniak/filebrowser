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
		subIFDJPEG:  largeJPEG,
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

type tiffFixture struct {
	thumbnailJPEG   []byte
	subIFDJPEG      []byte
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
		{tag: tiffTagSubIFDs, typ: tiffTypeLong, count: 1, value: ifd1Off},
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
	tag, typ       uint16
	count, value   uint32
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
