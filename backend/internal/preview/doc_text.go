package preview

import (
	"os"
	"unicode/utf8"
)

// folioTextMaxBytes matches folio/html readsAsText, which only accepts UTF-8 when
// the first 4096 bytes form valid UTF-8 (see gen2brain/folio html/txt.go).
const folioTextMaxBytes = 4096

const maxTextPreviewRead = 10 << 20 // 10 MiB cap for text snippet input

// folioTextPath returns a path folio can open for a text document. When the file
// is longer than folioTextMaxBytes or the first folioTextMaxBytes bytes end on a
// partial UTF-8 rune, a temp file with a safe prefix is written.
func folioTextPath(realPath, tempPath string, isText bool) (path string, cleanup func(), err error) {
	cleanup = func() {}
	if !isText {
		return realPath, cleanup, nil
	}

	data, err := os.ReadFile(realPath)
	if err != nil {
		return "", cleanup, err
	}
	if len(data) > maxTextPreviewRead {
		data = data[:maxTextPreviewRead]
	}

	checkLen := len(data)
	if checkLen > folioTextMaxBytes {
		checkLen = folioTextMaxBytes
	}
	if checkLen == 0 {
		return realPath, cleanup, nil
	}
	if utf8.Valid(data[:checkLen]) {
		return realPath, cleanup, nil
	}

	snippet := utf8PrefixMax(data, folioTextMaxBytes)
	if len(snippet) == 0 {
		return realPath, cleanup, nil
	}

	if err := os.WriteFile(tempPath, snippet, 0o644); err != nil {
		return "", cleanup, err
	}
	cleanup = func() { _ = os.Remove(tempPath) }
	return tempPath, cleanup, nil
}

func utf8PrefixMax(b []byte, max int) []byte {
	if len(b) > max {
		b = b[:max]
	}
	for len(b) > 0 && !utf8.Valid(b) {
		b = b[:len(b)-1]
	}
	return b
}
