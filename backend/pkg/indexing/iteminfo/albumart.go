package iteminfo

import (
	"bytes"
	"encoding/base64"
	"encoding/binary"
	"errors"
	"io"
	"os"
)

// Album art detection runs for every audio file during indexing, so it reads
// container headers and skips frame, block, box and comment payloads by offset
// where the format allows. Reads go through a small read-ahead window, so a
// cover that sits inside that window may be read along with the headers, and
// an unsynchronised ID3v2.3 tag is read (up to id3MaxUnsyncRead) because its
// frame headers are only valid after decoding. Images are never decoded or
// copied into a separate buffer, and every walk has a hard cap on how many
// entries it visits.
const (
	// artReadWindow is the read-ahead window used for header reads.
	artReadWindow = 4 << 10
	// id3MaxFrames caps the number of ID3v2 frames visited.
	id3MaxFrames = 4096
	// id3MaxUnsyncRead caps how much of an unsynchronised ID3v2 tag is read.
	id3MaxUnsyncRead = 256 << 10
	// flacMaxBlocks caps the number of FLAC metadata blocks visited.
	flacMaxBlocks = 256
	// mp4MaxBoxes caps the number of MP4 boxes visited across the whole walk.
	mp4MaxBoxes = 4096
	// oggMaxPages caps the number of Ogg pages visited.
	oggMaxPages = 4096
	// oggMaxHeaderPackets caps the number of Ogg header packets inspected.
	oggMaxHeaderPackets = 256
	// vorbisMaxComments caps the number of Vorbis comments inspected.
	vorbisMaxComments = 10000
)

// errArtLimit is returned when a walk reaches one of its caps.
var errArtLimit = errors.New("album art: walk limit reached")

// CouldHaveAlbumArt reports whether files with this extension are checked for
// embedded album art.
func CouldHaveAlbumArt(extension string) bool {
	return audioMetadataTypes[extension]
}

// HasAlbumArt reports whether the audio file at filePath has embedded cover
// art. MP3 (ID3v2 APIC/PIC), FLAC (PICTURE block), Ogg Vorbis, Opus, Speex and
// Ogg FLAC (METADATA_BLOCK_PICTURE, COVERART or a PICTURE block), and M4A/MP4
// (moov/udta/meta/ilst/covr, wherever moov is in the file) are supported. Other
// formats and any read or parse error report false.
func HasAlbumArt(filePath string, extension string) bool {
	if !CouldHaveAlbumArt(extension) {
		return false
	}
	file, err := os.Open(filePath)
	if err != nil {
		return false
	}
	defer file.Close()
	st, err := file.Stat()
	if err != nil {
		return false
	}
	hasArt, err := hasAlbumArtReader(file, st.Size(), extension)
	if err != nil {
		return false
	}
	return hasArt
}

// hasAlbumArtReader dispatches to the format checker for extension, reading
// from r, which holds size bytes.
func hasAlbumArtReader(r io.ReaderAt, size int64, extension string) (bool, error) {
	cr := newWindowReader(r, size)
	switch extension {
	case ".mp3":
		return id3HasPicture(cr, 0)
	case ".flac":
		return flacHasPicture(cr)
	case ".ogg", ".opus":
		return oggHasPicture(cr)
	case ".m4a", ".mp4":
		return mp4HasCover(cr)
	default:
		return false, nil
	}
}

// windowReader is an io.ReaderAt over a file of known size that serves small
// reads from a read-ahead window, so a header walk costs a few syscalls rather
// than one per header.
type windowReader struct {
	r      io.ReaderAt
	size   int64
	buf    []byte
	bufOff int64
}

// newWindowReader wraps r, which holds size bytes, with a read-ahead window.
func newWindowReader(r io.ReaderAt, size int64) *windowReader {
	return &windowReader{r: r, size: size}
}

// ReadAt implements io.ReaderAt. Reads past size return io.EOF.
func (w *windowReader) ReadAt(p []byte, off int64) (int, error) {
	if off < 0 {
		return 0, errors.New("album art: negative offset")
	}
	if off >= w.size {
		return 0, io.EOF
	}
	if len(p) >= artReadWindow {
		return w.r.ReadAt(p, off)
	}
	if off < w.bufOff || off+int64(len(p)) > w.bufOff+int64(len(w.buf)) {
		if cap(w.buf) < artReadWindow {
			w.buf = make([]byte, artReadWindow)
		}
		want := int64(artReadWindow)
		if off+want > w.size {
			want = w.size - off
		}
		n, err := w.r.ReadAt(w.buf[:want], off)
		w.buf = w.buf[:n]
		w.bufOff = off
		if n == 0 {
			if err == nil {
				err = io.ErrUnexpectedEOF
			}
			return 0, err
		}
	}
	n := copy(p, w.buf[off-w.bufOff:])
	if n < len(p) {
		return n, io.EOF
	}
	return n, nil
}

