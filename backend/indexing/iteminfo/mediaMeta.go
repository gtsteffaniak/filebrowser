package iteminfo

import (
	"bytes"
	"fmt"
	"io"
	"net/http"
	"os"
	"path/filepath"
	"strings"
)

func CouldHaveAlbumArt(extension string) bool {
	return audioMetadataTypes[extension]
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
		buffer := make([]byte, 12288) // 12KB seems to be enought
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
	buffer := make([]byte, 32768) // 32KB - 10 bytes already read
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
	buffer := make([]byte, 32768) // 32KB - 10 bytes already read
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

	// Collect file types using the actual detected MIME type, not just extension
	for _, k := range AllFiletypeOptions {
		if IsMatchingDetectedType(fi.Type, extension, k) {
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

// ShouldBubbleUpToFolderPreview checks if a file type should be used for folder previews.
// Only images, videos, and audio files with album art should bubble up.
// Text files, office documents, and PDFs should NOT bubble up to folder previews.
// This ensures consistency between indexing and preview generation.
func ShouldBubbleUpToFolderPreview(item ItemInfo) bool {
	// Get the simple type (e.g., "image", "video", "text")
	simpleType := strings.Split(item.Type, "/")[0]
	// Text files should NOT bubble up
	if simpleType == "text" {
		return false
	}
	// Office documents should NOT bubble up
	if IsOnlyOffice(item.Name) {
		return false
	}
	// Document convertable files (PDFs, etc.) should NOT bubble up
	if HasDocConvertableExtension(item.Name, item.Type) {
		return false
	}
	// Only allow images, videos, and audio with album art to bubble up
	if simpleType == "image" || simpleType == "video" || simpleType == "audio" {
		return true
	}
	return false
}
