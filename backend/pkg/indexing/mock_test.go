package indexing

import (
	"encoding/json"
	"strings"
	"testing"
)

// The performance harness baselines JSON payload size and DOM text length, so
// mock listing generation must be byte-for-byte reproducible for a given shape.
func TestCreateMockDataDeterministic(t *testing.T) {
	first := CreateMockData(25, 25)
	second := CreateMockData(25, 25)

	firstJSON, err := json.Marshal(first)
	if err != nil {
		t.Fatalf("marshal first: %v", err)
	}
	secondJSON, err := json.Marshal(second)
	if err != nil {
		t.Fatalf("marshal second: %v", err)
	}

	if string(firstJSON) != string(secondJSON) {
		t.Fatalf("mock data is not deterministic:\n first=%s\nsecond=%s",
			truncate(firstJSON), truncate(secondJSON))
	}
}

func TestCreateMockDataShape(t *testing.T) {
	got := CreateMockData(7, 11)
	if len(got.Folders) != 7 {
		t.Errorf("folders = %d, want 7", len(got.Folders))
	}
	if len(got.Files) != 11 {
		t.Errorf("files = %d, want 11", len(got.Files))
	}
}

func TestCreateMockDataDifferentShapesDiffer(t *testing.T) {
	a, err := json.Marshal(CreateMockData(10, 10))
	if err != nil {
		t.Fatalf("marshal: %v", err)
	}
	b, err := json.Marshal(CreateMockData(11, 11))
	if err != nil {
		t.Fatalf("marshal: %v", err)
	}
	if string(a) == string(b) {
		t.Fatal("different shapes produced identical output")
	}
}

func TestCreateMockDataExplicitSeed(t *testing.T) {
	a, err := json.Marshal(CreateMockDataSeeded(10, 10, 12345))
	if err != nil {
		t.Fatalf("marshal: %v", err)
	}
	b, err := json.Marshal(CreateMockDataSeeded(10, 10, 12345))
	if err != nil {
		t.Fatalf("marshal: %v", err)
	}
	if string(a) != string(b) {
		t.Fatal("explicit seed did not produce identical output")
	}

	c, err := json.Marshal(CreateMockDataSeeded(10, 10, 999))
	if err != nil {
		t.Fatalf("marshal: %v", err)
	}
	if string(a) == string(c) {
		t.Fatal("different seeds produced identical output")
	}
}

func truncate(b []byte) string {
	if len(b) > 200 {
		return string(b[:200]) + "..."
	}
	return string(b)
}

// Mock rows previously all carried Type "blob", so the listing never exercised
// preview eligibility, type-based icons, or media bubbling. File types must now
// be resolved from the extension exactly as real indexing does.
func TestCreateMockDataResolvesMimeTypes(t *testing.T) {
	got := CreateMockData(400, 400)

	allowed := map[string]bool{
		"text/plain":          true,
		"audio/mpeg":          true,
		"video/quicktime":     true,
		"application/msword":  true,
		"video/mp4":           true,
		"application/x-trash": true,
		"application/zip":     true,
		"image/jpeg":          true,
	}

	seen := map[string]int{}
	if len(got.Files) == 0 {
		t.Fatal("expected files")
	}
	for _, f := range got.Files {
		if f.Type == "blob" || f.Type == "" {
			t.Fatalf("%s resolved to %q; expected a real MIME type", f.Name, f.Type)
		}
		if !allowed[f.Type] {
			t.Fatalf("%s resolved to unexpected type %q", f.Name, f.Type)
		}
		seen[f.Type]++
	}

	// All eight mock extensions should appear across 400 sampled files.
	if len(seen) != len(allowed) {
		t.Errorf("saw %d distinct types, want %d: %v", len(seen), len(allowed), seen)
	}
}

// Sizes should be plausible per extension so the listing renders mixed units
// (bytes vs MB/GB) rather than a column of near-identical tiny values.
func TestCreateMockDataRealisticSizes(t *testing.T) {
	got := CreateMockData(400, 400)

	var sawLarge, sawSmall bool
	inspect := func(name string, size int64) {
		switch {
		case strings.HasSuffix(name, ".mp4"), strings.HasSuffix(name, ".mov"):
			if size < 10_000_000 {
				t.Errorf("%s size %d unexpectedly small for video", name, size)
			}
			sawLarge = true
		case strings.HasSuffix(name, ".txt"):
			if size > 50_000 {
				t.Errorf("%s size %d unexpectedly large for text", name, size)
			}
			sawSmall = true
		}
	}
	for _, f := range got.Files {
		inspect(f.Name, int64(f.Size))
	}

	if !sawLarge {
		t.Error("expected at least one large (video) file")
	}
	if !sawSmall {
		t.Error("expected at least one small (text) file")
	}
}

// Folder rows must be real directories. The frontend decides whether a row is
// navigable from `type === "directory"`, so emitting file MIME types here made
// rows that looked like folders in the payload behave as files in the UI.
func TestCreateMockDataFolderRowsAreDirectories(t *testing.T) {
	got := CreateMockData(20, 5)
	if len(got.Folders) != 20 {
		t.Fatalf("folders = %d, want 20", len(got.Folders))
	}
	for _, f := range got.Folders {
		if f.Type != "directory" {
			t.Errorf("folder row %s got type %q, want \"directory\"", f.Name, f.Type)
		}
		if f.Size != 0 {
			t.Errorf("folder row %s got size %d, want 0", f.Name, f.Size)
		}
	}
}

// Mock rows have no file on disk, so header sniffing would degrade to "blob"
// and undo the point of typing them. Types come from the extension alone.
func TestMockMimeTypeFromExtension(t *testing.T) {
	cases := map[string]string{
		"file-a.txt": "text/plain",
		"file-a.mp3": "audio/mpeg",
		"file-a.mov": "video/quicktime",
		"file-a.doc": "application/msword",
		"file-a.mp4": "video/mp4",
		"file-a.zip": "application/zip",
		"file-a.jpg": "image/jpeg",
		// Unknown or absent extensions fall back rather than erroring.
		"file-a.unknownext": "blob",
		"file-a":            "blob",
	}
	for name, want := range cases {
		if got := mockMimeType(name); got != want {
			t.Errorf("mockMimeType(%q) = %q, want %q", name, got, want)
		}
	}
}

// The charset parameter must be stripped so mock rows match the types real
// indexing produces (mime.TypeByExtension(".txt") appends "; charset=utf-8").
func TestMockMimeTypeStripsCharset(t *testing.T) {
	if got := mockMimeType("file-a.txt"); strings.Contains(got, ";") || strings.Contains(got, "charset") {
		t.Errorf("mockMimeType returned %q; expected the charset parameter stripped", got)
	}
	// The extension is matched case-insensitively.
	if got := mockMimeType("FILE-A.JPG"); got != "image/jpeg" {
		t.Errorf("mockMimeType(\"FILE-A.JPG\") = %q, want \"image/jpeg\"", got)
	}
}