// readFullAt reads exactly len(p) bytes at off.
func readFullAt(r io.ReaderAt, p []byte, off int64) error {
	n, err := r.ReadAt(p, off)
	if n == len(p) {
		return nil
	}
	if err == nil || err == io.EOF {
		err = io.ErrUnexpectedEOF
	}
	return err
}

// synchsafe32 decodes a 28-bit ID3v2 synchsafe integer. ok is false when a
// byte has its high bit set.
func synchsafe32(b []byte) (v int64, ok bool) {
	for _, c := range b[:4] {
		if c&0x80 != 0 {
			return 0, false
		}
		v = v<<7 | int64(c)
	}
	return v, true
}

// id3TagSize returns the total size of the ID3v2 tag at off, header and footer
// included, or 0 when there is no ID3v2 tag there.
func id3TagSize(r io.ReaderAt, off int64) (int64, []byte, error) {
	hdr := make([]byte, 10)
	if err := readFullAt(r, hdr, off); err != nil {
		return 0, nil, nil
	}
	if string(hdr[:3]) != "ID3" || hdr[3] == 0xff || hdr[4] == 0xff {
		return 0, nil, nil
	}
	body, ok := synchsafe32(hdr[6:10])
	if !ok {
		return 0, nil, nil
	}
	total := 10 + body
	if hdr[3] == 4 && hdr[5]&0x10 != 0 {
		total += 10 // footer
	}
	return total, hdr, nil
}

// id3HasPicture reports whether the ID3v2 tag at off has an embedded picture
// frame (APIC in v2.3/v2.4, PIC in v2.2); frames that only link to an image
// ("-->") do not count. Only frame headers and the first bytes of a picture
// frame are read, unless the whole tag is unsynchronised, in which case at most
// id3MaxUnsyncRead bytes of the tag are read. A tag whose declared size runs
// past the end of the file is rejected.
func id3HasPicture(r *windowReader, off int64) (bool, error) {
	total, hdr, err := id3TagSize(r, off)
	if err != nil || total == 0 {
		return false, err
	}
	version, flags := hdr[3], hdr[5]
	if version < 2 || version > 4 {
		return false, nil
	}
	start := off + 10
	end := off + total
	if end > r.size {
		return false, nil
	}
	if version == 4 && flags&0x10 != 0 {
		end -= 10
	}

	if flags&0x80 != 0 && version < 4 {
		// Tag-level unsynchronisation: frame headers are only valid after
		// removing the inserted zero bytes. Only a bounded prefix is read
		// and decoded; that prefix limits how far the walk can scan, while
		// frame sizes are still checked against the declared tag size.
		bodyLen := end - start
		n := bodyLen
		if n > id3MaxUnsyncRead {
			n = id3MaxUnsyncRead
		}
		buf := make([]byte, n)
		if err := readFullAt(r, buf, start); err != nil {
			return false, nil
		}
		buf = bytes.ReplaceAll(buf, []byte{0xff, 0x00}, []byte{0xff})
		scanEnd := int64(len(buf))
		boundEnd := scanEnd
		if n < bodyLen {
			// Unsync removal shrinks the decoded prefix by n-scanEnd bytes;
			// the full decoded tag is at most that much shorter than bodyLen.
			boundEnd = bodyLen - (n - scanEnd)
		}
		mr := newWindowReader(bytes.NewReader(buf), scanEnd)
		return id3WalkFrames(mr, version, flags, 0, scanEnd, boundEnd)
	}
	return id3WalkFrames(r, version, flags, start, end, end)
}

// id3WalkFrames walks ID3v2 frames starting at start and reports whether an
// embedded picture frame is present. Frame headers are only read before
// scanEnd, and every frame must end by boundEnd. For v2.4 it retries with plain
// 32-bit frame sizes when synchsafe sizes run into a malformed frame, or when
// any frame size reads differently under the two encodings, since some taggers
// write v2.4 tags with v2.3-style sizes.
func id3WalkFrames(r io.ReaderAt, version, flags byte, start, scanEnd, boundEnd int64) (bool, error) {
	pos := start
	if flags&0x40 != 0 {
		switch version {
		case 2:
			return false, nil // v2.2 compression; no defined scheme
		case 3:
			b := make([]byte, 4)
			if err := readFullAt(r, b, pos); err != nil {
				return false, nil
			}
			pos += 4 + int64(binary.BigEndian.Uint32(b))
		case 4:
			b := make([]byte, 4)
			if err := readFullAt(r, b, pos); err != nil {
				return false, nil
			}
			n, ok := synchsafe32(b)
			if !ok || n < 6 {
				return false, nil
			}
			pos += n
		}
	}
	res := id3Walk(r, version, pos, scanEnd, boundEnd, version == 4)
	if res.found || version != 4 {
		return res.found, nil
	}
	if res.ambiguous && !res.malformed && res.paddingEnd {
		return false, nil
	}
	if res.malformed || res.ambiguous {
		return id3Walk(r, version, pos, scanEnd, boundEnd, false).found, nil
	}
	return false, nil
}

