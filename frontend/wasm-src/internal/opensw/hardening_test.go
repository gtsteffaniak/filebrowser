package opensw

import (
	"encoding/binary"
	"errors"
	"strings"
	"testing"
)

// --- container hardening ------------------------------------------------------

func TestWalkCyclicDirectory(t *testing.T) {
	f := &cfbFile{paths: map[string]int{}}
	// Entry 1 (storage) whose left sibling points back at itself, forming a
	// cycle through the left pointer.
	f.entries = []cfbEntry{
		{name: "root", entryType: 5, child: 1, left: cfbFreeSector, right: cfbFreeSector},
		{name: "s", entryType: 1, left: 1, right: cfbFreeSector, child: cfbFreeSector},
	}
	err := f.walk(1, "", map[int]bool{0: true})
	if err == nil || !strings.Contains(err.Error(), "cyclic") {
		t.Fatalf("expected cyclic-directory error, got %v", err)
	}
}

func TestWalkCyclesThroughChildAndRight(t *testing.T) {
	// Cycle through the child pointer: 1 -> 2 -> 1.
	f := &cfbFile{paths: map[string]int{}}
	f.entries = []cfbEntry{
		{name: "root", entryType: 5, child: 1, left: cfbFreeSector, right: cfbFreeSector},
		{name: "a", entryType: 1, left: cfbFreeSector, right: cfbFreeSector, child: 2},
		{name: "b", entryType: 1, left: cfbFreeSector, right: 1, child: cfbFreeSector},
	}
	if err := f.walk(1, "", map[int]bool{0: true}); err == nil {
		t.Fatal("expected error for child/right cycle, got nil")
	}
	// Cycle through the right pointer only.
	f.entries = []cfbEntry{
		{name: "root", entryType: 5, child: 1, left: cfbFreeSector, right: cfbFreeSector},
		{name: "a", entryType: 1, left: cfbFreeSector, right: 2, child: cfbFreeSector},
		{name: "b", entryType: 1, left: cfbFreeSector, right: 1, child: cfbFreeSector},
	}
	if err := f.walk(1, "", map[int]bool{0: true}); err == nil {
		t.Fatal("expected error for right-pointer cycle, got nil")
	}
}

func TestReadChainSelfReferencingSector(t *testing.T) {
	// A FAT whose only entry points at itself: the chain never terminates.
	f := &cfbFile{data: make([]byte, 512+64), sectorSize: 64}
	fat := []uint32{0}
	if _, err := f.readChain(fat, 0, 1<<62); err == nil {
		t.Fatal("expected FAT-loop error, got nil")
	}
}

func TestReadChainTerminatesNormally(t *testing.T) {
	f := &cfbFile{data: make([]byte, 512+128), sectorSize: 64}
	fat := []uint32{1, cfbEndOfChain}
	out, err := f.readChain(fat, 0, 1<<62)
	if err != nil {
		t.Fatalf("valid chain failed: %v", err)
	}
	if len(out) != 128 {
		t.Fatalf("expected 128 bytes, got %d", len(out))
	}
}

func TestReadCFBRejectsOversizedDIFATCount(t *testing.T) {
	// Minimal header claiming ~4 billion DIFAT sectors in a 512-byte file.
	data := make([]byte, 512)
	copy(data, ole2Magic)
	binary.LittleEndian.PutUint16(data[30:32], 9) // 512-byte sectors
	binary.LittleEndian.PutUint16(data[32:34], 6)
	binary.LittleEndian.PutUint32(data[72:76], 0xFFFF_FFFF)
	_, err := readCFB(data)
	if err == nil || !strings.Contains(err.Error(), "beyond file size") {
		t.Fatalf("expected DIFAT bound error, got %v", err)
	}
}

