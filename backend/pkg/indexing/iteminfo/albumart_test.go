package iteminfo

import (
	"bytes"
	"encoding/base64"
	"encoding/binary"
	"image"
	"image/color"
	"image/png"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"

	"github.com/gtsteffaniak/go-media/pkg/media"
)

// fakeJPEG is a stand-in cover image; detection never decodes it.
var fakeJPEG = append([]byte{0xff, 0xd8, 0xff, 0xe0}, bytes.Repeat([]byte{0x42}, 600)...)

// writeTestCoverPNG writes a tiny PNG cover with the standard library so tests
// do not depend on FFmpeg's PNG encoder.
func writeTestCoverPNG(t *testing.T, path string) []byte {
	t.Helper()
	img := image.NewRGBA(image.Rect(0, 0, 64, 64))
	for y := 0; y < 64; y++ {
		for x := 0; x < 64; x++ {
			img.Set(x, y, color.RGBA{R: uint8(x * 4), G: uint8(y * 4), B: 128, A: 255})
		}
	}
	var buf bytes.Buffer
	if err := png.Encode(&buf, img); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(path, buf.Bytes(), 0o644); err != nil {
		t.Fatal(err)
	}
	return buf.Bytes()
}

// box builds an MP4 box.
func box(typ string, payload ...[]byte) []byte {
	body := bytes.Join(payload, nil)
	out := make([]byte, 8, 8+len(body))
	binary.BigEndian.PutUint32(out, uint32(8+len(body)))
	copy(out[4:], typ)
	return append(out, body...)
}

// boxLarge builds an MP4 box with a 64-bit extended size field (size=1).
func boxLarge(typ string, payload ...[]byte) []byte {
	body := bytes.Join(payload, nil)
	total := uint64(16 + len(body))
	out := make([]byte, 16, 16+len(body))
	binary.BigEndian.PutUint32(out, 1)
	copy(out[4:], typ)
	binary.BigEndian.PutUint64(out[8:], total)
	return append(out, body...)
}

// metaLayout selects how m4aFixture lays out the meta box.
type metaLayout int

const (
	metaISO           metaLayout = iota // full box: version/flags, hdlr, ilst
	metaQuickTime                       // plain container: hdlr, ilst
	metaQuickTimeFree                   // plain container: free, hdlr, ilst
	metaISOFree                         // full box: version/flags, free, hdlr, ilst
	metaQuickTimeExtHdlr                // plain container: extended-size hdlr, ilst
)

// m4aFixture builds an M4A file with an mdat of mdatSize bytes. moovFirst puts
// moov before mdat (faststart); withCover adds an ilst/covr item; layout picks
// the meta box format.
func m4aFixture(mdatSize int, moovFirst, withCover bool, layout metaLayout) []byte {
	ftyp := box("ftyp", []byte("M4A \x00\x00\x02\x00M4A mp42isom"))
	items := [][]byte{box("\xa9too", box("data", []byte{0, 0, 0, 1, 0, 0, 0, 0}, []byte("Lavf")))}
	if withCover {
		items = append(items, box("covr", box("data", []byte{0, 0, 0, 13, 0, 0, 0, 0}, fakeJPEG)))
	}
	hdlr := box("hdlr", make([]byte, 8), []byte("mdirappl"), make([]byte, 9))
	ilst := box("ilst", items...)
	free := box("free", make([]byte, 16))
	var meta []byte
	switch layout {
	case metaQuickTime:
		meta = box("meta", hdlr, ilst)
	case metaQuickTimeExtHdlr:
		meta = box("meta", boxLarge("hdlr", make([]byte, 8), []byte("mdirappl"), make([]byte, 9)), ilst)
	case metaQuickTimeFree:
		meta = box("meta", free, hdlr, ilst)
	case metaISOFree:
		meta = box("meta", []byte{0, 0, 0, 0}, free, hdlr, ilst)
	default:
		meta = box("meta", []byte{0, 0, 0, 0}, hdlr, ilst)
	}
	moov := box("moov", box("mvhd", make([]byte, 100)), box("trak", box("tkhd", make([]byte, 84))), box("udta", meta))
	// Media data that contains the signatures the old 12 KB scan matched on.
	pattern := []byte("BM\xff\xd8\xff\x00\x00\x00\x1cftyp")
	mdat := box("mdat", bytes.Repeat(pattern, mdatSize/len(pattern)+1)[:mdatSize])
	if moovFirst {
		return bytes.Join([][]byte{ftyp, moov, mdat}, nil)
	}
	return bytes.Join([][]byte{ftyp, mdat, moov}, nil)
}