// id3WalkResult is the outcome of one ID3v2 frame walk.
type id3WalkResult struct {
	found      bool // an embedded picture frame was reached
	malformed  bool // the walk stopped at a frame that is neither valid nor padding
	ambiguous  bool // a synchsafe frame size differs from its plain 32-bit reading
	paddingEnd bool // the walk ended on tag padding that runs to boundEnd
}

// id3Walk visits frame headers from pos while they start before scanEnd.
// A frame that would end past boundEnd stops the walk as malformed.
func id3Walk(r io.ReaderAt, version byte, pos, scanEnd, boundEnd int64, synchsafeSizes bool) id3WalkResult {
	var res id3WalkResult
	idLen, hdrLen, picID := 4, int64(10), "APIC"
	if version == 2 {
		idLen, hdrLen, picID = 3, 6, "PIC"
	}
	hdr := make([]byte, hdrLen)
	for i := 0; i < id3MaxFrames && pos+hdrLen <= scanEnd; i++ {
		if err := readFullAt(r, hdr, pos); err != nil {
			return res
		}
		if hdr[0] == 0 {
			if id3RangeAllZero(r, pos, boundEnd) {
				res.paddingEnd = true
				return res // padding
			}
			res.malformed = true
			return res
		}
		if !validFrameID(hdr[:idLen]) {
			res.malformed = true
			return res
		}
		var size int64
		switch {
		case version == 2:
			size = int64(hdr[3])<<16 | int64(hdr[4])<<8 | int64(hdr[5])
		case synchsafeSizes:
			s, ok := synchsafe32(hdr[4:8])
			if !ok {
				res.malformed = true
				return res
			}
			if s != int64(binary.BigEndian.Uint32(hdr[4:8])) {
				res.ambiguous = true
			}
			size = s
		default:
			size = int64(binary.BigEndian.Uint32(hdr[4:8]))
		}
		next := pos + hdrLen + size
		if size == 0 || next > boundEnd {
			res.malformed = true
			return res
		}
		if string(hdr[:idLen]) == picID {
			payOff, paySize := id3PicturePayload(version, hdr, pos+hdrLen, size)
			if id3PictureHasEmbedded(r, version, payOff, paySize) {
				res.found = true
				return res
			}
		}
		if next > scanEnd {
			return res // the rest of the tag is past the scan budget
		}
		pos = next
	}
	return res
}

// id3RangeAllZero reports whether every byte in [pos, boundEnd) is zero.
func id3RangeAllZero(r io.ReaderAt, pos, boundEnd int64) bool {
	for pos < boundEnd {
		var chunk [256]byte
		n := boundEnd - pos
		if n > int64(len(chunk)) {
			n = int64(len(chunk))
		}
		if err := readFullAt(r, chunk[:n], pos); err != nil {
			return false
		}
		for _, c := range chunk[:n] {
			if c != 0 {
				return false
			}
		}
		pos += n
	}
	return true
}

// id3PicturePayload returns the picture field offset and size inside a frame
// body, skipping ID3v2.3/v2.4 format fields declared in hdr.
func id3PicturePayload(version byte, hdr []byte, off, size int64) (int64, int64) {
	if version == 2 {
		return off, size
	}
	payOff, paySize := off, size
	if version == 4 && hdr[9]&0x01 != 0 {
		payOff += 4
		paySize -= 4
	}
	if version >= 3 && hdr[9]&0x40 != 0 {
		payOff++
		paySize--
	}
	if paySize <= 0 {
		return off, 0
	}
	return payOff, paySize
}

// id3PictureHasEmbedded reports whether a picture frame holds image data
// rather than a URL-only APIC/PIC frame.
func id3PictureHasEmbedded(r io.ReaderAt, version byte, off, size int64) bool {
	if size < 1 {
		return false
	}
	if version == 2 {
		if size < 4 {
			return false
		}
		var b [4]byte
		if readFullAt(r, b[:4], off) != nil {
			return false
		}
		if string(b[1:4]) == "-->" {
			return false
		}
		// encoding, format (3), type, description, image
		cur := off + 4
		rem := size - 4
		if rem < 2 {
			return false
		}
		cur++
		rem--
		return id3SkipToImageV22(r, cur, rem)
	}
	// APIC: encoding (1), MIME\\0, type (1), description\\0, image
	if size < 5 {
		return false
	}
	var b [5]byte
	if readFullAt(r, b[:5], off) != nil {
		return false
	}
	if string(b[1:5]) == "-->\x00" {
		return false
	}
	return id3SkipApicToImage(r, off+1, size-1)
}

