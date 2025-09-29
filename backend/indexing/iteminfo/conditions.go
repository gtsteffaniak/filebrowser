package iteminfo

import (
	"bytes"
	"fmt"
	"io"
	"mime"
	"net/http"
	"os"
	"path/filepath"
	"slices"
	"strconv"
	"strings"

	"github.com/gtsteffaniak/filebrowser/backend/common/settings"
)

var AllFiletypeOptions = []string{
	"image",
	"audio",
	"archive",
	"video",
	"doc",
	"text",
}

var SubtitleExts = []string{
	".vtt",
	".srt",
	".lrc",
	".sbv",
	".ass",
	".ssa",
	".sub",
	".smi",
}

var MuPdfConvertable = map[string]bool{
	".pdf":  true, // PDF
	".xps":  true, // XPS
	".epub": true, // EPUB
	".mobi": true, // MOBI
	".fb2":  true, // FB2
	".cbz":  true, // CBZ
	".svg":  true, // SVG
	".txt":  true, // TXT
	".docx": true, // DOCX
	".pptx": true, // PPTX
	".xlsx": true, // XLSX
	".hwp":  true, // HWP
	".hwpx": true, // HWPX
	".md":   true, // Markdown
}

var ResizableImageTypes = map[string]bool{
	".jpg":   true,
	".jpeg":  true,
	".png":   true,
	".gif":   true,
	".bmp":   true,
	".other": false,
}

// Known bundle-style extensions that are technically directories but treated as files
var BundleExtensions = []string{
	".app",       // macOS application bundle
	".bundle",    // macOS plugin bundle
	".framework", // macOS framework
	".plugin",    // macOS plugin
	".kext",      // macOS kernel extension
	".pkg",       // macOS installer package
	".mpkg",      // macOS multi-package
	".apk",       // Android package
	".aab",       // Android App Bundle
	".appx",      // Windows application package
	".msix",      // Windows modern app package
	".deb",       // Debian package
	".snap",      // Snap package
	".flatpak",   // Flatpak application
	".dmg",       // macOS disk image
	".iso",       // ISO disk image
}

// Document file extensions
var documentTypes = []string{
	// Common Document Formats
	".doc", ".docx", // Microsoft Word
	".pdf", // Portable Document Format
	".odt", // OpenDocument Text
	".rtf", // Rich Text Format
	".conf",
	".bash_history",
	".gitignore",
	".htpasswd",
	".profile",
	".dockerignore",
	".editorconfig",

	// Presentation Formats
	".ppt", ".pptx", // Microsoft PowerPoint
	".odp", // OpenDocument Presentation

	// google docs
	".gdoc",

	// google sheet
	".gsheet",

	// Spreadsheet Formats
	".xls", ".xlsx", // Microsoft Excel
	".ods", // OpenDocument Spreadsheet

	// Other Document Formats
	".epub", // Electronic Publication
	".mobi", // Amazon Kindle
	".fb2",  // FictionBook
}

var onlyOfficeSupported = []string{
	// Word Processing Documents
	".doc", ".docm", ".docx", ".dot", ".dotm", ".dotx", ".epub",
	".fb2", ".fodt", ".htm", ".html", ".mht", ".mhtml", ".odt",
	".ott", ".rtf", ".stw", ".sxw", ".txt", ".wps", ".wpt", ".xml",
	".hwp", ".hwpx", ".md", ".pages", // Added missing Word extensions

	// Spreadsheet Documents
	".csv", ".et", ".ett", ".fods", ".ods", ".ots", ".sxc", ".xls",
	".xlsb", ".xlsm", ".xlsx", ".xlt", ".xltm", ".xltx",
	".numbers", // Added missing Spreadsheet extension

	// Presentation Documents
	".dps", ".dpt", ".fodp", ".odp", ".otp", ".pot", ".potm", ".potx",
	".pps", ".ppsm", ".ppsx", ".ppt", ".pptm", ".pptx", ".sxi",
	".key", ".odg", // Added missing Presentation extensions

	// Other Office-Related Formats
	".djvu", ".docxf", ".oform", ".oxps", ".pdf", ".xps",

	// Diagram Documents (New category from List 2)
	".vsdm", ".vsdx", ".vssm", ".vssx", ".vstm", ".vstx",
}

