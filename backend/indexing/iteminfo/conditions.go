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

    // Look for common signatures in first 64KB for M4A files
    buffer := make([]byte, 65536)
    n, err := file.Read(buffer)
    if err != nil && err != io.EOF {
        return false, err
    }

    data := buffer[:n]

    // Signature check to detect album art without parsing the entire file
    if hasImageSignature(data) {
        return true, nil
    }

	// Handle different audio formats
	switch extension {
	case ".mp3":
		return hasAlbumArtMP3(file)
	case ".flac", ".ogg", ".opus":
		return hasAlbumArtVorbis(file)
	case ".m4a", ".mp4":
		return hasAlbumArtM4A(file, data)
	default:
		// For other formats, return false (no album art detection)
		return false, nil
	}
}

// hasAlbumArtMP3 checks for APIC/PIC frames in ID3v2 tags with optimized parsing
func hasAlbumArtMP3(file *os.File) (bool, error) {
	// Read 10-byte ID3v2 header
	header := make([]byte, 10)
	if _, err := io.ReadFull(file, header); err != nil {
		return false, nil // No ID3v2 tag found
	}

	// Check for "ID3" identifier
	if string(header[0:3]) != "ID3" {
		return false, nil // No ID3v2 tag found
	}

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

	// Decode synchsafe tag size
	tagSize := (int(header[6]) << 21) | (int(header[7]) << 14) | (int(header[8]) << 7) | int(header[9])

	// Limit reading to reasonable size for performance
	maxReadSize := 65536 // 64KB should be enough to find picture frames
	readSize := tagSize
	if readSize > maxReadSize {
		readSize = maxReadSize
	}

	// Read tag data
	tagData := make([]byte, readSize)
	if _, err := io.ReadFull(file, tagData); err != nil {
		return false, nil // Failed to read, assume no artwork
	}

	position := 0
	frameIDLen := len(pictureFrameID)

	// Loop through frames looking for picture frame
	for position+frameHeaderSize <= readSize {
		// Check if we've hit padding (null bytes)
		if tagData[position] == 0 {
			break
		}

		// Extract frame ID
		if position+frameIDLen > readSize {
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
			if position+6 > readSize {
				break
			}
			frameSize = int(tagData[position+3])<<16 | int(tagData[position+4])<<8 | int(tagData[position+5])
			position += 6 + frameSize
		} else {
			// ID3v2.3/2.4: 4-byte size
			if position+10 > readSize {
				break
			}
			frameSize = int(tagData[position+4])<<24 | int(tagData[position+5])<<16 |
				int(tagData[position+6])<<8 | int(tagData[position+7])
			position += 10 + frameSize
		}

		// Safety check to prevent infinite loops
		if frameSize <= 0 || position >= tagSize {
			break
		}
	}

	return false, nil
}

// hasAlbumArtVorbis checks for album art in FLAC, OGG, and OPUS files using surgical format-specific parsing
func hasAlbumArtVorbis(file *os.File) (bool, error) {
	// Read initial buffer to determine file type
	buffer := make([]byte, 32768) // 32KB should be enough for most metadata sections
	n, err := file.Read(buffer)
	if err != nil && err != io.EOF {
		return false, err
	}

	// Check if it's a FLAC file
	if n >= 4 && string(buffer[0:4]) == "fLaC" {
		return hasAlbumArtFLAC(buffer, n)
	}

	// Check if it's an OGG file (including OPUS)
	if n >= 4 && string(buffer[0:4]) == "OggS" {
		return hasAlbumArtOGG(buffer, n)
	}

	return false, nil
}

// hasAlbumArtFLAC checks for PICTURE metadata block (type 6) in FLAC files
func hasAlbumArtFLAC(buffer []byte, n int) (bool, error) {
	pos := 4 // Skip "fLaC" header

	for pos < n-4 {
		// Read metadata block header (4 bytes total)
		blockHeader := buffer[pos]
		blockType := blockHeader & 0x7F // bits 0-6
		isLast := blockHeader&0x80 != 0 // bit 7

		// Found picture block!
		if blockType == 6 { // PICTURE block type
			return true, nil
		}

		// Get block size (next 3 bytes, big-endian)
		if pos+4 > n {
			break
		}
		blockSize := int(buffer[pos+1])<<16 | int(buffer[pos+2])<<8 | int(buffer[pos+3])

		// Move to next block
		pos += 4 + blockSize

		// Stop if this was the last block or we've exceeded buffer
		if isLast || pos >= n {
			break
		}
	}

	return false, nil
}