// id3Frame builds an ID3v2.3 (plain size) or v2.4 (synchsafe size) frame.
func id3Frame(version byte, id string, body []byte) []byte {
	h := make([]byte, 10)
	copy(h, id)
	n := uint32(len(body))
	if version == 4 {
		h[4], h[5], h[6], h[7] = byte(n>>21&0x7f), byte(n>>14&0x7f), byte(n>>7&0x7f), byte(n&0x7f)
	} else {
		binary.BigEndian.PutUint32(h[4:], n)
	}
	return append(h, body...)
}

// id3Tag builds an ID3v2 tag around frames plus padding, followed by audio.
// Flag 0x80 unsynchronises the whole tag body.
func id3Tag(version, flags byte, frames ...[]byte) []byte {
	body := append(bytes.Join(frames, nil), make([]byte, 64)...)
	if flags&0x80 != 0 {
		body = unsync(body)
	}
	n := uint32(len(body))
	h := []byte{'I', 'D', '3', version, 0, flags, byte(n >> 21 & 0x7f), byte(n >> 14 & 0x7f), byte(n >> 7 & 0x7f), byte(n & 0x7f)}
	return append(append(h, body...), 0xff, 0xfb, 0x90, 0x00)
}

// flacBlock builds a FLAC metadata block.
func flacBlock(typ byte, last bool, body []byte) []byte {
	if last {
		typ |= 0x80
	}
	n := len(body)
	return append([]byte{typ, byte(n >> 16), byte(n >> 8), byte(n)}, body...)
}

// vorbisComment builds a Vorbis comment body (no codec prefix).
func vorbisComment(comments ...string) []byte {
	var b bytes.Buffer
	_ = binary.Write(&b, binary.LittleEndian, uint32(4))
	b.WriteString("test")
	_ = binary.Write(&b, binary.LittleEndian, uint32(len(comments)))
	for _, c := range comments {
		_ = binary.Write(&b, binary.LittleEndian, uint32(len(c)))
		b.WriteString(c)
	}
	return b.Bytes()
}

// oggStream lays packets out in Ogg pages of at most maxSegs lacing values,
// so long packets span pages. CRCs are not filled in; detection ignores them.
func oggStream(maxSegs int, packets ...[]byte) []byte {
	var lacing []byte
	var data []byte
	for _, p := range packets {
		n := len(p)
		for n >= 255 {
			lacing = append(lacing, 255)
			n -= 255
		}
		lacing = append(lacing, byte(n))
		data = append(data, p...)
	}
	var out []byte
	seq := uint32(0)
	continued := false
	for len(lacing) > 0 {
		k := maxSegs
		if k > len(lacing) {
			k = len(lacing)
		}
		h := make([]byte, 27)
		copy(h, "OggS")
		if seq == 0 {
			h[5] = 0x02 // beginning of stream
		}
		if continued {
			h[5] |= 0x01 // first packet continues from the previous page
		}
		binary.LittleEndian.PutUint32(h[14:], 0x1234)
		binary.LittleEndian.PutUint32(h[18:], seq)
		h[26] = byte(k)
		dl := 0
		for _, l := range lacing[:k] {
			dl += int(l)
		}
		out = append(out, h...)
		out = append(out, lacing[:k]...)
		out = append(out, data[:dl]...)
		continued = lacing[k-1] == 255
		lacing, data = lacing[k:], data[dl:]
		seq++
	}
	return out
}

// flacPicture builds a FLAC PICTURE block body with the given MIME type and
// data; MIME type "-->" makes data a URI instead of an image.
func flacPicture(mime string, data []byte) []byte {
	var b bytes.Buffer
	_ = binary.Write(&b, binary.BigEndian, uint32(3))
	_ = binary.Write(&b, binary.BigEndian, uint32(len(mime)))
	b.WriteString(mime)
	_ = binary.Write(&b, binary.BigEndian, [6]uint32{0, 1, 1, 24, 0, uint32(len(data))})
	b.Write(data)
	return b.Bytes()
}