func TestParseEntryNameLengthBounds(t *testing.T) {
	e := make([]byte, 128)
	e[66] = 2 // stream
	// nameLen 66: does not fit the 64-byte name field.
	binary.LittleEndian.PutUint16(e[64:66], 66)
	if _, err := parseEntry(e); err == nil {
		t.Fatal("expected error for nameLen > 64, got nil")
	}
	// Odd nameLen cannot be UTF-16 units plus terminator.
	binary.LittleEndian.PutUint16(e[64:66], 5)
	if _, err := parseEntry(e); err == nil {
		t.Fatal("expected error for odd nameLen, got nil")
	}
	// Valid: 4 bytes = one character plus the NUL terminator.
	binary.LittleEndian.PutUint16(e[64:66], 4)
	binary.LittleEndian.PutUint16(e[0:2], 'a')
	if _, err := parseEntry(e); err != nil {
		t.Fatalf("valid record rejected: %v", err)
	}
}

func TestParseEntrySizeHighBitRejected(t *testing.T) {
	e := make([]byte, 128)
	e[66] = 2
	binary.LittleEndian.PutUint16(e[64:66], 0)
	binary.LittleEndian.PutUint64(e[120:128], 0x8000_0000_0000_0000)
	_, err := parseEntry(e)
	if err == nil || !strings.Contains(err.Error(), "out of range") {
		t.Fatalf("expected size range error, got %v", err)
	}
}

// --- assembly hardening -------------------------------------------------------

const assemblyXMLNS = `xmlns="http://www.solidworks.com/sw2003/schema"`
const unitTransform = "1 0 0 0 1 0 0 0 1 0 0 0 0 0 0 1"

func TestTreeComponentsSelfReferenceCycle(t *testing.T) {
	xml := `<root ` + assemblyXMLNS + `>` +
		`<swFile id="1" swDocType="ASSEMBLY"/>` +
		`<swModel id="m1" swFileRef="1">` +
		`<swReference swName="self" swReferenceNumber="1" swTransform="` + unitTransform + `" swModelRef="m1"/>` +
		`</swModel></root>`
	_, err := treeComponents([]byte(xml))
	if !errors.Is(err, ErrAssembly) || !strings.Contains(err.Error(), "cyclic") {
		t.Fatalf("expected cyclic-assembly error, got %v", err)
	}
}

func TestTreeComponentsTwoModelCycle(t *testing.T) {
	xml := `<root ` + assemblyXMLNS + `>` +
		`<swFile id="1" swDocType="ASSEMBLY"/>` +
		`<swFile id="2" swDocType="ASSEMBLY"/>` +
		`<swModel id="m1" swFileRef="1">` +
		`<swReference swName="a" swReferenceNumber="1" swTransform="` + unitTransform + `" swModelRef="m2"/>` +
		`</swModel>` +
		`<swModel id="m2" swFileRef="2">` +
		`<swReference swName="b" swReferenceNumber="1" swTransform="` + unitTransform + `" swModelRef="m1"/>` +
		`</swModel></root>`
	_, err := treeComponents([]byte(xml))
	if !errors.Is(err, ErrAssembly) || !strings.Contains(err.Error(), "cyclic") {
		t.Fatalf("expected cyclic-assembly error, got %v", err)
	}
}

func TestTreeComponentsSameSubassemblyInTwoBranchesIsValid(t *testing.T) {
	sub := `<swFile id="2" swDocType="ASSEMBLY"/><swModel id="m2" swFileRef="2">` +
		`<swReference swName="p" swReferenceNumber="1" swTransform="` + unitTransform + `" swModelRef="none"/>` +
		`</swModel>`
	xml := `<root ` + assemblyXMLNS + `>` +
		`<swFile id="1" swDocType="ASSEMBLY"/>` + sub +
		`<swModel id="m1" swFileRef="1">` +
		`<swReference swName="l" swReferenceNumber="1" swTransform="` + unitTransform + `" swModelRef="m2"/>` +
		`<swReference swName="r" swReferenceNumber="1" swTransform="` + unitTransform + `" swModelRef="m2"/>` +
		`</swModel></root>`
	comps, err := treeComponents([]byte(xml))
	if err != nil {
		t.Fatalf("same subassembly in separate branches must be valid, got %v", err)
	}
	// Two instances of the shared subassembly's part plus nothing else.
	if len(comps) != 2 {
		t.Fatalf("expected 2 components, got %d", len(comps))
	}
}