// Text-based file extensions
var textTypes = []string{
	// Common Text Formats
	".txt",
	".md", // Markdown

	// Scripting and Programming Languages
	".sh",        // Bash script
	".py",        // Python
	".js",        // JavaScript
	".ts",        // TypeScript
	".php",       // PHP
	".rb",        // Ruby
	".go",        // Go
	".java",      // Java
	".c", ".cpp", // C/C++
	".cs",    // C#
	".swift", // Swift

	// Configuration Files
	".yaml", ".yml", // YAML
	".json", // JSON
	".xml",  // XML
	".ini",  // INI
	".toml", // TOML
	".cfg",  // Configuration file

	// Other Text-Based Formats
	".css",          // Cascading Style Sheets
	".html", ".htm", // HyperText Markup Language
	".sql", // SQL
	".csv", // Comma-Separated Values
	".tsv", // Tab-Separated Values
	".log", // Log file
	".bat", // Batch file
	".ps1", // PowerShell script
	".tex", // LaTeX
	".bib", // BibTeX
}

// Compressed file extensions
var compressedFile = []string{
	".7z",
	".rar",
	".zip",
	".tar",
	".gz",
	".xz",
	".bz2",
	".tgz",  // tar.gz
	".tbz2", // tar.bz2
	".lzma",
	".lz4",
	".zstd",
}

var audioMetadataTypes = []string{
	".mp3",
	".flac",
	".ogg",
	".opus",
	".m4a",
	".mp4",
	".wav",
	".ape",
	".wv",
}

func CouldHaveAlbumArt(extension string) bool {
	for _, typefile := range audioMetadataTypes {
		if extension == typefile {
			return true
		}
	}
	return false
}

func ExtendedMimeTypeCheck(extension string) string {
	if IsDoc(extension) {
		return "application/document"
	}
	if IsText(extension) {
		return "text/plain"
	}
	return "blob"
}

func ToInt(str string) int {
	val, err := strconv.Atoi(str)
	if err != nil {
		return 0
	}
	return val
}

func UpdateSize(given string) int {
	size := ToInt(given)
	if size == 0 {
		return 100
	} else {
		return size
	}
}

func IsText(extension string) bool {
	for _, typefile := range textTypes {
		if extension == typefile {
			return true
		}
	}
	return false
}

func IsDoc(extension string) bool {
	for _, typefile := range documentTypes {
		if extension == typefile {
			return true
		}
	}
	return false
}

func IsArchive(extension string) bool {
	for _, typefile := range compressedFile {
		if extension == typefile {
			return true
		}
	}
	return false
}

func IsOnlyOffice(name string) bool {
	extention := filepath.Ext(name)
	for _, typefile := range onlyOfficeSupported {
		if extention == typefile {
			return true
		}
	}
	return false
}

func IsMatchingType(extension string, matchType string) bool {
	mimetype := mime.TypeByExtension(extension)
	if strings.HasPrefix(mimetype, matchType) {
		return true
	}
	switch matchType {
	case "doc":
		return IsDoc(extension)
	case "text":
		return IsText(extension)
	case "archive":
		return IsArchive(extension)
	}
	return false
}

// DetectType detects the MIME type of a file and updates the ItemInfo struct.
func (i *ItemInfo) DetectType(realPath string, saveContent bool) {
	name := i.Name
	ext := strings.ToLower(filepath.Ext(name))

	// Attempt MIME detection by file extension
	switch ext {
	case ".md":
		i.Type = "text/markdown"
		return
	case ".heic", ".heif":
		i.Type = "image/heic"
		return
	}
	i.Type = strings.Split(mime.TypeByExtension(ext), ";")[0]

	if i.Type == "" {
		i.Type = ExtendedMimeTypeCheck(ext)
	}
	// do header detection for certain files to ensure the type is correct for undetected or ambiguous files
	if !settings.Config.Server.DisableTypeDetectionByHeader {
		switch ext {
		case ".ts", ".xcf":
			i.Type = DetectTypeByHeader(realPath)
			return
		}
		if i.Type == "blob" || i.Type == "" {
			i.Type = DetectTypeByHeader(realPath)
		}
	}
	if i.Type == "" || i.Type == "application/octet-stream" {
		i.Type = "blob"
	}
}

