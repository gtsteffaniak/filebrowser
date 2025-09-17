package iteminfo

import (
	"bytes"
	"encoding/binary"
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

	// Handle different audio formats
	switch extension {
	case ".mp3":
		return hasAlbumArtMP3(file)
	case ".flac", ".ogg":
		return hasAlbumArtVorbis(file)
	case ".m4a", ".mp4":
		return hasAlbumArtM4A(file)
	default:
		// For other formats, return false (no album art detection)
		return false, nil
	}
}

// hasAlbumArtMP3 checks for APIC frames in ID3v2 tags
func hasAlbumArtMP3(file *os.File) (bool, error) {
	// 1. Read the 10-byte ID3v2 header.
	header := make([]byte, 10)
	if _, err := io.ReadFull(file, header); err != nil {
		return false, nil // No ID3v2 tag found
	}

	// 2. Check for the "ID3" identifier.
	if string(header[0:3]) != "ID3" {
		return false, nil // No ID3v2 tag found
	}

	// 3. Decode the tag size. It's a "synchsafe" integer.
	tagSize := (int(header[6]) << 21) | (int(header[7]) << 14) | (int(header[8]) << 7) | int(header[9])

	// 4. Read the entire tag content into a buffer.
	tagData := make([]byte, tagSize)
	if _, err := io.ReadFull(file, tagData); err != nil {
		return false, fmt.Errorf("failed to read tag data: %w", err)
	}

	position := 0
	// 5. Loop through the frames within the tag data.
	for position < tagSize {
		if position+10 > tagSize {
			break
		}

		frameHeader := tagData[position : position+10]
		frameID := string(frameHeader[0:4])

		// We found it! The APIC frame contains the album art.
		if frameID == "APIC" {
			return true, nil
		}

		// If it's not APIC, find the frame's size to skip over it.
		var frameSize uint32
		if err := binary.Read(bytes.NewReader(frameHeader[4:8]), binary.BigEndian, &frameSize); err != nil {
			return false, fmt.Errorf("corrupt frame header: %w", err)
		}

		// Move position to the next frame header.
		position += 10 + int(frameSize)

		// Break if the next frame would start with padding (null bytes).
		if position >= tagSize || tagData[position] == 0 {
			break
		}
	}

	return false, nil
}

// hasAlbumArtVorbis checks for METADATA_BLOCK_PICTURE in Vorbis comments (FLAC, OGG)
func hasAlbumArtVorbis(file *os.File) (bool, error) {
	// For FLAC files, look for METADATA_BLOCK_PICTURE in Vorbis comments
	// This is a simplified check - in practice, you'd need to parse the FLAC structure
	// For now, we'll do a basic search for common picture-related strings
	buffer := make([]byte, 8192) // Read first 8KB
	n, err := file.Read(buffer)
	if err != nil && err != io.EOF {
		return false, err
	}

	// Look for METADATA_BLOCK_PICTURE or COVERART in the buffer
	content := string(buffer[:n])
	if strings.Contains(content, "METADATA_BLOCK_PICTURE") ||
		strings.Contains(content, "COVERART") ||
		strings.Contains(content, "COVER_ART") {
		return true, nil
	}

	return false, nil
}

// hasAlbumArtM4A checks for embedded artwork in M4A/MP4 files
func hasAlbumArtM4A(file *os.File) (bool, error) {
	// For M4A files, look for 'covr' atom which contains artwork
	// This is a simplified check - in practice, you'd need to parse the MP4 structure
	buffer := make([]byte, 8192) // Read first 8KB
	n, err := file.Read(buffer)
	if err != nil && err != io.EOF {
		return false, err
	}

	// Look for 'covr' atom in the buffer
	content := string(buffer[:n])
	if strings.Contains(content, "covr") {
		return true, nil
	}

	return false, nil
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