// apicFrame builds an ID3v2.3 APIC frame body with the given MIME type;
// MIME type "-->" makes data a URL instead of an image.
func apicFrame(mime string, data []byte) []byte {
	body := append([]byte{0}, mime...)
	body = append(body, 0, 3, 0) // MIME terminator, front cover, empty description
	return append(body, data...)
}

// unsync applies ID3v2 unsynchronisation to b.
func unsync(b []byte) []byte {
	return bytes.ReplaceAll(b, []byte{0xff}, []byte{0xff, 0x00})
}

// pictureComment returns a METADATA_BLOCK_PICTURE comment wrapping fakeJPEG.
func pictureComment(name string) string {
	return name + "=" + base64.StdEncoding.EncodeToString(flacPicture("image/jpeg", fakeJPEG))
}

// checkBytes runs detection on data as if it were a file with extension ext.
func checkBytes(t *testing.T, data []byte, ext string) bool {
	t.Helper()
	got, err := hasAlbumArtReader(bytes.NewReader(data), int64(len(data)), ext)
	if err != nil {
		return false
	}
	return got
}

// TestHasAlbumArtSynthetic covers each format with hand-built files, including
// layouts the previous fixed-size reads missed or misread.
func TestHasAlbumArtSynthetic(t *testing.T) {
	bigLyrics := id3Frame(3, "USLT", bytes.Repeat([]byte("la "), 40000)) // 120 KB before APIC
	vorbisIdent := append([]byte("\x01vorbis"), make([]byte, 22)...)
	opusHead := append([]byte("OpusHead"), make([]byte, 11)...)
	id3Prefix := id3Tag(3, 0, id3Frame(3, "TIT2", []byte("\x00t")))
	id3Prefix = id3Prefix[:len(id3Prefix)-4] // drop the fake MP3 frame header
	bigCover := bytes.Repeat(fakeJPEG, 500)  // ~300 KB, full of 0xff bytes
	linkAPIC := id3Frame(3, "APIC", apicFrame("-->", []byte("http://example.com/c.jpg")))
	// A v2.4 PRIV frame written with a plain size of 256; read as synchsafe the
	// size is 128, which lands on zero bytes that look like padding.
	privPlain := id3Frame(3, "PRIV", make([]byte, 256))
	truncatedCover := m4aFixture(4096, true, true, metaISO)
	truncatedCover = truncatedCover[:bytes.Index(truncatedCover, []byte("covr"))+4+16+20]
	oggArt := oggStream(255, append([]byte("\x01vorbis"), make([]byte, 22)...), append([]byte("\x03vorbis"), vorbisComment("TITLE=t", pictureComment("METADATA_BLOCK_PICTURE"))...))
	// VORBIS_COMMENT whose single comment declares 100 bytes but the block holds 32.
	shortComment := vorbisComment("x")[:12]
	shortComment = binary.LittleEndian.AppendUint32(shortComment, 100)
	shortComment = append(shortComment, "COVERART=AAAAAAAAAAAAAAAAAAAAAAA"...)
	longComment := "LYRICS=" + string(bytes.Repeat([]byte("x"), 70000))

	tests := []struct {
		name string
		ext  string
		data []byte
		want bool
	}{
		{"m4a cover, moov after 2MB mdat", ".m4a", m4aFixture(2<<20, false, true, metaISO), true},
		{"m4a cover, faststart", ".m4a", m4aFixture(2<<20, true, true, metaISO), true},
		{"m4a cover, quicktime meta", ".m4a", m4aFixture(4096, false, true, metaQuickTime), true},
		{"m4a no cover, signatures in mdat", ".m4a", m4aFixture(64<<10, true, false, metaISO), false},
		{"m4a no cover, moov at end", ".m4a", m4aFixture(64<<10, false, false, metaISO), false},
		{"mp4 extension", ".mp4", m4aFixture(4096, false, true, metaISO), true},
		{"m4a truncated before moov", ".m4a", m4aFixture(64<<10, false, true, metaISO)[:40<<10], false},
		{"not mp4", ".m4a", []byte("\x00\x01\x02\x03\x04\x05\x06\x07\x08\x09garbage"), false},

		{"mp3 v2.3 APIC", ".mp3", id3Tag(3, 0, id3Frame(3, "TIT2", []byte("\x00t")), id3Frame(3, "APIC", apicFrame("image/jpeg", fakeJPEG))), true},
		{"mp3 v2.3 APIC after 120KB frame", ".mp3", id3Tag(3, 0, bigLyrics, id3Frame(3, "APIC", apicFrame("image/jpeg", fakeJPEG))), true},
		{"mp3 v2.4 synchsafe APIC after large frame", ".mp3", id3Tag(4, 0, id3Frame(4, "TXXX", make([]byte, 300)), id3Frame(4, "APIC", apicFrame("image/jpeg", fakeJPEG))), true},
		{"mp3 v2.4 with v2.3-style sizes", ".mp3", id3Tag(4, 0, id3Frame(3, "TXXX", bytes.Repeat([]byte{1}, 200)), id3Frame(3, "APIC", apicFrame("image/jpeg", fakeJPEG))), true},
		{"mp3 v2.2 PIC", ".mp3", id3Tag(2, 0, append([]byte("TT2\x00\x00\x02\x00t"), append([]byte("PIC\x00\x02\x58"), append([]byte{0}, append([]byte("JPG"), append([]byte{0, 3, 0}, fakeJPEG...)...)...)...)...)), true},
		{"mp3 no APIC", ".mp3", id3Tag(3, 0, id3Frame(3, "TIT2", []byte("\x00APIC in text"))), false},
		{"mp3 no ID3", ".mp3", []byte{0xff, 0xfb, 0x90, 0x00, 0, 0, 0, 0, 0, 0, 0, 0}, false},

		{"flac PICTURE after 100KB padding", ".flac", append([]byte("fLaC"), bytes.Join([][]byte{flacBlock(0, false, make([]byte, 34)), flacBlock(1, false, make([]byte, 100000)), flacBlock(6, true, flacPicture("image/jpeg", fakeJPEG))}, nil)...), true},
		{"flac picture comment", ".flac", append([]byte("fLaC"), bytes.Join([][]byte{flacBlock(0, false, make([]byte, 34)), flacBlock(4, true, vorbisComment("TITLE=t", pictureComment("metadata_block_picture")))}, nil)...), true},
		{"flac no picture", ".flac", append([]byte("fLaC"), bytes.Join([][]byte{flacBlock(0, false, make([]byte, 34)), flacBlock(4, true, vorbisComment("TITLE=METADATA_BLOCK_PICTURE="))}, nil)...), false},
		{"flac behind ID3v2", ".flac", append(id3Prefix, append([]byte("fLaC"), flacBlock(6, true, flacPicture("image/jpeg", fakeJPEG))...)...), true},

		{"ogg vorbis picture", ".ogg", oggStream(255, vorbisIdent, append([]byte("\x03vorbis"), vorbisComment("TITLE=t", pictureComment("METADATA_BLOCK_PICTURE"))...), []byte("\x05vorbis")), true},
		{"ogg vorbis picture after 70KB comment spanning pages", ".ogg", oggStream(40, vorbisIdent, append([]byte("\x03vorbis"), vorbisComment(longComment, pictureComment("METADATA_BLOCK_PICTURE"))...)), true},
		{"ogg vorbis legacy COVERART", ".ogg", oggStream(255, vorbisIdent, append([]byte("\x03vorbis"), vorbisComment("coverart=AAAA")...)), true},
		{"ogg vorbis no picture", ".ogg", oggStream(255, vorbisIdent, append([]byte("\x03vorbis"), vorbisComment("TITLE=METADATA_BLOCK_PICTURE=x")...)), false},
		{"opus picture", ".opus", oggStream(255, opusHead, append([]byte("OpusTags"), vorbisComment("ARTIST=a", pictureComment("METADATA_BLOCK_PICTURE"))...)), true},
		{"opus no picture", ".opus", oggStream(255, opusHead, append([]byte("OpusTags"), vorbisComment("ARTIST=a")...)), false},
		{"ogg flac PICTURE packet", ".ogg", oggStream(255, append([]byte("\x7fFLAC\x01\x00\x00\x02fLaC"), flacBlock(0, false, make([]byte, 34))...), flacBlock(4, false, vorbisComment("TITLE=t")), flacBlock(6, true, flacPicture("image/jpeg", fakeJPEG))), true},
		{"not ogg", ".ogg", []byte("RIFF....WAVEfmt "), false},

		{"m4a quicktime meta with leading free", ".m4a", m4aFixture(4096, false, true, metaQuickTimeFree), true},
		{"m4a quicktime meta with extended-size hdlr", ".m4a", m4aFixture(4096, false, true, metaQuickTimeExtHdlr), true},
		{"m4a iso meta with leading free", ".m4a", m4aFixture(4096, false, true, metaISOFree), true},
		{"m4a cover data truncated", ".m4a", truncatedCover, false},
		{"mp3 unsynchronised v2.3 with 300KB APIC first", ".mp3", id3Tag(3, 0x80, id3Frame(3, "APIC", apicFrame("image/jpeg", bigCover)), id3Frame(3, "TIT2", []byte("\x00t"))), true},
		{"mp3 unsynchronised v2.3 truncated tag", ".mp3", id3Tag(3, 0x80, id3Frame(3, "APIC", apicFrame("image/jpeg", bigCover)))[:100<<10], false},
		{"mp3 v2.4 plain sizes with zero-filled payload", ".mp3", id3Tag(4, 0, privPlain, id3Frame(3, "APIC", apicFrame("image/jpeg", fakeJPEG))), true},
		{"mp3 APIC link only", ".mp3", id3Tag(3, 0, linkAPIC), false},
		{"mp3 APIC link then image", ".mp3", id3Tag(3, 0, linkAPIC, id3Frame(3, "APIC", apicFrame("image/jpeg", fakeJPEG))), true},
		{"mp3 v2.2 PIC link only", ".mp3", id3Tag(2, 0, append([]byte("PIC\x00\x00\x0c"), "\x00-->\x03\x00http://x"...)), false},
		{"flac PICTURE link only", ".flac", append([]byte("fLaC"), flacBlock(6, true, flacPicture("-->", []byte("http://example.com/c.jpg")))...), false},
		{"flac PICTURE link then image", ".flac", append([]byte("fLaC"), bytes.Join([][]byte{flacBlock(6, false, flacPicture("-->", []byte("http://x"))), flacBlock(6, true, flacPicture("image/jpeg", fakeJPEG))}, nil)...), true},
		{"flac picture comment link only", ".flac", append([]byte("fLaC"), flacBlock(4, true, vorbisComment("METADATA_BLOCK_PICTURE="+base64.StdEncoding.EncodeToString(flacPicture("-->", []byte("http://example.com/c.jpg")))))...), false},
		{"flac truncated COVERART comment", ".flac", append([]byte("fLaC"), flacBlock(4, true, shortComment)...), false},
		{"ogg flac PICTURE link only", ".ogg", oggStream(255, append([]byte("\x7fFLAC\x01\x00\x00\x01fLaC"), flacBlock(0, false, make([]byte, 34))...), flacBlock(6, true, flacPicture("-->", []byte("http://x")))), false},
		{"ogg truncated inside picture comment", ".ogg", oggArt[:len(oggArt)-300], false},

		{"unsupported extension", ".wav", []byte("RIFF....WAVEfmt "), false},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := checkBytes(t, tt.data, tt.ext); got != tt.want {
				t.Fatalf("hasAlbumArtReader(%s) = %v, want %v", tt.ext, got, tt.want)
			}
		})
	}
}

