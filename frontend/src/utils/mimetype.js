// Raw camera image MIME types (must match backend iteminfo.rawImageMimeType)
const RAW_IMAGE_MIME_TYPES = new Set([
  "image/x-canon-cr2", "image/x-canon-cr3", "image/x-nikon-nef",
  "image/x-sony-arw", "image/x-olympus-orf", "image/x-panasonic-rw2",
  "image/x-panasonic-raw", "image/x-adobe-dng", "image/x-fuji-raf",
  "image/x-pentax-pef", "image/x-leica-rwl", "image/x-hasselblad-3fr",
  "image/x-hasselblad-fff", "image/x-epson-erf", "image/x-minolta-mrw",
  "image/x-kodak-dcr", "image/x-kodak-dc2", "image/x-sigma-x3f",
  "image/x-phaseone-iiq", "image/x-kodak-nkc", "image/x-red-r3d",
]);

// Extension to type mappings (based on backend conditions.go)
const IMAGE_EXTENSIONS = new Set([
  ".jpg", ".jpeg", ".png", ".gif", ".bmp", ".svg", ".webp", ".ico",
  ".tiff", ".tif", ".heic", ".heif", ".avif", ".jfif", ".pjpeg", ".pjp",
  ".pbm", ".pgm", ".ppm", ".pam",
  // Raw image formats
  ".cr2", ".cr3", ".nef", ".nrw", ".arw", ".srf", ".sr2", ".orf",
  ".rw2", ".raw", ".dng", ".raf", ".pef", ".ptx", ".rwl", ".3fr",
  ".fff", ".erf", ".mrw", ".dcr", ".kdc", ".dc2", ".x3f", ".iiq", ".nkc", ".r3d"
]);

const VIDEO_EXTENSIONS = new Set([
  ".mp4", ".avi", ".mkv", ".mov", ".wmv", ".flv", ".webm", ".m4v",
  ".mpg", ".mpeg", ".3gp", ".ogv", ".ts", ".m3u8"
]);

const AUDIO_EXTENSIONS = new Set([
  ".mp3", ".wav", ".ogg", ".m4a", ".flac", ".aac", ".wma", ".opus",
  ".ape", ".wv", ".aiff", ".aif", ".mid", ".midi"
]);

const ARCHIVE_EXTENSIONS = new Set([
  ".zip", ".rar", ".7z", ".tar", ".gz", ".bz2", ".xz", ".tgz", ".tbz2",
  ".lzma", ".lz4", ".zstd"
]);

const DOCUMENT_EXTENSIONS = new Set([
  ".doc", ".docx", ".pdf", ".odt", ".rtf", ".ppt", ".pptx", ".odp",
  ".xls", ".xlsx", ".ods", ".epub", ".mobi", ".fb2", ".gdoc", ".gsheet",
  ".conf", ".bash_history", ".gitignore", ".htpasswd", ".profile",
  ".dockerignore", ".editorconfig"
]);

const TEXT_EXTENSIONS = new Set([
  ".txt", ".md", ".sh", ".py", ".js", ".ts", ".php", ".rb", ".go",
  ".java", ".c", ".cpp", ".cs", ".swift", ".yaml", ".yml", ".json",
  ".xml", ".ini", ".toml", ".cfg", ".css", ".html", ".htm", ".sql",
  ".csv", ".tsv", ".log", ".bat", ".ps1", ".tex", ".bib"
]);

const MODEL_3D_EXTENSIONS = new Set([
  ".gltf", ".glb", ".obj", ".stl", ".ply", ".dae", ".3mf", ".3ds",
  ".usdz", ".usd", ".usda", ".usdc", ".amf", ".vrml", ".wrl",
  ".vtk", ".vtp", ".pcd", ".xyz", ".vox"
]);

export function isRawImageMimeType(mimeType) {
  return typeof mimeType === "string" && RAW_IMAGE_MIME_TYPES.has(mimeType);
}

