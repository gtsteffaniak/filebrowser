//go:build !386 && !arm

package imagemeta

import (
	"encoding/binary"
	"os"
	"path/filepath"
	"testing"
)

func TestHeicTransformOrientationMapping(t *testing.T) {
	tests := []struct {
		name string
		t    heicTransform
		want string
	}{
		{
			name: "irot normal",
			t:    heicTransform{irot: 0, found: true},
			want: "Horizontal (normal)",
		},
		{
			name: "irot 90 cw",
			t:    heicTransform{irot: 3, found: true},
			want: "Rotate 90 CW",
		},
		{
			name: "missing",
			t:    heicTransform{},
			want: "",
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := heicTransformOrientation(tt.t); got != tt.want {
				t.Fatalf("heicTransformOrientation() = %q, want %q", got, tt.want)
			}
		})
	}
}

func TestParseHEICPrimaryIrotUsesPrimaryItem(t *testing.T) {
	data := buildSyntheticHEIC(heicFixture{
		primaryItemID: 1,
		primaryIrot:   0,
		thumbnailItem: &heicItemFixture{itemID: 2, irot: 3},
		decoyIrotInMDAT: true,
	})
	got := parseHEICPrimaryIrot(data)
	if !got.found || got.irot != 0 {
		t.Fatalf("parseHEICPrimaryIrot() = %+v, want irot=0 from primary item", got)
	}
}

func TestParseHEICPrimaryIrotRotate90(t *testing.T) {
	data := buildSyntheticHEIC(heicFixture{
		primaryItemID: 1,
		primaryIrot:   3,
	})
	got := parseHEICPrimaryIrot(data)
	if !got.found || got.irot != 3 {
		t.Fatalf("parseHEICPrimaryIrot() = %+v, want irot=3", got)
	}
}

func TestParseHEICPrimaryIrotMissingPITM(t *testing.T) {
	data := buildSyntheticHEIC(heicFixture{
		omitPITM:      true,
		primaryItemID: 1,
		primaryIrot:   0,
	})
	got := parseHEICPrimaryIrot(data)
	if got.found {
		t.Fatalf("parseHEICPrimaryIrot() = %+v, want not found without pitm", got)
	}
}

func TestGetOrientationSyntheticHEIC(t *testing.T) {
	path := writeTempHEIC(t, buildSyntheticHEIC(heicFixture{
		primaryItemID: 1,
		primaryIrot:   0,
		thumbnailItem: &heicItemFixture{itemID: 2, irot: 3},
	}))
	if got := GetOrientation(t.Context(), path); got != "Horizontal (normal)" {
		t.Fatalf("GetOrientation() = %q, want Horizontal (normal)", got)
	}
}

func TestGetOrientationSyntheticHEICRotate(t *testing.T) {
	path := writeTempHEIC(t, buildSyntheticHEIC(heicFixture{
		primaryItemID: 1,
		primaryIrot:   3,
	}))
	if got := GetOrientation(t.Context(), path); got != "Rotate 90 CW" {
		t.Fatalf("GetOrientation() = %q, want Rotate 90 CW", got)
	}
}

func TestFindBMFFBoxRangeRespectsDepthLimit(t *testing.T) {
	target := bmffBox("targ", []byte{1, 2, 3})
	nested := target
	for i := 0; i < maxBMFFRecursionDepth+4; i++ {
		nested = bmffBox("meta", nested)
	}
	if got := findBMFFBoxRange(nested, 0, len(nested), "targ", 0); got != nil {
		t.Fatal("expected depth limit to prevent finding deeply nested box")
	}
}

func TestParseIPCOPropertiesReadsPlainIrotBox(t *testing.T) {
	ipco := bmffBox("ipco", bmffIrotBox(2))
	props := parseIPCOProperties(ipco)
	if len(props) != 1 || props[0].irot != 2 {
		t.Fatalf("parseIPCOProperties() = %+v, want irot=2", props)
	}
}

type heicItemFixture struct {
	itemID uint16
	irot   uint8
}

type heicFixture struct {
	primaryItemID   uint16
	primaryIrot     uint8
	thumbnailItem   *heicItemFixture
	omitPITM        bool
	decoyIrotInMDAT bool
}

func writeTempHEIC(t *testing.T, data []byte) string {
	t.Helper()
	path := filepath.Join(t.TempDir(), "sample.heic")
	if err := os.WriteFile(path, data, 0o600); err != nil {
		t.Fatal(err)
	}
	return path
}

