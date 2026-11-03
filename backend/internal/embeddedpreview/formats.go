package embeddedpreview

import (
	"archive/zip"
	"encoding/xml"
	"fmt"
	"io"
	"path"
	"regexp"
	"sort"
	"strings"
)

// maxEntryBytes caps how much of one embedded entry is read. It matches the 20 MB limit
// the text pipeline uses (adapters/fs/files), and guards against zip bombs.
const maxEntryBytes = 20 << 20

var imageExts = map[string]bool{".jpg": true, ".jpeg": true, ".png": true, ".webp": true, ".gif": true}

// pickFirstEntry returns the first named entry that exists and has data.
func pickFirstEntry(zr *zip.Reader, names ...string) (Result, error) {
	for _, name := range names {
		f := findEntry(zr, name)
		if f == nil {
			continue
		}
		data, err := readEntry(f)
		if err != nil || len(data) == 0 {
			continue
		}
		return Result{Data: data, Ext: strings.ToLower(path.Ext(f.Name))}, nil
	}
	return Result{}, ErrNotFound
}

// Krita stores a flattened composite at the root. preview.png is the fallback.
func pickKrita(zr *zip.Reader) (Result, error) {
	return pickFirstEntry(zr, "mergedimage.png", "preview.png")
}

// OpenRaster stores a merged image at the root and an optional thumbnail.
func pickORA(zr *zip.Reader) (Result, error) {
	return pickFirstEntry(zr, "mergedimage.png", "Thumbnails/thumbnail.png")
}

// Sketch stores a preview next to the document data.
func pickSketch(zr *zip.Reader) (Result, error) {
	return pickFirstEntry(zr, "previews/preview.png")
}

// MuseScore writes a thumbnail into the score container.
func pickMuseScore(zr *zip.Reader) (Result, error) {
	return pickFirstEntry(zr, "Thumbnails/thumbnail.png")
}

// OpenDocument (ODT, ODS, ODP, ODG) writes Thumbnails/thumbnail.png when the app saves one.
func pickODF(zr *zip.Reader) (Result, error) {
	return pickFirstEntry(zr, "Thumbnails/thumbnail.png")
}

// Office Open XML (DOCX, XLSX, PPTX) keeps an optional thumbnail under docProps.
func pickOOXML(zr *zip.Reader) (Result, error) {
	return pickFirstEntry(zr, "docProps/thumbnail.jpeg", "docProps/thumbnail.jpg", "docProps/thumbnail.png")
}

// CBZ cover: the first image in natural page order, ignoring macOS metadata and hidden files.
func pickCBZ(zr *zip.Reader) (Result, error) {
	var pages []*zip.File
	for _, f := range zr.File {
		if f.FileInfo().IsDir() || strings.HasPrefix(f.Name, "__MACOSX/") {
			continue
		}
		base := path.Base(f.Name)
		if strings.HasPrefix(base, ".") {
			continue
		}
		if imageExts[strings.ToLower(path.Ext(base))] {
			pages = append(pages, f)
		}
	}
	sort.SliceStable(pages, func(i, j int) bool {
		return naturalKey(pages[i].Name) < naturalKey(pages[j].Name)
	})
	for _, f := range pages {
		data, err := readEntry(f)
		if err != nil || len(data) == 0 {
			continue
		}
		return Result{Data: data, Ext: strings.ToLower(path.Ext(f.Name))}, nil
	}
	return Result{}, ErrNotFound
}

const xpsThumbnailRel = "http://schemas.microsoft.com/xps/2005/06/thumbnail"

type xpsRelationships struct {
	Relationships []struct {
		Type   string `xml:"Type,attr"`
		Target string `xml:"Target,attr"`
	} `xml:"Relationship"`
}

// XPS thumbnails are optional. The package-level relationship in _rels/.rels points at one when present.
func pickXPS(zr *zip.Reader) (Result, error) {
	rels := findEntry(zr, "_rels/.rels")
	if rels == nil {
		return Result{}, ErrNotFound
	}
	relData, err := readEntry(rels)
	if err != nil {
		return Result{}, ErrNotFound
	}
	var parsed xpsRelationships
	if err := xml.Unmarshal(relData, &parsed); err != nil {
		return Result{}, ErrNotFound
	}
	for _, rel := range parsed.Relationships {
		if rel.Type != xpsThumbnailRel {
			continue
		}
		target := strings.TrimPrefix(rel.Target, "/")
		ext := strings.ToLower(path.Ext(target))
		if ext != ".jpg" && ext != ".jpeg" && ext != ".png" {
			continue
		}
		return pickFirstEntry(zr, target)
	}
	return Result{}, ErrNotFound
}

// findEntry matches the exact name first, then case-insensitively (some writers vary the case).
func findEntry(zr *zip.Reader, name string) *zip.File {
	for _, f := range zr.File {
		if f.Name == name {
			return f
		}
	}
	for _, f := range zr.File {
		if strings.EqualFold(f.Name, name) {
			return f
		}
	}
	return nil
}

// readEntry reads one zip entry, refusing anything larger than maxEntryBytes.
func readEntry(f *zip.File) ([]byte, error) {
	if f.UncompressedSize64 > maxEntryBytes {
		return nil, fmt.Errorf("embedded entry %q is too large", f.Name)
	}
	rc, err := f.Open()
	if err != nil {
		return nil, err
	}
	defer rc.Close()
	data, err := io.ReadAll(io.LimitReader(rc, maxEntryBytes+1))
	if err != nil {
		return nil, err
	}
	if len(data) > maxEntryBytes {
		return nil, fmt.Errorf("embedded entry %q is too large", f.Name)
	}
	return data, nil
}

var digitRun = regexp.MustCompile(`\d+`)

// naturalKey makes "page2" sort before "page10" by zero-padding digit runs.
func naturalKey(name string) string {
	return digitRun.ReplaceAllStringFunc(strings.ToLower(name), func(d string) string {
		return strings.Repeat("0", 20-len(d)) + d
	})
}