// id3SkipToImageV22 skips PIC type and description and requires image bytes.
func id3SkipToImageV22(r io.ReaderAt, off, size int64) bool {
	if size < 1 {
		return false
	}
	off++
	size--
	for size > 0 {
		var c [1]byte
		if readFullAt(r, c[:], off) != nil {
			return false
		}
		off++
		size--
		if c[0] == 0 {
			return size >= 1
		}
	}
	return false
}

// id3SkipApicToImage skips MIME, picture type, and description; image remains.
func id3SkipApicToImage(r io.ReaderAt, off, size int64) bool {
	for size > 0 {
		var c [1]byte
		if readFullAt(r, c[:], off) != nil {
			return false
		}
		off++
		size--
		if c[0] == 0 {
			break
		}
	}
	if size < 2 {
		return false
	}
	off++
	size--
	for size > 0 {
		var c [1]byte
		if readFullAt(r, c[:], off) != nil {
			return false
		}
		off++
		size--
		if c[0] == 0 {
			return size >= 1
		}
	}
	return false
}

// validFrameID reports whether id is made of A-Z and 0-9 only.
func validFrameID(id []byte) bool {
	for _, c := range id {
		if (c < 'A' || c > 'Z') && (c < '0' || c > '9') {
			return false
		}
	}
	return true
}

// flacHasPicture reports whether a FLAC file has a PICTURE metadata block, or a
// METADATA_BLOCK_PICTURE or COVERART entry in its VORBIS_COMMENT block. A
// leading ID3v2 tag is skipped. Only block headers and comment names are read.
func flacHasPicture(r *windowReader) (bool, error) {
	off, _, err := id3TagSize(r, 0)
	if err != nil {
		return false, err
	}
	magic := make([]byte, 4)
	if err := readFullAt(r, magic, off); err != nil || string(magic) != "fLaC" {
		return false, nil
	}
	pos := off + 4
	hdr := make([]byte, 4)
	for i := 0; i < flacMaxBlocks; i++ {
		if err := readFullAt(r, hdr, pos); err != nil {
			return false, nil
		}
		last := hdr[0]&0x80 != 0
		typ := hdr[0] & 0x7f
		size := int64(hdr[1])<<16 | int64(hdr[2])<<8 | int64(hdr[3])
		body := pos + 4
		if body+size > r.size {
			return false, nil
		}
		switch typ {
		case 6:
			if flacPictureHasEmbedded(r, body, size) {
				return true, nil
			}
		case 4:
			sr := &sectionPacket{r: r, off: body, end: body + size}
			if ok, err := vorbisCommentHasPicture(sr); err == nil && ok {
				return true, nil
			}
		case 127:
			return false, nil // invalid block type
		}
		if last {
			return false, nil
		}
		pos = body + size
	}
	return false, nil
}

// flacPictureMinLen is the size of a FLAC PICTURE block with empty strings and
// no image data: type, MIME length, description length, width, height, depth,
// colours and data length, 4 bytes each.
const flacPictureMinLen = 32

// flacPictureHeadLen is how much of a PICTURE block flacPictureIsLink needs.
const flacPictureHeadLen = 11

// flacPictureIsLink reports whether a FLAC PICTURE block, given by its first
// flacPictureHeadLen bytes, has MIME type "-->", meaning the block holds a URI
// instead of an embedded image.
func flacPictureIsLink(head []byte) bool {
	return len(head) >= flacPictureHeadLen && binary.BigEndian.Uint32(head[4:8]) == 3 && string(head[8:11]) == "-->"
}

// flacPictureHasEmbedded reports whether a PICTURE metadata block of size bytes
// at off holds a nonempty embedded image (not a URI-only block).
func flacPictureHasEmbedded(r io.ReaderAt, off, size int64) bool {
	if size < flacPictureMinLen {
		return false
	}
	head := make([]byte, flacPictureHeadLen)
	if readFullAt(r, head, off) != nil || flacPictureIsLink(head) {
		return false
	}
	mimeLen := int64(binary.BigEndian.Uint32(head[4:8]))
	cur := off + 8 + mimeLen
	if cur+4 > off+size {
		return false
	}
	var descLenBuf [4]byte
	if readFullAt(r, descLenBuf[:], cur) != nil {
		return false
	}
	descLen := int64(binary.BigEndian.Uint32(descLenBuf[:]))
	cur += 4 + descLen + 16 // width, height, depth, colours, data length
	if cur+4 > off+size {
		return false
	}
	var dataLenBuf [4]byte
	if readFullAt(r, dataLenBuf[:], cur) != nil {
		return false
	}
	dataLen := int64(binary.BigEndian.Uint32(dataLenBuf[:]))
	return dataLen > 0 && cur+4+dataLen <= off+size
}

