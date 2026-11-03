package embeddedpreview

import (
	"archive/zip"
	"bytes"
	"context"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/stretchr/testify/require"
)

// writeZip builds a zip container with the given entries and returns its path.
func writeZip(t *testing.T, name string, entries map[string][]byte) string {
	t.Helper()
	var buf bytes.Buffer
	zw := zip.NewWriter(&buf)
	for entryName, data := range entries {
		w, err := zw.Create(entryName)
		require.NoError(t, err)
		_, err = w.Write(data)
		require.NoError(t, err)
	}
	require.NoError(t, zw.Close())
	p := filepath.Join(t.TempDir(), name)
	require.NoError(t, os.WriteFile(p, buf.Bytes(), 0o644))
	return p
}

func extract(t *testing.T, name, path string) (Result, error) {
	t.Helper()
	return Extract(context.Background(), name, "application/octet-stream", path)
}

func TestKrita_mergedImageThenPreview(t *testing.T) {
	p := writeZip(t, "art.kra", map[string][]byte{
		"mergedimage.png": []byte("merged"),
		"preview.png":     []byte("preview"),
	})
	res, err := extract(t, "art.kra", p)
	require.NoError(t, err)
	require.Equal(t, []byte("merged"), res.Data)
	require.Equal(t, ".png", res.Ext)
	require.False(t, res.Oriented)

	p = writeZip(t, "fallback.kra", map[string][]byte{"preview.png": []byte("preview")})
	res, err = extract(t, "fallback.kra", p)
	require.NoError(t, err)
	require.Equal(t, []byte("preview"), res.Data)
}

func TestKrita_missingImage(t *testing.T) {
	p := writeZip(t, "art.kra", map[string][]byte{"maindoc.xml": []byte("<doc/>")})
	_, err := extract(t, "art.kra", p)
	require.ErrorIs(t, err, ErrNotFound)
}

func TestCBZ_firstPageInNaturalOrder(t *testing.T) {
	p := writeZip(t, "comic.cbz", map[string][]byte{
		"page10.jpg":           []byte("ten"),
		"page2.jpg":            []byte("two"),
		"__MACOSX/._page1.jpg": []byte("junk"),
		".hidden.png":          []byte("hidden"),
		"notes.txt":            []byte("text"),
	})
	res, err := extract(t, "comic.cbz", p)
	require.NoError(t, err)
	require.Equal(t, []byte("two"), res.Data)
	require.Equal(t, ".jpg", res.Ext)
}

func TestCBZ_noImages(t *testing.T) {
	p := writeZip(t, "comic.cbz", map[string][]byte{"readme.txt": []byte("x")})
	_, err := extract(t, "comic.cbz", p)
	require.ErrorIs(t, err, ErrNotFound)
}

const xpsRels = `<?xml version="1.0" encoding="utf-8"?>
<Relationships xmlns="http://schemas.openxmlformats.org/package/2006/relationships">
<Relationship Id="rId1" Type="http://schemas.microsoft.com/xps/2005/06/thumbnail" Target="/Metadata/Thumbnail.jpeg"/>
<Relationship Id="rId2" Type="http://schemas.microsoft.com/xps/2005/06/fixedrepresentation" Target="/FixedDocSeq.fdseq"/>
</Relationships>`

func TestXPS_thumbnailRelationship(t *testing.T) {
	p := writeZip(t, "doc.xps", map[string][]byte{
		"_rels/.rels":             []byte(xpsRels),
		"Metadata/Thumbnail.jpeg": []byte("thumb"),
	})
	res, err := extract(t, "doc.xps", p)
	require.NoError(t, err)
	require.Equal(t, []byte("thumb"), res.Data)
	require.Equal(t, ".jpeg", res.Ext)
}

func TestXPS_noThumbnail(t *testing.T) {
	rels := `<Relationships><Relationship Type="http://schemas.microsoft.com/xps/2005/06/fixedrepresentation" Target="/FixedDocSeq.fdseq"/></Relationships>`
	p := writeZip(t, "doc.xps", map[string][]byte{"_rels/.rels": []byte(rels)})
	_, err := extract(t, "doc.xps", p)
	require.ErrorIs(t, err, ErrNotFound)
}

