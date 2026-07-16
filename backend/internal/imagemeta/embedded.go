package imagemeta

import (
	"context"
	"io"
	"os"
	"path/filepath"
	"strings"

	extimagemeta "github.com/evanoberholster/imagemeta"
	"github.com/evanoberholster/imagemeta/meta/exif"
)

var rawImageExtensions = map[string]struct{}{
	".cr2": {}, ".cr3": {}, ".nef": {}, ".nrw": {}, ".arw": {}, ".srf": {}, ".sr2": {},
	".orf": {}, ".rw2": {}, ".raw": {}, ".dng": {}, ".raf": {}, ".pef": {}, ".ptx": {},
	".rwl": {}, ".3fr": {}, ".fff": {}, ".erf": {}, ".mrw": {}, ".dcr": {}, ".kdc": {},
	".dc2": {}, ".x3f": {}, ".iiq": {}, ".nkc": {}, ".r3d": {},
}

func isRawImageExtension(ext string) bool {
	_, ok := rawImageExtensions[strings.ToLower(ext)]
	return ok
}

func isHEICExtension(ext string) bool {
	switch strings.ToLower(ext) {
	case ".heic", ".heif", ".heics":
		return true
	default:
		return false
	}
}

// ExtractEmbeddedPreview returns embedded JPEG preview bytes when present.
// Returns nil, nil when no preview exists or the file cannot be read.
func ExtractEmbeddedPreview(ctx context.Context, path string) ([]byte, error) {
	if ctx.Err() != nil || path == "" {
		return nil, ctx.Err()
	}

	ext := strings.ToLower(filepath.Ext(path))
	if !isRawImageExtension(ext) && !isHEICExtension(ext) {
		return nil, nil
	}

	f, err := os.Open(path)
	if err != nil {
		return nil, nil
	}
	defer f.Close()

	var data []byte
	switch {
	case ext == ".cr3":
		data, err = extimagemeta.PreviewCR3(f)
	case isRawImageExtension(ext):
		data, err = extractTIFFEmbeddedPreview(f)
	case isHEICExtension(ext):
		// imagemeta only extracts ISO-BMFF previews for CR3; HEIC falls back to FFmpeg.
		return nil, nil
	}
	if err != nil || len(data) == 0 {
		return nil, nil
	}
	if !IsJPEG(data) {
		return nil, nil
	}
	return data, nil
}

func extractTIFFEmbeddedPreview(f *os.File) ([]byte, error) {
	if _, err := f.Seek(0, io.SeekStart); err != nil {
		return nil, err
	}

	ex, err := extimagemeta.Decode(f)
	if err != nil {
		return nil, err
	}

	for _, c := range previewCandidates(ex) {
		if c.length == 0 {
			continue
		}
		data, readErr := readFileRange(f, c.offset, c.length)
		if readErr != nil || len(data) == 0 {
			continue
		}
		if IsJPEG(data) {
			return data, nil
		}
	}
	return nil, nil
}

type previewCandidate struct {
	offset, length uint32
}

func previewCandidates(ex exif.Exif) []previewCandidate {
	var out []previewCandidate

	if ex.MakerNote.Canon != nil {
		info := ex.MakerNote.Canon.PreviewImageInfo
		if info.PreviewImageLength > 0 && info.PreviewImageStart > 0 {
			out = append(out, previewCandidate{
				offset: info.PreviewImageStart,
				length: info.PreviewImageLength,
			})
		}
	}

	appendIFD := func(ifd *exif.ImageIFD) {
		if ifd == nil {
			return
		}
		if ifd.ImageLength > 0 && ifd.ImageOffset > 0 {
			out = append(out, previewCandidate{offset: ifd.ImageOffset, length: ifd.ImageLength})
		}
	}
	appendIFD(ex.IFD1)
	appendIFD(ex.IFD2)

	if ex.IFD0.ThumbnailLength > 0 && ex.IFD0.ThumbnailOffset > 0 {
		out = append(out, previewCandidate{
			offset: ex.IFD0.ThumbnailOffset,
			length: ex.IFD0.ThumbnailLength,
		})
	}
	if ex.IFD0.ImageLength > 0 && ex.IFD0.ImageOffset > 0 {
		out = append(out, previewCandidate{
			offset: ex.IFD0.ImageOffset,
			length: ex.IFD0.ImageLength,
		})
	}

	return out
}

const maxPreviewReadSize = 100 * 1024 * 1024 // embedded previews are never larger

func readFileRange(f *os.File, offset, length uint32) ([]byte, error) {
	if length == 0 {
		return nil, nil
	}
	if length > maxPreviewReadSize {
		return nil, nil
	}
	data := make([]byte, length)
	n, err := f.ReadAt(data, int64(offset))
	if err != nil && err != io.EOF {
		return nil, err
	}
	return data[:n], nil
}

// IsJPEG reports whether data begins with a JPEG SOI marker.
func IsJPEG(data []byte) bool {
	return len(data) >= 2 && data[0] == 0xff && data[1] == 0xd8
}