// packetReader reads one logical packet sequentially and can skip ahead
// without reading the skipped bytes.
type packetReader interface {
	io.Reader
	// skip advances n bytes; it returns io.ErrUnexpectedEOF if the packet
	// is shorter.
	skip(n int64) error
}

// sectionPacket is a packetReader over the byte range [off, end) of r.
type sectionPacket struct {
	r   io.ReaderAt
	off int64
	end int64
}

// Read implements io.Reader.
func (s *sectionPacket) Read(p []byte) (int, error) {
	if s.off >= s.end {
		return 0, io.EOF
	}
	if rem := s.end - s.off; int64(len(p)) > rem {
		p = p[:rem]
	}
	n, err := s.r.ReadAt(p, s.off)
	s.off += int64(n)
	if n > 0 {
		err = nil
	}
	return n, err
}

// skip implements packetReader.
func (s *sectionPacket) skip(n int64) error {
	if n > s.end-s.off {
		s.off = s.end
		return io.ErrUnexpectedEOF
	}
	s.off += n
	return nil
}

// boundedPacket limits how many bytes may be read or skipped from an inner
// packetReader (used for FLAC metadata blocks inside Ogg packets).
type boundedPacket struct {
	pr  packetReader
	max int64
	rem int64
}

func newBoundedPacket(pr packetReader, max int64) *boundedPacket {
	return &boundedPacket{pr: pr, max: max, rem: max}
}

func (b *boundedPacket) Read(p []byte) (int, error) {
	if b.rem <= 0 {
		return 0, io.EOF
	}
	if int64(len(p)) > b.rem {
		p = p[:b.rem]
	}
	n, err := b.pr.Read(p)
	b.rem -= int64(n)
	if b.rem <= 0 && (err == nil || err == io.EOF) {
		return n, io.EOF
	}
	return n, err
}

func (b *boundedPacket) skip(n int64) error {
	if n > b.rem {
		b.rem = 0
		return io.ErrUnexpectedEOF
	}
	if err := b.pr.skip(n); err != nil {
		return err
	}
	b.rem -= n
	return nil
}

// vorbisPictureFields are the comment names that carry embedded cover art,
// compared case-insensitively as the Vorbis comment spec requires.
var vorbisPictureFields = [][]byte{
	[]byte("METADATA_BLOCK_PICTURE="),
	[]byte("COVERART="),
	[]byte("COVER_ART="),
}

// vorbisCommentPrefixLen is how much of each Vorbis comment is read: enough
// for the longest picture field name plus a base64 prefix that covers a
// minimal METADATA_BLOCK_PICTURE block header.
const vorbisCommentPrefixLen = 72

// vorbisCommentHasPicture parses a Vorbis comment structure (vendor string,
// count, length-prefixed comments) and reports whether any comment is a
// non-empty cover art field. METADATA_BLOCK_PICTURE values that only link to
// an image ("-->") do not count, and a matching comment must be complete.
// Only the first vorbisCommentPrefixLen bytes of each comment are read; the
// rest is skipped.
func vorbisCommentHasPicture(pr packetReader) (bool, error) {
	var n4 [4]byte
	if _, err := io.ReadFull(pr, n4[:]); err != nil {
		return false, err
	}
	if err := pr.skip(int64(binary.LittleEndian.Uint32(n4[:]))); err != nil {
		return false, err
	}
	if _, err := io.ReadFull(pr, n4[:]); err != nil {
		return false, err
	}
	count := binary.LittleEndian.Uint32(n4[:])
	if count > vorbisMaxComments {
		count = vorbisMaxComments
	}
	prefix := make([]byte, vorbisCommentPrefixLen)
	for i := uint32(0); i < count; i++ {
		if _, err := io.ReadFull(pr, n4[:]); err != nil {
			return false, err
		}
		length := int64(binary.LittleEndian.Uint32(n4[:]))
		want := length
		if want > int64(len(prefix)) {
			want = int64(len(prefix))
		}
		if _, err := io.ReadFull(pr, prefix[:want]); err != nil {
			return false, err
		}
		match := vorbisPictureComment(prefix[:want], length)
		// Skipping the rest also checks that the comment is complete.
		if err := pr.skip(length - want); err != nil {
			return false, err
		}
		if match {
			return true, nil
		}
	}
	return false, nil
}