export function getTypeInfo(mimeType) {
    if (!mimeType) {
        return {
            classes: "material-icons",
            materialIcon: "file",
            simpleType: "file",
        };
    }
    if (mimeType === "directory" || mimeType === "application/vnd.google-apps.folder") {
        return {
            classes: "primary-icons material-icons",
            materialIcon: "folder",
            simpleType: "directory",
        };
    }

    if (mimeType.startsWith("image/")) {
        return {
            classes: "orange-icons material-icons",
            materialIcon: "photo",
            simpleType: "image",
        };
    }

    if (
        mimeType.startsWith("audio/") ||
        mimeType === "application/vnd.google-apps.audio"
    ) {
        return {
            classes: "plum-icons material-icons",
            materialIcon: "volume_up",
            simpleType: "audio",
        };
    }

    if (
        mimeType.startsWith("video/") ||
        mimeType === "application/vnd.google-apps.video"
    ) {
        return {
            classes: "skyblue-icons material-icons",
            materialIcon: "movie",
            simpleType: "video",
        };
    }

    if (mimeType == "file_download") {
        return {
            classes: "material-icons simple-icons",
            materialIcon: "file_download",
            simpleType: "file_download",
        };
    }

    if (mimeType.startsWith("font/")) {
        return {
            classes: "gray-icons material-icons",
            materialIcon: "font_download",
            simpleType: "font",
        };
    }

    if (
        mimeType === "application/zip" ||
        mimeType === "application/x-7z-compressed" ||
        mimeType === "application/x-bzip" ||
        mimeType === "application/x-rar-compressed" ||
        mimeType === "application/x-tar" ||
        mimeType === "application/gzip" ||
        mimeType === "application/x-xz" ||
        mimeType === "application/x-zip-compressed" ||
        mimeType === "application/x-compressed" ||
        mimeType === "application/x-gzip"
    ) {
        return {
            classes: "tan-icons material-icons",
            materialIcon: "archive",
            simpleType: "archive",
        };
    }

    if (mimeType === "application/pdf") {
        return {
            classes: "red-icons material-icons",
            materialIcon: "picture_as_pdf",
            simpleType: "pdf",
        };
    }

    if (
        mimeType === "application/msword" ||
        mimeType ===
        "application/vnd.openxmlformats-officedocument.wordprocessingml.document" ||
        mimeType === "application/vnd.google-apps.document" ||
        mimeType === "text/rtf" ||
        mimeType === "application/rtf"
    ) {
        return {
            classes: "deep-blue-icons material-icons",
            materialIcon: "description",
            simpleType: "document",
        };
    }

    if (
        mimeType === "application/vnd.ms-excel" ||
        mimeType ===
        "application/vnd.openxmlformats-officedocument.spreadsheetml.sheet" ||
        mimeType === "application/vnd.google-apps.spreadsheet"
    ) {
        return {
            classes: "green-icons material-icons",
            materialIcon: "border_all",
            simpleType: "document",
        };
    }
    if (mimeType === "text/csv") {
        return {
            classes: "green-icons material-icons",
            materialIcon: "border_all",
            simpleType: "document",
        };
    }

    if (
        mimeType === "application/vnd.ms-powerpoint" ||
        mimeType ===
        "application/vnd.openxmlformats-officedocument.presentationml.presentation" ||
        mimeType === "application/vnd.google-apps.presentation"
    ) {
        return {
            classes: "red-orange-icons material-icons",
            materialIcon: "slideshow",
            simpleType: "document",
        };
    }

    if (mimeType.startsWith("text/")) {
        return {
            classes: "white-icons material-icons",
            materialIcon: "description",
            simpleType: "text",
        };
    }

    if (mimeType === "application/json" || mimeType === "application/xml") {
        return {
            classes: "yellow-icons material-icons",
            materialIcon: "code",
            simpleType: "text",
        };
    }

    if (
        mimeType === "application/octet-stream" ||
        mimeType === "application/x-executable" ||
        mimeType === "application/vnd.google-apps.unknown"
    ) {
        return {
            classes: "gray-icons material-icons",
            materialIcon: "memory",
            simpleType: "binary",
        };
    }

    if (mimeType === "application/javascript" || mimeType === "text/javascript") {
        return {
            classes: "yellow-icons material-symbols-outlined",
            materialIcon: "javascript",
            simpleType: "text",
        };
    }

    if (
        mimeType === "application/x-python" ||
        mimeType === "application/vnd.google-apps.sites"
    ) {
        return {
            classes: "gray-icons material-symbols-outlined",
            materialIcon: "code_blocks",
            simpleType: "text",
        };
    }

    if (
        mimeType === "application/x-disk-image" ||
        mimeType === "application/x-iso-image" ||
        mimeType === "application/x-apple-diskimage" ||
        mimeType === "application/x-cd-image"
    ) {
        return {
            classes: "gray-icons material-symbols-outlined",
            materialIcon: "album",
            simpleType: "binary",
        };
    }

    if (mimeType === "invalid_link") {
        return {
            classes: "lightgray-icons material-icons",
            materialIcon: "link_off",
            simpleType: "invalid_link",
        };
    }

    // 3D model formats
    if (
        mimeType.startsWith("model/") ||
        mimeType === "model/gltf+json" ||
        mimeType === "model/gltf-binary" ||
        mimeType === "model/obj" ||
        mimeType === "model/stl" ||
        mimeType === "model/ply" ||
        mimeType === "model/vnd.collada+xml" ||
        mimeType === "model/3mf" ||
        mimeType === "model/3ds" ||
        mimeType === "model/vnd.usdz+zip" ||
        mimeType === "model/vnd.usd+zip" ||
        mimeType === "model/x-amf" ||
        mimeType === "model/vrml" ||
        mimeType === "model/x-vrml" ||
        mimeType === "model/vtk" ||
        mimeType === "model/vox" ||
        mimeType === "application/vnd.google-earth.kmz"
    ) {
        return {
            classes: "purple-icons material-icons",
            materialIcon: "view_in_ar",
            simpleType: "3d-model",
        };
    }

    // Default fallback
    return {
        classes: "lightgray-icons material-icons",
        materialIcon: "description",
        simpleType: "blob",
    };
}

