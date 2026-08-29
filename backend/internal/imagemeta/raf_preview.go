//go:build !386 && !arm

package imagemeta

import (
	"encoding/binary"
	"io"
	"os"
)

const (
	rafMagicLen       = 16
	rafMagic          = "FUJIFILMCCD-RAW"
	rafJPEGOffsetPos  = 0x54
	rafJPEGLengthPos  = 0x58
	rafHeaderMinBytes = 0x5C
)

// extractRAFEmbeddedPreview reads the camera-embedded JPEG from a Fuji RAF file.
// Offset and length are stored as big-endian uint32 at 0x54 and 0x58 in the RAF header directory.
func extractRAFEmbeddedPreview(f *os.File) ([]byte, error) {
	info, err := f.Stat()
	if err != nil {
		return nil, err
	}
	if info.Size() < rafHeaderMinBytes {
		return nil, nil
	}

	header := make([]byte, rafHeaderMinBytes)
	n, err := f.ReadAt(header, 0)
	if err != nil && err != io.EOF {
		return nil, err
	}
	if n < rafHeaderMinBytes {
		return nil, nil
	}
	if !isRAFMagic(header[:rafMagicLen]) {
		return nil, nil
	}

	offset := binary.BigEndian.Uint32(header[rafJPEGOffsetPos:rafJPEGLengthPos])
	length := binary.BigEndian.Uint32(header[rafJPEGLengthPos:rafHeaderMinBytes])
	if offset == 0 || length == 0 {
		return nil, nil
	}
	if int64(offset)+int64(length) > info.Size() {
		return nil, nil
	}

	data, err := readFileRange(f, offset, length)
	if err != nil || len(data) == 0 || !IsJPEG(data) {
		return nil, nil
	}
	return data, nil
}

func isRAFMagic(b []byte) bool {
	if len(b) < len(rafMagic) {
		return false
	}
	for i := 0; i < len(rafMagic); i++ {
		if b[i] != rafMagic[i] {
			return false
		}
	}
	return true
}