// hasAlbumArtOGG checks for METADATA_BLOCK_PICTURE in OGG Vorbis comments (including OPUS)
func hasAlbumArtOGG(buffer []byte, n int) (bool, error) {
	// Look for Vorbis comment patterns
	vorbisPrefix := []byte("\x03vorbis")
	opusPrefix := []byte("OpusTags")

	// Search for either Vorbis or Opus tags
	for i := 0; i < n-8; i++ {
		if i+len(vorbisPrefix) < n && bytes.Equal(buffer[i:i+len(vorbisPrefix)], vorbisPrefix) {
			// Found Vorbis comment block, look for METADATA_BLOCK_PICTURE
			return searchForPictureInVorbisComment(buffer[i+len(vorbisPrefix):], n-i-len(vorbisPrefix))
		}
		if i+len(opusPrefix) < n && bytes.Equal(buffer[i:i+len(opusPrefix)], opusPrefix) {
			// Found Opus tags block, look for METADATA_BLOCK_PICTURE
			return searchForPictureInVorbisComment(buffer[i+len(opusPrefix):], n-i-len(opusPrefix))
		}
	}

	return false, nil
}

// searchForPictureInVorbisComment looks for METADATA_BLOCK_PICTURE field in Vorbis comments
func searchForPictureInVorbisComment(buffer []byte, n int) (bool, error) {
	// Convert to string and look for the METADATA_BLOCK_PICTURE field
	// This field contains base64-encoded picture data
	content := string(buffer[:n])

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
func hasAlbumArtM4A(file *os.File, initialBuffer []byte) (bool, error) {
    // Look for 'covr' atom directly in initial buffer
    if bytes.Contains(initialBuffer, []byte("covr")) {
        return true, nil
    }

    // Also check for common image signatures that might indicate embedded artwork
    if hasImageSignature(initialBuffer) {
        return true, nil
    }

    // If initial buffer (64KB) wasn't enough, read more but with limits
    if len(initialBuffer) < 262144 { // 256KB total maximum
        additionalBytes := 262144 - len(initialBuffer)
        additionalBuffer := make([]byte, additionalBytes)
        n, err := file.Read(additionalBuffer)
        if err != nil && err != io.EOF {
            return false, err
        }

        // Combine buffers and search for the embedded art
        combined := append(initialBuffer, additionalBuffer[:n]...)
        if bytes.Contains(combined, []byte("covr")) || hasImageSignature(combined) {
            return true, nil
        }
    }

    // Parse MP4 atom structure more thoroughly but with limits
    return parseM4AAtoms(initialBuffer)
}

func parseM4AAtoms(data []byte) (bool, error) {
    pos := 0
    maxPos := len(data) - 8
    iterationCount := 0
    maxIterations := 1000 // Safety limit to prevent infinite loops

    for pos < maxPos && iterationCount < maxIterations {
        iterationCount++

        // Read atom size (big-endian)
        atomSize := int(data[pos])<<24 | int(data[pos+1])<<16 |
                   int(data[pos+2])<<8 | int(data[pos+3])

        // Validate atom size
        if atomSize < 8 || atomSize > len(data)-pos {
            pos += 8 // Move to next potential atom
            continue
        }

        atomType := string(data[pos+4 : pos+8])

        // Check for metadata container atoms
        if atomType == "moov" || atomType == "udta" || atomType == "meta" ||
           atomType == "ilst" || atomType == "trak" {

            // Look for 'covr' in nested atoms with depth limit
            subPos := pos + 8
            subEnd := pos + atomSize
            if subEnd > len(data) {
                subEnd = len(data)
            }

            subIteration := 0
            for subPos < subEnd-8 && subIteration < 100 {
                subIteration++

                subSize := int(data[subPos])<<24 | int(data[subPos+1])<<16 |
                          int(data[subPos+2])<<8 | int(data[subPos+3])

                if subSize < 8 || subSize > subEnd-subPos {
                    break
                }

                subType := string(data[subPos+4 : subPos+8])
                if subType == "covr" {
                    return true, nil
                }

                subPos += subSize
            }
        }

        // Check atom type directly
        if atomType == "covr" {
            return true, nil
        }

        pos += atomSize
    }

    return false, nil
}

// Check image signature for M4A files
func hasImageSignature(data []byte) bool {
    signatures := [][]byte{
        []byte("\xFF\xD8\xFF"),           // JPEG
        []byte("\x89PNG\x0D\x0A\x1A\x0A"), // PNG
        []byte("GIF8"),                   // GIF
        []byte("BM"),                     // BMP
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
