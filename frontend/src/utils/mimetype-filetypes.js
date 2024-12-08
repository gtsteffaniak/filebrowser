export function getMaterialIconForType(mimeType) {
    switch (mimeType) {
        case "directory":
            return ["folder-icons", "folder"];
        case "audio/mpeg":
        case "audio/wav":
        case "audio/ogg":
        case "audio/mp3":
        case "audio/flac":
            return ["audio-icons", "volume_up"];
        case "image/jpeg":
        case "image/png":
        case "image/gif":
        case "image/svg+xml":
        case "image/webp":
        case "image/bmp":
            return ["image-icons", "photo"];
        case "video/mp4":
        case "video/x-matroska":
        case "video/quicktime":
        case "video/webm":
        case "video/avi":
            return ["video-icons", "movie"];
        case "application/zip":
        case "application/x-7z-compressed":
        case "application/x-bzip":
        case "application/x-rar-compressed":
        case "application/x-tar":
        case "application/gzip":
            return ["archive-icons", "archive"];
        case "application/pdf":
            return ["pdf-icons", "picture_as_pdf"];
        case "application/msword":
        case "application/vnd.openxmlformats-officedocument.wordprocessingml.document":
            return ["document-icons", "description"];
        case "application/vnd.ms-excel":
        case "application/vnd.openxmlformats-officedocument.spreadsheetml.sheet":
            return ["spreadsheet-icons", "table_chart"];
        case "application/vnd.ms-powerpoint":
        case "application/vnd.openxmlformats-officedocument.presentationml.presentation":
            return ["presentation-icons", "slideshow"];
        case "text/plain":
        case "text/csv":
        case "application/json":
        case "application/xml":
        case "text/markdown":
            return ["text-icons", "text_snippet"];
        case "application/octet-stream":
        case "application/x-executable":
            return ["binary-icons", "memory"];
        case "application/javascript":
        case "application/x-python":
        case "text/html":
        case "text/css":
        case "text/javascript":
            return ["code-icons", "code"];
        default:
            return ["file-icons", "insert_drive_file"];
    }
}
