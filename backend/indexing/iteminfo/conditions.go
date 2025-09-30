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
// Uses a hybrid approach: small initial read for fast exit, then targeted reads based on format.
func hasAlbumArtLowLevel(filePath string, extension string) (bool, error) {
	file, err := os.Open(filePath)
	if err != nil {
		return false, fmt.Errorf("failed to open file: %w", err)
	}
	defer file.Close()

	// Step 1: Perform a small initial read to identify file type signature (magic bytes)
	initialBuffer := make([]byte, 10)
	n, err := io.ReadFull(file, initialBuffer)
	if err != nil {
		// File is too small or can't be read - no metadata possible
		return false, nil
	}

	// Step 2: Fast Path Exit & Format-Specific Handling
	switch extension {
	case ".mp3":
		// Check for ID3 signature in first 3 bytes
		if string(initialBuffer[0:3]) != "ID3" {
			// No ID3 tag, fast exit without reading more
			return false, nil
		}
		// ID3 tag found - calculate exact tag size and read only what's needed
		return hasAlbumArtMP3Optimized(file, initialBuffer)

	case ".flac":
		// Check for FLAC signature
		if n >= 4 && string(initialBuffer[0:4]) != "fLaC" {
			// Not a valid FLAC file, fast exit
			return false, nil
		}
		// Valid FLAC - read larger chunk for metadata blocks
		return hasAlbumArtFLACOptimized(file, initialBuffer)

	case ".ogg", ".opus":
		// Check for OGG signature
		if n >= 4 && string(initialBuffer[0:4]) != "OggS" {
			// Not a valid OGG file, fast exit
			return false, nil
		}
		// Valid OGG/Opus - read larger chunk for vorbis comments
		return hasAlbumArtOGGOptimized(file, initialBuffer)

	case ".m4a", ".mp4":
		// M4A/MP4 files need larger read for atom structure
		// Reset to beginning and read sufficient data
		if _, err := file.Seek(0, 0); err != nil {
			return false, err
		}
		buffer := make([]byte, 65536) // 64KB
		n, err := file.Read(buffer)
		if err != nil && err != io.EOF {
			return false, err
		}
		return hasAlbumArtM4A(buffer[:n]), nil

	default:
		// Unknown format
		return false, nil
	}
}

// hasAlbumArtMP3Optimized checks for APIC/PIC frames with minimal I/O
// Takes the 10-byte header already read and reads only the necessary tag data
func hasAlbumArtMP3Optimized(file *os.File, header []byte) (bool, error) {
	// Header already checked for "ID3" signature in caller

	// Determine ID3v2 version for proper frame ID detection
	version := header[3]
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

	// Decode synchsafe tag size from header
	tagSize := (int(header[6]) << 21) | (int(header[7]) << 14) | (int(header[8]) << 7) | int(header[9])

	// Limit reading to reasonable size for performance (32KB max)
	maxReadSize := 32768
	readSize := tagSize
	if readSize > maxReadSize {
		readSize = maxReadSize
	}
	if readSize <= 0 {
		return false, nil
	}

	// Read only the tag data needed
	tagData := make([]byte, readSize)
	n, err := io.ReadFull(file, tagData)
	if err != nil && err != io.EOF && err != io.ErrUnexpectedEOF {
		return false, nil
	}
	tagData = tagData[:n]

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

// hasAlbumArtFLACOptimized checks for PICTURE metadata block (type 6) in FLAC files
func hasAlbumArtFLACOptimized(file *os.File, initialBuffer []byte) (bool, error) {
	// Read a reasonable amount for FLAC metadata (typically in first 64KB)
	buffer := make([]byte, 65526) // 64KB - 10 bytes already read
	n, err := file.Read(buffer)
	if err != nil && err != io.EOF {
		return false, err
	}

	// Combine initial buffer with the rest
	fullData := append(initialBuffer, buffer[:n]...)

	pos := 4 // Skip "fLaC" header

	for pos < len(fullData)-4 {
		// Read metadata block header (4 bytes total)
		blockHeader := fullData[pos]
		blockType := blockHeader & 0x7F // bits 0-6
		isLast := blockHeader&0x80 != 0 // bit 7

		// Found picture block!
		if blockType == 6 { // PICTURE block type
			return true, nil
		}

		// Get block size (next 3 bytes, big-endian)
		if pos+4 > len(fullData) {
			break
		}
		blockSize := int(fullData[pos+1])<<16 | int(fullData[pos+2])<<8 | int(fullData[pos+3])

		// Move to next block
		pos += 4 + blockSize

		// Stop if this was the last block or we've exceeded buffer
		if isLast || pos >= len(fullData) {
			break
		}
	}

	return false, nil
}

// hasAlbumArtOGGOptimized checks for METADATA_BLOCK_PICTURE in OGG Vorbis comments
func hasAlbumArtOGGOptimized(file *os.File, initialBuffer []byte) (bool, error) {
	// Read a reasonable amount for OGG metadata
	buffer := make([]byte, 65526) // 64KB - 10 bytes already read
	n, err := file.Read(buffer)
	if err != nil && err != io.EOF {
		return false, err
	}

	// Combine initial buffer with the rest
	fullData := append(initialBuffer, buffer[:n]...)

	// Look for Vorbis comment patterns
	vorbisPrefix := []byte("\x03vorbis")
	opusPrefix := []byte("OpusTags")

	// Search for either Vorbis or Opus tags
	for i := 0; i < len(fullData)-8; i++ {
		if i+len(vorbisPrefix) < len(fullData) && bytes.Equal(fullData[i:i+len(vorbisPrefix)], vorbisPrefix) {
			// Found Vorbis comment block, look for METADATA_BLOCK_PICTURE
			return searchForPictureInVorbisComment(fullData[i+len(vorbisPrefix):])
		}
		if i+len(opusPrefix) < len(fullData) && bytes.Equal(fullData[i:i+len(opusPrefix)], opusPrefix) {
			// Found Opus tags block, look for METADATA_BLOCK_PICTURE
			return searchForPictureInVorbisComment(fullData[i+len(opusPrefix):])
		}
	}

	return false, nil
}

// searchForPictureInVorbisComment looks for METADATA_BLOCK_PICTURE field in Vorbis comments
func searchForPictureInVorbisComment(buffer []byte) (bool, error) {
	// Convert to string and look for the METADATA_BLOCK_PICTURE field
	content := string(buffer)

	// Look for the exact field name used in Vorbis comments
	if strings.Contains(content, "METADATA_BLOCK_PICTURE=") {
		return true, nil
	}

	// Some files might use alternative field names
	if strings.Contains(content, "COVERART=") || strings.Contains(content, "COVER_ART=") {
		return true, nil
	}

	return false, nil
}

// hasAlbumArtM4A checks for embedded artwork in M4A/MP4 files
func hasAlbumArtM4A(data []byte) bool {
	signatures := [][]byte{
		[]byte("\xFF\xD8\xFF"),                     // JPEG
		[]byte("\x89PNG\x0D\x0A\x1A\x0A"),          // PNG
		[]byte("GIF8"),                             // GIF
		[]byte("BM"),                               // BMP
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