// vorbisPictureComment reports whether a comment of total size length, whose
// first bytes are prefix, is a non-empty cover art field that embeds an image.
func vorbisPictureComment(prefix []byte, length int64) bool {
	for _, field := range vorbisPictureFields {
		if length <= int64(len(field)) || len(prefix) < len(field) || !bytes.EqualFold(prefix[:len(field)], field) {
			continue
		}
		if len(field) == len("METADATA_BLOCK_PICTURE=") {
			valueLen := length - int64(len(field))
			minB64 := base64.StdEncoding.EncodedLen(flacPictureMinLen)
			if valueLen < int64(minB64) {
				continue
			}
			value := prefix[len(field):]
			read := len(value)
			if read > minB64 {
				read = minB64
			}
			read -= read % 4
			if read < 16 {
				continue
			}
			block := make([]byte, base64.StdEncoding.DecodedLen(read))
			if _, err := base64.StdEncoding.Decode(block, value[:read]); err != nil {
				continue
			}
			if len(block) < flacPictureMinLen {
				continue
			}
			if flacPictureIsLink(block[:flacPictureHeadLen]) {
				continue
			}
			dataLen := binary.BigEndian.Uint32(block[28:32])
			if dataLen == 0 {
				continue
			}
		}
		return true
	}
	return false
}

// oggHasPicture reports whether the first logical stream of an Ogg file
// carries cover art: a METADATA_BLOCK_PICTURE or COVERART comment for Vorbis,
// Opus and Speex, or a PICTURE block (or such a comment) for Ogg FLAC. Only
// page headers, packet prefixes and comment names are read.
func oggHasPicture(r *windowReader) (bool, error) {
	op, err := newOggPacketReader(r)
	if err != nil {
		return false, nil
	}
	ident := make([]byte, 8)
	n, _ := io.ReadFull(op, ident)
	ident = ident[:n]
	switch {
	case bytes.HasPrefix(ident, []byte("\x01vorbis")):
		if !op.nextPacket() {
			return false, nil
		}
		if !packetHasPrefix(op, "\x03vorbis") {
			return false, nil
		}
		ok, _ := vorbisCommentHasPicture(op)
		return ok, nil
	case bytes.HasPrefix(ident, []byte("OpusHead")):
		if !op.nextPacket() || !packetHasPrefix(op, "OpusTags") {
			return false, nil
		}
		ok, _ := vorbisCommentHasPicture(op)
		return ok, nil
	case bytes.HasPrefix(ident, []byte("Speex   ")):
		if !op.nextPacket() {
			return false, nil
		}
		ok, _ := vorbisCommentHasPicture(op)
		return ok, nil
	case bytes.HasPrefix(ident, []byte("\x7fFLAC")):
		// Each following header packet is one FLAC metadata block.
		hdr := make([]byte, 4)
		for i := 0; i < oggMaxHeaderPackets && op.nextPacket(); i++ {
			if _, err := io.ReadFull(op, hdr); err != nil {
				return false, nil
			}
			typ := hdr[0] & 0x7f
			switch typ {
			case 6:
				size := int64(hdr[1])<<16 | int64(hdr[2])<<8 | int64(hdr[3])
				blockOff := op.segOff + op.segRead
				if size >= flacPictureMinLen && flacPictureHasEmbedded(op.r, blockOff, size) {
					if op.skip(size) == nil {
						return true, nil
					}
				} else if size > 0 {
					_ = op.skip(size)
				}
			case 4:
				size := int64(hdr[1])<<16 | int64(hdr[2])<<8 | int64(hdr[3])
				bp := newBoundedPacket(op, size)
				if ok, err := vorbisCommentHasPicture(bp); err == nil && ok {
					return true, nil
				}
			}
			if hdr[0]&0x80 != 0 {
				return false, nil
			}
		}
		return false, nil
	default:
		return false, nil
	}
}

// packetHasPrefix reads len(prefix) bytes from pr and reports whether they
// equal prefix.
func packetHasPrefix(pr io.Reader, prefix string) bool {
	b := make([]byte, len(prefix))
	if _, err := io.ReadFull(pr, b); err != nil {
		return false
	}
	return string(b) == prefix
}

// oggPacketReader reads the packets of the first logical stream of an Ogg
// file one at a time. Payload bytes that are skipped are never read; only page
// headers and lacing tables are.
type oggPacketReader struct {
	r      *windowReader
	serial uint32
	pages  int

	nextPage int64  // file offset of the next page header
	pageSeq  uint32 // sequence number of the current page
	lacing   []byte // lacing values of the current page
	seg      int    // index of the current segment in lacing
	segOff   int64  // file offset of the current segment's data
	segRead  int64  // bytes already consumed from the current segment
	ended    bool   // the current packet has ended
	inPacket bool   // false before the first byte of a new packet is read
}

// newOggPacketReader positions a reader at the first packet of the first page.
func newOggPacketReader(r *windowReader) (*oggPacketReader, error) {
	op := &oggPacketReader{r: r}
	serial, err := op.loadPage(0, true)
	if err != nil {
		return nil, err
	}
	op.serial = serial
	return op, nil
}

