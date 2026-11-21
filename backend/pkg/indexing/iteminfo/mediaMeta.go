package iteminfo

import (
	"net/http"
	"os"
	"path/filepath"
	"strings"
)

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
	conditions := options.Conditions
	lowerFileName := strings.ToLower(fi.Name)

	st := searchTerm
	if !conditions["exact"] {
		st = strings.ToLower(st)
	}

	if !strings.Contains(lowerFileName, st) {
		return false
	}

	return fi.MatchesSearchAuxiliaryFilters(options)
}

// MatchesSearchAuxiliaryFilters applies type/size/date constraints from SearchOptions without checking the file name.
// Used when SQLite has already filtered names (e.g. name GLOB patterns).
func (fi ItemInfo) MatchesSearchAuxiliaryFilters(options SearchOptions) bool {
	fileTypes := map[string]bool{}
	largerThan := int64(options.LargerThan) * 1024 * 1024
	smallerThan := int64(options.SmallerThan) * 1024 * 1024
	conditions := options.Conditions
	lowerFileName := strings.ToLower(fi.Name)

	var fileSize int64
	extension := filepath.Ext(lowerFileName)

	for _, k := range AllFiletypeOptions {
		if IsMatchingDetectedType(fi.Type, extension, k) {
			fileTypes[k] = true
		}
	}
	isDir := fi.Type == "directory"
	fileTypes["dir"] = isDir
	fileSize = fi.Size

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
			notMatchType := v != fileTypes[t]
			if notMatchType {
				return false
			}
		}
	}

	if options.ModifiedNewerThan > 0 && fi.ModTime.Unix() < options.ModifiedNewerThan {
		return false
	}
	if options.ModifiedOlderThan > 0 && fi.ModTime.Unix() >= options.ModifiedOlderThan {
		return false
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
	if item.Type == "directory" || !item.HasPreview {
		return false
	}
	// Get the simple type (e.g., "image", "video", "text")
	simpleType := strings.Split(item.Type, "/")[0]
	// Only allow images, videos, and audio with album art to bubble up
	if simpleType == "image" || simpleType == "video" || simpleType == "audio" {
		return true
	}
	return false
}