// hasAlbumArtLowLevel efficiently checks for album art in audio files.
// It handles ID3v2 tags (MP3) and Vorbis comments (FLAC, OGG) with low-level parsing.
func hasAlbumArtLowLevel(filePath string, extension string) (bool, error) {
	file, err := os.Open(filePath)
	if err != nil {
		return false, fmt.Errorf("failed to open file: %w", err)
	}
	defer file.Close()

	// Same buffer for all the audio formats, will read the first 64KB.
    buffer := make([]byte, 65536) // 64KB
    n, err := file.Read(buffer)
    if err != nil && err != io.EOF {
        return false, err
    }

    data := buffer[:n]

    hasAlbumArtM4A := hasAlbumArtM4A(data)

	// Handle different audio formats
	switch extension {
	case ".mp3":
		return hasAlbumArtMP3(data)
	case ".flac", ".ogg", ".opus":
		return hasAlbumArtVorbis(data), nil
	case ".m4a", ".mp4":
		// For M4A check signature or covr atom
		return hasAlbumArtM4A || bytes.Contains(data, []byte("covr")), nil
	default:
		// For other formats, return false (no album art detection)
		return false, nil
	}
}

// hasAlbumArtMP3 checks for APIC/PIC frames in ID3v2 tags with optimized parsing
func hasAlbumArtMP3(data []byte) (bool, error) {
	// Read 10-byte ID3v2 header
	if len(data) < 10 {
        return false, nil
    }

	// Check for "ID3" identifier
	if string(data[0:3]) != "ID3" {
        return false, nil
    }

	// Determine ID3v2 version for proper frame ID detection
	version := data[3]
	var pictureFrameID string
	var frameHeaderSize int

	switch version {
	case 2:
		pictureFrameID = "PIC" // ID3v2.2 uses 3-byte frame IDs
		frameHeaderSize = 6
	case 3, 4:
		pictureFrameID = "APIC" // ID3v2.3/2.4 use 4-byte frame IDs
		frameHeaderSize = 10
	default:
		return false, nil // Unsupported version
	}

	// Decode synchsafe tag size
	tagSize := (int(data[6]) << 21) | (int(data[7]) << 14) | (int(data[8]) << 7) | int(data[9])

	// Limit reading to reasonable size for performance
	readSize := tagSize
	if readSize > len(data)-10 {
		readSize = len(data) - 10
	}

	// Read tag data
	tagData := data[10 : 10+readSize]

	position := 0
	frameIDLen := len(pictureFrameID)

	// Loop through frames looking for picture frame
	for position+frameHeaderSize <= len(tagData) {
		// Check if we've hit padding (null bytes)
		if tagData[position] == 0 {
			break
		}

		// Extract frame ID
		if position+frameIDLen > len(tagData) {
			break
		}
		frameID := string(tagData[position : position+frameIDLen])

		// Found picture frame!
		if frameID == pictureFrameID {
			return true, nil
		}

		// Skip to next frame
		var frameSize int
		if version == 2 {
			// ID3v2.2: 3-byte size
			if position+6 > len(tagData) {
				break
			}
			frameSize = int(tagData[position+3])<<16 | int(tagData[position+4])<<8 | int(tagData[position+5])
			position += 6 + frameSize
		} else {
			// ID3v2.3/2.4: 4-byte size
			if position+10 > len(tagData) {
				break
			}
			frameSize = int(tagData[position+4])<<24 | int(tagData[position+5])<<16 |
				int(tagData[position+6])<<8 | int(tagData[position+7])
			position += 10 + frameSize
		}

		// Safety check to prevent infinite loops
		if frameSize <= 0 || position >= len(tagData) {
			break
		}
	}

	return false, nil
}

// hasAlbumArtVorbis checks for album art in FLAC, OGG, and OPUS files
func hasAlbumArtVorbis(data []byte) bool {
    // Check for FLAC signature and picture block
    if len(data) >= 4 && string(data[0:4]) == "fLaC" {
        return bytes.Contains(data, []byte("\x06")) // Picture block type
    }

    // Check for OGG signature and cover art (opus included)
    if len(data) >= 4 && string(data[0:4]) == "OggS" {
        return bytes.Contains(data, []byte("METADATA_BLOCK_PICTURE")) ||
               bytes.Contains(data, []byte("COVERART")) ||
               bytes.Contains(data, []byte("COVER_ART"))
    }

    return false
}