// loadPage reads the page header at off. When first is false, pages of other
// logical streams are skipped until one with op.serial is found.
func (op *oggPacketReader) loadPage(off int64, first bool) (uint32, error) {
	hdr := make([]byte, 27)
	for {
		op.pages++
		if op.pages > oggMaxPages {
			return 0, errArtLimit
		}
		if err := readFullAt(op.r, hdr, off); err != nil {
			return 0, err
		}
		if string(hdr[:4]) != "OggS" || hdr[4] != 0 {
			return 0, errors.New("album art: bad ogg page")
		}
		serial := binary.LittleEndian.Uint32(hdr[14:18])
		nseg := int(hdr[26])
		lacing := make([]byte, nseg)
		if err := readFullAt(op.r, lacing, off+27); err != nil {
			return 0, err
		}
		dataOff := off + 27 + int64(nseg)
		var dataLen int64
		for _, l := range lacing {
			dataLen += int64(l)
		}
		if dataOff+dataLen > op.r.size {
			// The lacing claims more data than the file holds.
			return 0, io.ErrUnexpectedEOF
		}
		if !first && serial != op.serial {
			off = dataOff + dataLen
			continue
		}
		pageSeq := binary.LittleEndian.Uint32(hdr[18:22])
		continued := hdr[5]&0x01 != 0
		if !first && op.inPacket && continued {
			if pageSeq != op.pageSeq+1 {
				return 0, errors.New("album art: broken ogg packet")
			}
		}
		op.lacing = lacing
		op.seg = 0
		op.segOff = dataOff
		op.segRead = 0
		op.nextPage = dataOff + dataLen
		op.pageSeq = pageSeq
		return serial, nil
	}
}

// advanceSegment moves to the next segment of the current packet, loading the
// next page when needed. It returns false when no data is left.
func (op *oggPacketReader) advanceSegment() bool {
	op.segOff += int64(op.lacing[op.seg])
	op.seg++
	op.segRead = 0
	for op.seg >= len(op.lacing) {
		if _, err := op.loadPage(op.nextPage, false); err != nil {
			return false
		}
	}
	return true
}

// Read implements io.Reader over the current packet and returns io.EOF at
// its end.
func (op *oggPacketReader) Read(p []byte) (int, error) {
	op.inPacket = true
	total := 0
	for len(p) > 0 && !op.ended {
		if op.seg >= len(op.lacing) {
			return total, io.ErrUnexpectedEOF
		}
		segLen := int64(op.lacing[op.seg])
		rem := segLen - op.segRead
		if rem > 0 {
			k := int64(len(p))
			if k > rem {
				k = rem
			}
			if err := readFullAt(op.r, p[:k], op.segOff+op.segRead); err != nil {
				return total, err
			}
			op.segRead += k
			total += int(k)
			p = p[k:]
			continue
		}
		if segLen < 255 {
			op.ended = true
			break
		}
		if !op.advanceSegment() {
			return total, io.ErrUnexpectedEOF
		}
	}
	if total == 0 && op.ended {
		return 0, io.EOF
	}
	return total, nil
}

// skip implements packetReader.
func (op *oggPacketReader) skip(n int64) error {
	op.inPacket = true
	for n > 0 {
		if op.ended || op.seg >= len(op.lacing) {
			return io.ErrUnexpectedEOF
		}
		segLen := int64(op.lacing[op.seg])
		rem := segLen - op.segRead
		if rem >= n {
			op.segRead += n
			if segLen < 255 && op.segRead >= segLen {
				op.ended = true
			}
			return nil
		}
		n -= rem
		op.segRead = segLen
		if segLen < 255 {
			op.ended = true
			if n > 0 {
				return io.ErrUnexpectedEOF
			}
			return nil
		}
		if !op.advanceSegment() {
			return io.ErrUnexpectedEOF
		}
	}
	return nil
}

// nextPacket skips the rest of the current packet and positions the reader at
// the start of the next one. It returns false when there is none.
func (op *oggPacketReader) nextPacket() bool {
	for !op.ended {
		if op.seg >= len(op.lacing) {
			return false
		}
		segLen := int64(op.lacing[op.seg])
		op.segRead = segLen
		if segLen < 255 {
			op.ended = true
			break
		}
		if !op.advanceSegment() {
			return false
		}
	}
	// Move past the terminating segment of the packet that just ended.
	op.inPacket = false
	if op.seg >= len(op.lacing) || !op.advanceSegment() {
		return false
	}
	op.ended = false
	op.inPacket = false
	return true
}

// errMP4BoxBounds is returned when a box's declared size runs past its parent
// or the end of the file.
var errMP4BoxBounds = errors.New("album art: mp4 box extends past its parent")

// mp4Box is one ISO BMFF box header.
type mp4Box struct {
	typ     [4]byte
	payload int64 // file offset of the payload
	end     int64 // file offset just past the box
}

// mp4Walker iterates over boxes and counts them against mp4MaxBoxes.
type mp4Walker struct {
	r     *windowReader
	boxes int
	hdr   [16]byte
}