func TestTreeComponentsPropagatesBOMExclusionFromSubassembly(t *testing.T) {
	xml := `<root ` + assemblyXMLNS + `>` +
		`<swFile id="1" swDocType="ASSEMBLY"/>` +
		`<swFile id="2" swDocType="ASSEMBLY"/>` +
		`<swFile id="3" swDocType="PART" swPath="C:\parts\bolt.sldprt"/>` +
		`<swModel id="m1" swFileRef="1">` +
		`<swReference swName="sub" swReferenceNumber="1" swTransform="` + unitTransform + `" swModelRef="m2" swExcludeFromBOM="YES"/>` +
		`</swModel>` +
		`<swModel id="m2" swFileRef="2">` +
		`<swReference swName="bolt" swReferenceNumber="1" swTransform="` + unitTransform + `" swModelRef="m3"/>` +
		`</swModel>` +
		`<swModel id="m3" swFileRef="3"/>` +
		`</root>`
	components, err := treeComponents([]byte(xml))
	if err != nil {
		t.Fatalf("treeComponents() error = %v", err)
	}
	if len(components) != 1 || !components[0].ExcludeFromBOM {
		t.Fatalf("subassembly BOM exclusion was not inherited: %+v", components)
	}
}

func TestSwMatrixRejectsNonUnitScale(t *testing.T) {
	_, err := swMatrix("1 0 0 0 1 0 0 0 1 0 0 0 1 0 0 2")
	if !errors.Is(err, ErrAssembly) {
		t.Fatalf("expected non-unit scale rejection, got %v", err)
	}
	if _, err := swMatrix(unitTransform); err != nil {
		t.Fatalf("unit scale must keep working: %v", err)
	}
}

// --- DIB preview hardening ----------------------------------------------------

// dibHeader builds a minimal 8-bit paletted DIB header (40-byte
// BITMAPINFOHEADER, 2 palette colours) with the given dimensions.
func dibHeader(w, h int32, compression uint32) []byte {
	hdr := make([]byte, 44)
	binary.LittleEndian.PutUint32(hdr[4:8], 40)
	binary.LittleEndian.PutUint32(hdr[8:12], uint32(w))
	binary.LittleEndian.PutUint32(hdr[12:16], uint32(h))
	binary.LittleEndian.PutUint16(hdr[18:20], 8)    // bits per pixel
	binary.LittleEndian.PutUint32(hdr[20:24], compression)
	binary.LittleEndian.PutUint32(hdr[36:40], 2)    // colours used
	return hdr
}

func TestDibToPNGRejectsOversizedDimensions(t *testing.T) {
	// 20 000 x 20 000 passes the per-axis check but exceeds the pixel cap.
	data := dibHeader(20_000, 20_000, 0)
	if _, err := dibToPNG(data); err == nil || !strings.Contains(err.Error(), "pixel limit") {
		t.Fatalf("expected pixel-limit error, got %v", err)
	}
	// Same dimensions with an RLE marker must also be rejected up front.
	data = dibHeader(20_000, 20_000, 1)
	if _, err := dibToPNG(data); err == nil || !strings.Contains(err.Error(), "pixel limit") {
		t.Fatalf("expected pixel-limit error for RLE, got %v", err)
	}
}

func TestDibToPNGRLETruncatedAbsoluteRun(t *testing.T) {
	// [0, 255]: an absolute run of 255 bytes at the very end of the stream.
	data := dibHeader(4, 2, 1)
	palette := make([]byte, 8) // 2 colours
	data = append(append(data, palette...), 0, 255)
	if _, err := dibToPNG(data); err == nil || !strings.Contains(err.Error(), "truncated RLE8") {
		t.Fatalf("expected truncated-run error, got %v", err)
	}
}
