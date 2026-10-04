package archiveencoding

import (
	"archive/zip"
	"encoding/binary"
	"golang.org/x/text/encoding/japanese"
	"hash/crc32"
	"reflect"
	"testing"
)

func entry(name string) *zip.File { return &zip.File{FileHeader: zip.FileHeader{Name: name}} }
func cp932(t *testing.T, name string) string {
	t.Helper()
	s, err := japanese.ShiftJIS.NewEncoder().String(name)
	if err != nil {
		t.Fatal(err)
	}
	return s
}
func TestJapaneseSuggestionMatchesDecoding(t *testing.T) {
	t.Parallel()
	want := []string{"日本語/テストソ001.txt", "日本語/テストソ002.txt"}
	files := []*zip.File{entry(cp932(t, want[0])), entry(cp932(t, want[1]))}
	preview := Inspect(files)
	if preview.Suggested != "cp932" {
		t.Fatalf("suggestion: %+v", preview)
	}
	names, err := DecodeNames(files, preview.Suggested)
	if err != nil || !reflect.DeepEqual(names, want) {
		t.Fatalf("names=%q err=%v", names, err)
	}
	for _, c := range preview.Candidates {
		if c.Encoding == preview.Suggested && !reflect.DeepEqual(c.Names, names) {
			t.Fatalf("preview mismatch: %q", c.Names)
		}
	}
}
func TestAmbiguousNamesRequireSelection(t *testing.T) {
	t.Parallel()
	files := []*zip.File{entry("caf\x82.txt")}
	if p := Inspect(files); p.Suggested != "" {
		t.Fatalf("ambiguous name was guessed: %+v", p)
	}
	if _, err := DecodeNames(files, ""); err == nil {
		t.Fatal("expected explicit choice")
	}
	names, err := DecodeNames(files, "cp437")
	if err != nil || names[0] != "café.txt" {
		t.Fatalf("names=%q err=%v", names, err)
	}
	for _, c := range []string{"utf-8", "unknown"} {
		if _, err = DecodeNames(files, c); err == nil {
			t.Fatalf("invalid encoding accepted: %s", c)
		}
	}
}
func unicodeExtra(raw, name string) []byte {
	payload := append([]byte{1, 0, 0, 0, 0}, []byte(name)...)
	binary.LittleEndian.PutUint32(payload[1:], crc32.ChecksumIEEE([]byte(raw)))
	extra := []byte{0x75, 0x70, 0, 0}
	binary.LittleEndian.PutUint16(extra[2:], uint16(len(payload)))
	return append(extra, payload...)
}
func TestUnicodeMetadataTakesPrecedence(t *testing.T) {
	t.Parallel()
	flagged := entry("日本語.txt")
	flagged.Flags = 0x800
	extra := entry("legacy.txt")
	extra.Extra = unicodeExtra(extra.Name, "別の名前.txt")
	names, err := DecodeNames([]*zip.File{flagged, extra}, "cp437")
	if err != nil || !reflect.DeepEqual(names, []string{"日本語.txt", "別の名前.txt"}) {
		t.Fatalf("names=%q err=%v", names, err)
	}
	extra.Extra[5] ^= 1
	names, err = DecodeNames([]*zip.File{extra}, "cp437")
	if err != nil || names[0] != "legacy.txt" {
		t.Fatalf("names=%q err=%v", names, err)
	}
}
func TestInvalidUnicodeMetadata(t *testing.T) {
	t.Parallel()
	for _, raw := range []string{"bad\xff.txt", "bad\x00.txt"} {
		f := entry(raw)
		f.Flags = 0x800
		if _, err := DecodeNames([]*zip.File{f}, "cp932"); err == nil {
			t.Fatalf("invalid declared UTF-8 accepted: %q", raw)
		}
	}
	f := entry("ok.txt")
	f.Extra = []byte{0x75, 0x70, 0xff, 0xff}
	if _, err := DecodeNames([]*zip.File{f}, "utf-8"); err != nil {
		t.Fatal(err)
	}
}
func TestUTF8AndASCIIArchives(t *testing.T) {
	t.Parallel()
	for _, files := range [][]*zip.File{nil, {entry("readme.txt")}, {entry("日本語.txt")}} {
		if p := Inspect(files); p.Suggested != "utf-8" {
			t.Fatalf("expected UTF-8: %+v", p)
		}
	}
}
func TestPreviewBoundAndAllNamesValidated(t *testing.T) {
	t.Parallel()
	files := make([]*zip.File, 20)
	for i := range files {
		files[i] = entry("readme.txt")
	}
	files = append(files, entry("invalid\xff.txt"))
	for _, c := range Inspect(files).Candidates {
		if len(c.Names) > 5 {
			t.Fatal("unbounded preview")
		}
		if c.Encoding == "utf-8" {
			t.Fatal("entry beyond preview was not validated")
		}
	}
}