/**
 * Get type information based on file name/extension (fallback when mimeType is not available)
 * @param {string} fileName - The name of the file including extension
 * @returns {object} Type information object with classes, materialIcon, and simpleType
 */
export function getTypeInfoFromName(fileName) {
    if (!fileName) {
        return {
            classes: "material-icons",
            materialIcon: "file",
            simpleType: "file",
        };
    }

    // Get extension (lowercase)
    const ext = fileName.includes('.') 
        ? '.' + fileName.split('.').pop().toLowerCase() 
        : '';

    // Check for specific known files without extensions or special cases
    if (ext === '' || fileName.startsWith('.')) {
        // Check if it's a known config file
        if (TEXT_EXTENSIONS.has(fileName.toLowerCase()) || DOCUMENT_EXTENSIONS.has(fileName.toLowerCase())) {
            return {
                classes: "white-icons material-icons",
                materialIcon: "description",
                simpleType: "text",
            };
        }
    }

    // 3D models
    if (MODEL_3D_EXTENSIONS.has(ext)) {
        return {
            classes: "purple-icons material-icons",
            materialIcon: "view_in_ar",
            simpleType: "3d-model",
        };
    }

    // Images
    if (IMAGE_EXTENSIONS.has(ext)) {
        return {
            classes: "orange-icons material-icons",
            materialIcon: "photo",
            simpleType: "image",
        };
    }

    // Videos
    if (VIDEO_EXTENSIONS.has(ext)) {
        return {
            classes: "skyblue-icons material-icons",
            materialIcon: "movie",
            simpleType: "video",
        };
    }

    // Audio
    if (AUDIO_EXTENSIONS.has(ext)) {
        return {
            classes: "plum-icons material-icons",
            materialIcon: "volume_up",
            simpleType: "audio",
        };
    }

    // Archives
    if (ARCHIVE_EXTENSIONS.has(ext)) {
        return {
            classes: "tan-icons material-icons",
            materialIcon: "archive",
            simpleType: "archive",
        };
    }

    // PDF
    if (ext === '.pdf') {
        return {
            classes: "red-icons material-icons",
            materialIcon: "picture_as_pdf",
            simpleType: "pdf",
        };
    }

    // Word documents
    if (['.doc', '.docx', '.rtf', '.odt'].includes(ext)) {
        return {
            classes: "deep-blue-icons material-icons",
            materialIcon: "description",
            simpleType: "document",
        };
    }

    // Excel/spreadsheets
    if (['.xls', '.xlsx', '.ods', '.csv'].includes(ext)) {
        return {
            classes: "green-icons material-icons",
            materialIcon: "border_all",
            simpleType: "document",
        };
    }

    // PowerPoint/presentations
    if (['.ppt', '.pptx', '.odp'].includes(ext)) {
        return {
            classes: "red-orange-icons material-icons",
            materialIcon: "slideshow",
            simpleType: "document",
        };
    }

    // JavaScript
    if (ext === '.js' || ext === '.mjs' || ext === '.cjs') {
        return {
            classes: "yellow-icons material-symbols-outlined",
            materialIcon: "javascript",
            simpleType: "text",
        };
    }

    // Python
    if (ext === '.py') {
        return {
            classes: "gray-icons material-symbols-outlined",
            materialIcon: "code_blocks",
            simpleType: "text",
        };
    }

    // JSON/XML
    if (ext === '.json' || ext === '.xml') {
        return {
            classes: "yellow-icons material-icons",
            materialIcon: "code",
            simpleType: "text",
        };
    }

    // Disk images
    if (['.iso', '.dmg', '.img'].includes(ext)) {
        return {
            classes: "gray-icons material-symbols-outlined",
            materialIcon: "album",
            simpleType: "binary",
        };
    }

    // Text files (catch-all for text extensions)
    if (TEXT_EXTENSIONS.has(ext)) {
        return {
            classes: "white-icons material-icons",
            materialIcon: "description",
            simpleType: "text",
        };
    }

    // Document files (catch-all for document extensions)
    if (DOCUMENT_EXTENSIONS.has(ext)) {
        return {
            classes: "deep-blue-icons material-icons",
            materialIcon: "description",
            simpleType: "document",
        };
    }

    // Default fallback
    return {
        classes: "lightgray-icons material-icons",
        materialIcon: "description",
        simpleType: "blob",
    };
}