func buildSyntheticHEIC(fix heicFixture) []byte {
	var properties []byte
	propertyIndex := uint8(1)

	appendProperty := func(box []byte) uint8 {
		idx := propertyIndex
		properties = append(properties, box...)
		propertyIndex++
		return idx
	}

	primaryIspe := appendProperty(bmffFullBox("ispe", 0, []byte{0, 0, 1, 0, 0, 0, 2, 0}))
	primaryIrot := appendProperty(bmffIrotBox(fix.primaryIrot))

	var ipmaEntries []ipmaEntryFixture
	primaryAssoc := []ipmaAssocFixture{
		{index: primaryIspe, essential: true},
		{index: primaryIrot, essential: false},
	}
	ipmaEntries = append(ipmaEntries, ipmaEntryFixture{
		itemID: fix.primaryItemID,
		assoc:  primaryAssoc,
	})

	if fix.thumbnailItem != nil {
		thumbIspe := appendProperty(bmffFullBox("ispe", 0, []byte{0, 0, 0, 64, 0, 0, 0, 64}))
		thumbIrot := appendProperty(bmffIrotBox(fix.thumbnailItem.irot))
		ipmaEntries = append(ipmaEntries, ipmaEntryFixture{
			itemID: fix.thumbnailItem.itemID,
			assoc: []ipmaAssocFixture{
				{index: thumbIspe, essential: true},
				{index: thumbIrot, essential: false},
			},
		})
	}

	ipco := bmffBox("ipco", properties)
	ipma := bmffIPMABox(ipmaEntries)
	iprp := bmffBox("iprp", append(ipco, ipma...))

	var metaChildren []byte
	if !fix.omitPITM {
		metaChildren = append(metaChildren, bmffPITMBox(fix.primaryItemID)...)
	}
	metaChildren = append(metaChildren, iprp...)
	meta := bmffFullBox("meta", 0, metaChildren)

	out := []byte{}
	out = append(out, bmffFtypBox()...)
	out = append(out, meta...)
	if fix.decoyIrotInMDAT {
		decoy := make([]byte, 64)
		copy(decoy[4:8], []byte("irot"))
		decoy[8] = 3
		out = append(out, bmffBox("mdat", decoy)...)
	}
	return out
}

type ipmaAssocFixture struct {
	index     uint8
	essential bool
}

type ipmaEntryFixture struct {
	itemID uint16
	assoc  []ipmaAssocFixture
}

func bmffBox(typ string, payload []byte) []byte {
	size := 8 + len(payload)
	out := make([]byte, size)
	binary.BigEndian.PutUint32(out[0:4], uint32(size))
	copy(out[4:8], typ)
	copy(out[8:], payload)
	return out
}

func bmffFullBox(typ string, version byte, payload []byte) []byte {
	size := 12 + len(payload)
	out := make([]byte, size)
	binary.BigEndian.PutUint32(out[0:4], uint32(size))
	copy(out[4:8], typ)
	out[8] = version
	copy(out[12:], payload)
	return out
}

func bmffFtypBox() []byte {
	payload := append([]byte("heic"), 0, 0, 0, 0)
	payload = append(payload, []byte("mif1")...)
	return bmffBox("ftyp", payload)
}

func bmffPITMBox(itemID uint16) []byte {
	payload := make([]byte, 2)
	binary.BigEndian.PutUint16(payload, itemID)
	return bmffFullBox("pitm", 0, payload)
}

func bmffIrotBox(angle uint8) []byte {
	return bmffBox("irot", []byte{angle & 0x03})
}

func bmffIPMABox(entries []ipmaEntryFixture) []byte {
	var payload []byte
	payload = appendUint32BE(payload, uint32(len(entries)))
	for _, entry := range entries {
		payload = appendUint16BE(payload, entry.itemID)
		payload = append(payload, byte(len(entry.assoc)))
		for _, assoc := range entry.assoc {
			value := assoc.index & 0x7f
			if assoc.essential {
				value |= 0x80
			}
			payload = append(payload, value)
		}
	}
	return bmffFullBox("ipma", 0, payload)
}

func appendUint32BE(dst []byte, v uint32) []byte {
	var b [4]byte
	binary.BigEndian.PutUint32(b[:], v)
	return append(dst, b[:]...)
}

func appendUint16BE(dst []byte, v uint16) []byte {
	var b [2]byte
	binary.BigEndian.PutUint16(b[:], v)
	return append(dst, b[:]...)
}