// TestHasAlbumArtTruncated makes sure every prefix of a valid file is handled
// without panics or hangs.
func TestHasAlbumArtTruncated(t *testing.T) {
	files := map[string][]byte{
		".m4a":  m4aFixture(512, false, true, metaISO),
		".mp3":  id3Tag(4, 0, id3Frame(4, "TIT2", []byte("\x00t")), id3Frame(4, "APIC", apicFrame("image/jpeg", fakeJPEG))),
		".flac": append([]byte("fLaC"), flacBlock(6, true, flacPicture("image/jpeg", fakeJPEG))...),
		".ogg":  oggStream(3, append([]byte("\x01vorbis"), make([]byte, 22)...), append([]byte("\x03vorbis"), vorbisComment("TITLE=t", pictureComment("METADATA_BLOCK_PICTURE"))...)),
	}
	for ext, data := range files {
		for n := 0; n < len(data); n++ {
			checkBytes(t, data[:n], ext)
		}
	}
}

// TestHasAlbumArtFFmpeg checks files written by ffmpeg, and that the result
// matches go-media's HasArtwork, which parses the full tag.
func TestHasAlbumArtFFmpeg(t *testing.T) {
	if testing.Short() {
		t.Skip("starts ffmpeg")
	}
	if _, err := exec.LookPath("ffmpeg"); err != nil {
		t.Skip("ffmpeg not installed")
	}
	if _, err := exec.LookPath("ffprobe"); err != nil {
		t.Skip("ffprobe not installed")
	}
	dir := t.TempDir()
	cover := filepath.Join(dir, "cover.png")
	coverData := writeTestCoverPNG(t, cover)
	var pic bytes.Buffer
	_ = binary.Write(&pic, binary.BigEndian, uint32(3))
	_ = binary.Write(&pic, binary.BigEndian, uint32(9))
	pic.WriteString("image/png")
	_ = binary.Write(&pic, binary.BigEndian, [6]uint32{0, 64, 64, 24, 0, uint32(len(coverData))})
	pic.Write(coverData)
	picture := "METADATA_BLOCK_PICTURE=" + base64.StdEncoding.EncodeToString(pic.Bytes())

	encOut, err := exec.Command("ffmpeg", "-hide_banner", "-encoders").CombinedOutput()
	if err != nil {
		t.Skipf("ffmpeg -encoders: %v\n%s", err, encOut)
	}
	encoders := map[string]bool{}
	for _, line := range strings.Split(string(encOut), "\n") {
		if f := strings.Fields(line); len(f) >= 2 && len(f[0]) == 6 {
			encoders[f[1]] = true
		}
	}

	src := []string{"-f", "lavfi", "-i", "sine=d=20"}
	withCover := append(append([]string{}, src...), "-i", cover, "-map", "0", "-map", "1", "-c:v", "copy", "-disposition:v", "attached_pic")
	cases := []struct {
		name string
		args []string
		want bool
	}{
		{"art_moov_end.m4a", append(append([]string{}, withCover...), "-c:a", "aac"), true},
		{"art_faststart.m4a", append(append([]string{}, withCover...), "-c:a", "aac", "-movflags", "+faststart"), true},
		{"art_mp4brand.m4a", append(append([]string{}, withCover...), "-c:a", "aac", "-f", "mp4"), true},
		{"plain.m4a", append(append([]string{}, src...), "-c:a", "aac"), false},
		{"plain_mp4brand.m4a", append(append([]string{}, src...), "-c:a", "aac", "-f", "mp4"), false},
		{"art_v23.mp3", append(append([]string{}, withCover...), "-c:a", "libmp3lame", "-id3v2_version", "3"), true},
		{"art_v24.mp3", append(append([]string{}, withCover...), "-c:a", "libmp3lame", "-id3v2_version", "4"), true},
		{"plain.mp3", append(append([]string{}, src...), "-c:a", "libmp3lame"), false},
		{"art.flac", append(append([]string{}, withCover...), "-c:a", "flac"), true},
		{"plain.flac", append(append([]string{}, src...), "-c:a", "flac"), false},
		{"art.ogg", append(append([]string{}, src...), "-c:a", "libvorbis", "-metadata", picture), true},
		{"plain.ogg", append(append([]string{}, src...), "-c:a", "libvorbis"), false},
		{"art.opus", append(append([]string{}, src...), "-c:a", "libopus", "-metadata", picture), true},
		{"plain.opus", append(append([]string{}, src...), "-c:a", "libopus"), false},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			for i, a := range tc.args {
				if a == "-c:a" && i+1 < len(tc.args) && !encoders[tc.args[i+1]] {
					t.Skipf("ffmpeg has no %s encoder", tc.args[i+1])
				}
			}
			path := filepath.Join(dir, tc.name)
			out, err := exec.Command("ffmpeg", append(append([]string{"-v", "error", "-y"}, tc.args...), path)...).CombinedOutput()
			if err != nil {
				t.Skipf("ffmpeg could not write %s: %v\n%s", tc.name, err, out)
			}
			if got := HasAlbumArt(path, filepath.Ext(path)); got != tc.want {
				t.Fatalf("HasAlbumArt(%s) = %v, want %v", tc.name, got, tc.want)
			}
			f, err := os.Open(path)
			if err != nil {
				t.Fatal(err)
			}
			defer f.Close()
			st, err := f.Stat()
			if err != nil {
				t.Fatal(err)
			}
			if gm := media.HasArtwork(f, st.Size()); gm != tc.want {
				t.Fatalf("go-media HasArtwork(%s) = %v, want %v", tc.name, gm, tc.want)
			}
		})
	}
}