// children calls fn for each box in [start, end) until fn returns true or an
// error, and reports whether fn returned true.
func (w *mp4Walker) children(start, end int64, fn func(b mp4Box) (bool, error)) (bool, error) {
	pos := start
	for pos+8 <= end {
		w.boxes++
		if w.boxes > mp4MaxBoxes {
			return false, errArtLimit
		}
		if err := readFullAt(w.r, w.hdr[:8], pos); err != nil {
			return false, err
		}
		size := int64(binary.BigEndian.Uint32(w.hdr[:4]))
		hdrLen := int64(8)
		switch size {
		case 0:
			size = end - pos // box runs to the end of its parent
		case 1:
			if err := readFullAt(w.r, w.hdr[8:16], pos+8); err != nil {
				return false, err
			}
			large := binary.BigEndian.Uint64(w.hdr[8:16])
			if large > uint64(end-pos) {
				return false, errMP4BoxBounds
			}
			size = int64(large)
			hdrLen = 16
		}
		if size < hdrLen {
			return false, errors.New("album art: bad mp4 box size")
		}
		if size > end-pos {
			// Truncated file or corrupt size: the box does not fit in
			// its parent, so nothing inside it can be trusted.
			return false, errMP4BoxBounds
		}
		var b mp4Box
		copy(b.typ[:], w.hdr[4:8])
		b.payload = pos + hdrLen
		b.end = pos + size
		ok, err := fn(b)
		if ok || err != nil {
			return ok, err
		}
		pos = b.end
	}
	return false, nil
}

// mp4HasCover reports whether an MP4/M4A file has an iTunes-style covr item
// with image data (moov/udta/meta/ilst/covr/data, or moov/meta/ilst/covr/data).
// The walk seeks over mdat and every other box it does not need, so the result
// does not depend on whether moov is before or after the media data.
func mp4HasCover(r *windowReader) (bool, error) {
	w := &mp4Walker{r: r}
	first := true
	return w.children(0, r.size, func(b mp4Box) (bool, error) {
		if first {
			first = false
			if !printableFourCC(b.typ) {
				return false, errors.New("album art: not an mp4 file")
			}
		}
		if string(b.typ[:]) != "moov" {
			return false, nil
		}
		return w.children(b.payload, b.end, func(c mp4Box) (bool, error) {
			switch string(c.typ[:]) {
			case "udta":
				return w.children(c.payload, c.end, func(d mp4Box) (bool, error) {
					if string(d.typ[:]) != "meta" {
						return false, nil
					}
					return w.metaHasCover(d)
				})
			case "meta":
				return w.metaHasCover(c)
			}
			return false, nil
		})
	})
}

// metaHasCover looks for ilst/covr/data inside a meta box. meta is a full box
// (4 bytes of version and flags before its children) in ISO files and a plain
// container in QuickTime files; both are handled, whatever the child order.
func (w *mp4Walker) metaHasCover(meta mp4Box) (bool, error) {
	start := meta.payload
	if !w.plausibleChild(start, meta.end) {
		start += 4 // version and flags
	}
	return w.children(start, meta.end, func(il mp4Box) (bool, error) {
		if string(il.typ[:]) != "ilst" {
			return false, nil
		}
		return w.children(il.payload, il.end, func(item mp4Box) (bool, error) {
			if string(item.typ[:]) != "covr" {
				return false, nil
			}
			return w.children(item.payload, item.end, func(d mp4Box) (bool, error) {
				// data payload: 4 bytes type indicator, 4 bytes locale, image.
				return string(d.typ[:]) == "data" && d.end-d.payload > 8, nil
			})
		})
	})
}

// plausibleChild reports whether a box header at pos fits in [pos, end) and has
// a printable type. An ISO meta box starts with version and flags (normally
// zero), which never read as a valid header, so this tells the meta layouts
// apart without assuming which child comes first.
func (w *mp4Walker) plausibleChild(pos, end int64) bool {
	var h [16]byte
	if pos+8 > end || readFullAt(w.r, h[:8], pos) != nil {
		return false
	}
	size := int64(binary.BigEndian.Uint32(h[:4]))
	hdrLen := int64(8)
	switch size {
	case 0:
		size = end - pos
	case 1:
		if pos+16 > end || readFullAt(w.r, h[8:16], pos+8) != nil {
			return false
		}
		large := binary.BigEndian.Uint64(h[8:16])
		if large > uint64(end-pos) {
			return false
		}
		size = int64(large)
		hdrLen = 16
	}
	if size < hdrLen || size > end-pos {
		return false
	}
	var typ [4]byte
	copy(typ[:], h[4:8])
	return printableFourCC(typ)
}

// printableFourCC reports whether t is four printable ASCII bytes.
func printableFourCC(t [4]byte) bool {
	for _, c := range t {
		if c < 0x20 || c > 0x7e {
			return false
		}
	}
	return true
}
