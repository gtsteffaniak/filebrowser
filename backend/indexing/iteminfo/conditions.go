package iteminfo

import (
	"mime"
	"path/filepath"
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

var SubtitleExts = map[string]bool{
	".vtt": true,
	".srt": true,
	".lrc": true,
	".sbv": true,
	".ass": true,
	".ssa": true,
	".sub": true,
	".smi": true,
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
	".tiff":  true,
	".tif":   true,
	".heic":  true,
	".heif":  true,
	".webp":  true,
	".pbm":   true,
	".pgm":   true,
	".ppm":   true,
	".pam":   true,
	".other": false,
}

// Known bundle-style extensions that are technically directories but treated as files
var BundleExtensions = map[string]bool{
	".app":       true, // macOS application bundle
	".bundle":    true, // macOS plugin bundle
	".framework": true, // macOS framework
	".plugin":    true, // macOS plugin
	".kext":      true, // macOS kernel extension
	".pkg":       true, // macOS installer package
	".mpkg":      true, // macOS multi-package
	".apk":       true, // Android package
	".aab":       true, // Android App Bundle
	".appx":      true, // Windows application package
	".msix":      true, // Windows modern app package
	".deb":       true, // Debian package
	".snap":      true, // Snap package
	".flatpak":   true, // Flatpak application
	".dmg":       true, // macOS disk image
	".iso":       true, // ISO disk image
}

// Document file extensions
var documentTypes = map[string]bool{
	// Common Document Formats
	".doc":          true, // Microsoft Word
	".docx":         true, // Microsoft Word
	".pdf":          true, // Portable Document Format
	".odt":          true, // OpenDocument Text
	".rtf":          true, // Rich Text Format
	".conf":         true,
	".bash_history": true,
	".gitignore":    true,
	".htpasswd":     true,
	".profile":      true,
	".dockerignore": true,
	".editorconfig": true,
	".ppt":          true, // Microsoft PowerPoint
	".pptx":         true, // Microsoft PowerPoint
	".odp":          true, // OpenDocument Presentation
	".gdoc":         true, // google docs
	".gsheet":       true, // google sheet
	".xls":          true, // Microsoft Excel
	".xlsx":         true, // Microsoft Excel
	".ods":          true, // OpenDocument Spreadsheet
	".epub":         true, // Electronic Publication
	".mobi":         true, // Amazon Kindle
	".fb2":          true, // FictionBook
}

var onlyOfficeSupported = map[string]bool{
	// Word Processing Documents
	".doc":   true,
	".docm":  true,
	".docx":  true,
	".dot":   true,
	".dotm":  true,
	".dotx":  true,
	".epub":  true,
	".fb2":   true,
	".fodt":  true,
	".htm":   true,
	".html":  true,
	".mht":   true,
	".mhtml": true,
	".odt":   true,
	".ott":   true,
	".rtf":   true,
	".stw":   true,
	".sxw":   true,
	".txt":   true,
	".wps":   true,
	".wpt":   true,
	".xml":   true,
	".hwp":   true,
	".hwpx":  true,
	".md":    true,
	".pages": true,
	// Spreadsheet Documents
	".csv":     true,
	".et":      true,
	".ett":     true,
	".fods":    true,
	".ods":     true,
	".ots":     true,
	".sxc":     true,
	".xls":     true,
	".xlsb":    true,
	".xlsm":    true,
	".xlsx":    true,
	".xlt":     true,
	".xltm":    true,
	".xltx":    true,
	".numbers": true,
	// Presentation Documents
	".dps":  true,
	".dpt":  true,
	".fodp": true,
	".odp":  true,
	".otp":  true,
	".pot":  true,
	".potm": true,
	".potx": true,
	".pps":  true,
	".ppsm": true,
	".ppsx": true,
	".ppt":  true,
	".pptm": true,
	".pptx": true,
	".sxi":  true,
	".key":  true,
	".odg":  true,
	// Other Office-Related Formats
	".djvu":  true,
	".docxf": true,
	".oform": true,
	".oxps":  true,
	".pdf":   true,
	".xps":   true,
	// Diagram Documents
	".vsdm": true,
	".vsdx": true,
	".vssm": true,
	".vssx": true,
	".vstm": true,
	".vstx": true,
}

// Text-based file extensions
var textTypes = map[string]bool{
	// Common Text Formats
	".txt":   true,
	".md":    true, // Markdown
	".sh":    true, // Bash script
	".py":    true, // Python
	".js":    true, // JavaScript
	".ts":    true, // TypeScript
	".php":   true, // PHP
	".rb":    true, // Ruby
	".go":    true, // Go
	".java":  true, // Java
	".c":     true, // C
	".cpp":   true, // C++
	".cs":    true, // C#
	".swift": true, // Swift
	".yaml":  true, // YAML
	".yml":   true, // YAML
	".json":  true, // JSON
	".xml":   true, // XML
	".ini":   true, // INI
	".toml":  true, // TOML
	".cfg":   true, // Configuration file
	".css":   true, // Cascading Style Sheets
	".html":  true, // HyperText Markup Language
	".htm":   true, // HyperText Markup Language
	".sql":   true, // SQL
	".csv":   true, // Comma-Separated Values
	".tsv":   true, // Tab-Separated Values
	".log":   true, // Log file
	".bat":   true, // Batch file
	".ps1":   true, // PowerShell script
	".tex":   true, // LaTeX
	".bib":   true, // BibTeX
}

// Compressed file extensions
var compressedFile = map[string]bool{
	".7z":   true,
	".rar":  true,
	".zip":  true,
	".tar":  true,
	".gz":   true,
	".xz":   true,
	".bz2":  true,
	".tgz":  true, // tar.gz
	".tbz2": true, // tar.bz2
	".lzma": true,
	".lz4":  true,
	".zstd": true,
}

var audioMetadataTypes = map[string]bool{
	".mp3":  true,
	".flac": true,
	".ogg":  true,
	".opus": true,
	".m4a":  true,
	".mp4":  true,
	".wav":  true,
	".ape":  true,
	".wv":   true,
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
	return textTypes[extension]
}

func IsDoc(extension string) bool {
	return documentTypes[extension]
}

func IsArchive(extension string) bool {
	return compressedFile[extension]
}

func IsOnlyOffice(name string) bool {
	extention := filepath.Ext(name)
	return onlyOfficeSupported[extention]
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

// IsMatchingDetectedType checks if the detected MIME type matches the search type.
// This uses the actual detected type rather than just the extension, which is more accurate
// for ambiguous extensions like .ts (TypeScript vs MPEG Transport Stream)
func IsMatchingDetectedType(detectedType string, extension string, matchType string) bool {
	// Check if the detected MIME type starts with the match type (e.g., "video/", "image/", "audio/")
	// No need to strip charset since HasPrefix works with it
	if strings.HasPrefix(detectedType, matchType+"/") {
		return true
	}

	// For special categories that don't map directly to MIME type prefixes,
	// use extension-based checking
	switch matchType {
	case "doc":
		// Check if detected type is application/document or use extension check
		if strings.HasPrefix(detectedType, "application/document") {
			return true
		}
		return IsDoc(extension)
	case "text":
		// Already checked with HasPrefix above
		return false
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

// hasBundleExtension checks if a file has a known bundle-style extension.
func hasBundleExtension(name string) bool {
	ext := strings.ToLower(filepath.Ext(name))
	return BundleExtensions[ext]
}

func HasDocConvertableExtension(name, mimetype string) bool {
	if !settings.Env.MuPdfAvailable {
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

var ONLYOFFICE_READONLY_FILE_EXTENSIONS = map[string]bool{
	"pages":   true,
	"numbers": true,
	"key":     true,
}

func CanEditOnlyOffice(modify bool, extention string) bool {
	if !modify {
		return false
	}
	return !ONLYOFFICE_READONLY_FILE_EXTENSIONS[strings.ToLower(extention)]
}