// hasAlbumArtM4A checks for embedded artwork in M4A/MP4 files
func hasAlbumArtM4A(data []byte) bool {
    signatures := [][]byte{
        []byte("\xFF\xD8\xFF"),            // JPEG
        []byte("\x89PNG\x0D\x0A\x1A\x0A"), // PNG
        []byte("GIF8"),                    // GIF
        []byte("BM"),                      // BMP
        []byte("\x00\x00\x00\x1C\x66\x74\x79\x70"), // MP4/M4A start
    }

    for _, sig := range signatures {
        if bytes.Contains(data, sig) {
            return true
        }
    }
    return false
}

func HasAlbumArt(filePath string, extension string) bool {
	if !CouldHaveAlbumArt(extension) {
		return false
	}

	// Use low-level detection for all audio formats
	hasArt, err := hasAlbumArtLowLevel(filePath, extension)
	if err != nil {
		// If there's an error with low-level detection, return false
		// This is safer than potentially crashing the indexing process
		return false
	}

	return hasArt
}

// DetectTypeByHeader detects the MIME type of a file based on its header.
func DetectTypeByHeader(realPath string) string {
	file, err := os.Open(realPath)
	if err != nil {
		return "blob"
	}
	defer file.Close()

	buffer := make([]byte, 512)
	n, err := file.Read(buffer)
	if err != nil {
		return "blob"
	}
	return http.DetectContentType(buffer[:n])
}

// returns true if the file name contains the search term
// returns file type if the file name contains the search term
// returns size of file/dir if the file name contains the search term
func (fi ItemInfo) ContainsSearchTerm(searchTerm string, options SearchOptions) bool {

	fileTypes := map[string]bool{}
	largerThan := int64(options.LargerThan) * 1024 * 1024
	smallerThan := int64(options.SmallerThan) * 1024 * 1024
	conditions := options.Conditions
	lowerFileName := strings.ToLower(fi.Name)

	// Convert to lowercase if not exact match
	if !conditions["exact"] {
		searchTerm = strings.ToLower(searchTerm)
	}

	// Check if the file name contains the search term
	if !strings.Contains(lowerFileName, searchTerm) {
		return false
	}

	// Initialize file size and fileTypes map
	var fileSize int64
	extension := filepath.Ext(lowerFileName)

	// Collect file types
	for _, k := range AllFiletypeOptions {
		if IsMatchingType(extension, k) {
			fileTypes[k] = true
		}
	}
	isDir := fi.Type == "directory"
	fileTypes["dir"] = isDir
	fileSize = fi.Size

	// Evaluate all conditions
	for t, v := range conditions {
		if t == "exact" {
			continue
		}
		switch t {
		case "larger":
			if largerThan > 0 {
				if fileSize <= largerThan {
					return false
				}
			}
		case "smaller":
			if smallerThan > 0 {
				if fileSize >= smallerThan {
					return false
				}
			}
		default:
			// Handle other file type conditions
			notMatchType := v != fileTypes[t]
			if notMatchType {
				return false
			}
		}
	}

	return true
}

// IsDirectory determines if a path should be treated as a directory.
// It treats known bundle-style directories as files instead.
func IsDirectory(fileInfo os.FileInfo) bool {
	if !fileInfo.IsDir() {
		return false
	}

	if !hasBundleExtension(fileInfo.Name()) {
		return true
	}

	// For bundle-type dirs, treat them as files
	return false
}

// hasBundleExtension checks if a file has a known bundle-style extension.
func hasBundleExtension(name string) bool {
	ext := strings.ToLower(filepath.Ext(name))
	for _, bundleExt := range BundleExtensions {
		if ext == bundleExt {
			return true
		}
	}
	return false
}

func HasDocConvertableExtension(name, mimetype string) bool {
	if !settings.Config.Server.MuPdfAvailable {
		return false
	}
	if strings.HasPrefix(mimetype, "text") {
		return true
	}
	ext := strings.ToLower(filepath.Ext(name))
	val, ok := MuPdfConvertable[ext]
	if ok {
		return val
	}
	return false
}

var ONLYOFFICE_READONLY_FILE_EXTENSIONS = []string{"pages", "numbers", "key"}

func CanEditOnlyOffice(modify bool, extention string) bool {
	if !modify {
		return false
	}
	return !slices.Contains(ONLYOFFICE_READONLY_FILE_EXTENSIONS, strings.ToLower(extention))
}