func TestOpenDocument_thumbnail(t *testing.T) {
	p := writeZip(t, "letter.odt", map[string][]byte{"Thumbnails/thumbnail.png": []byte("odf")})
	res, err := extract(t, "letter.odt", p)
	require.NoError(t, err)
	require.Equal(t, []byte("odf"), res.Data)
	require.Equal(t, ".png", res.Ext)
}

func TestOOXML_thumbnailVariants(t *testing.T) {
	for _, entry := range []string{"docProps/thumbnail.jpeg", "docProps/thumbnail.png"} {
		p := writeZip(t, "deck.pptx", map[string][]byte{entry: []byte("thumb")})
		res, err := extract(t, "deck.pptx", p)
		require.NoError(t, err, entry)
		require.Equal(t, []byte("thumb"), res.Data)
	}
	p := writeZip(t, "sheet.xlsx", map[string][]byte{"xl/workbook.xml": []byte("<x/>")})
	_, err := extract(t, "sheet.xlsx", p)
	require.ErrorIs(t, err, ErrNotFound)
}

func TestORA_mergedImage(t *testing.T) {
	p := writeZip(t, "paint.ora", map[string][]byte{"mergedimage.png": []byte("ora")})
	res, err := extract(t, "paint.ora", p)
	require.NoError(t, err)
	require.Equal(t, []byte("ora"), res.Data)
}

func TestSketch_preview(t *testing.T) {
	p := writeZip(t, "ui.sketch", map[string][]byte{"previews/preview.png": []byte("sketch")})
	res, err := extract(t, "ui.sketch", p)
	require.NoError(t, err)
	require.Equal(t, []byte("sketch"), res.Data)
}

func TestMuseScore_thumbnail(t *testing.T) {
	p := writeZip(t, "score.mscz", map[string][]byte{"Thumbnails/thumbnail.png": []byte("score")})
	res, err := extract(t, "score.mscz", p)
	require.NoError(t, err)
	require.Equal(t, []byte("score"), res.Data)
}

func TestExtract_notAZip(t *testing.T) {
	p := filepath.Join(t.TempDir(), "broken.cbz")
	require.NoError(t, os.WriteFile(p, []byte("not a zip"), 0o644))
	_, err := extract(t, "broken.cbz", p)
	require.Error(t, err)
}

func TestExtract_unsupportedFormat(t *testing.T) {
	_, err := extract(t, "notes.txt", "/nonexistent")
	require.ErrorIs(t, err, ErrNotFound)
}

func TestSupports(t *testing.T) {
	require.True(t, Supports("art.kra", "application/octet-stream"))
	require.True(t, Supports("letter.ODT", ""))
	require.True(t, Supports("photo.cr2", "image/x-canon-cr2"))
	require.True(t, Supports("photo.heic", "image/heic"))
	require.False(t, Supports("photo.jpg", "image/jpeg"))
	require.False(t, Supports("movie.mp4", "video/mp4"))
}

func TestNaturalKey_ordersNumbers(t *testing.T) {
	require.Less(t, naturalKey("page2.jpg"), naturalKey("page10.jpg"))
	require.Less(t, naturalKey("page002.jpg"), naturalKey("page010.jpg"))
}

// Digit runs longer than any fixed padding width must not panic and must still sort numerically.
func TestNaturalKey_longDigitRuns(t *testing.T) {
	long := strings.Repeat("9", 40)
	require.NotPanics(t, func() { naturalKey("p" + long + ".jpg") })
	require.Less(t, naturalKey("page10.jpg"), naturalKey("page"+long+".jpg"))
	require.Less(t, naturalKey("page"+long+".jpg"), naturalKey("page1"+long+".jpg"))
}

func TestCBZ_longPageNumbersDoNotPanic(t *testing.T) {
	long := strings.Repeat("1", 25)
	p := writeZip(t, "comic.cbz", map[string][]byte{
		"page" + long + ".jpg": []byte("long"),
		"page2.jpg":            []byte("two"),
	})
	res, err := extract(t, "comic.cbz", p)
	require.NoError(t, err)
	require.Equal(t, []byte("two"), res.Data)
}
