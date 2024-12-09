// Returns an object with classes and materialIcon
// Example: { classes: "icon-classes", materialIcon: "volume_up" }
export function getIconForType(mimeType) {
    switch (mimeType) {
        case "directory":
            return { classes: "folder-icons", materialIcon: "folder" };
        case "audio/mpeg":
        case "audio/wav":
        case "audio/ogg":
        case "audio/mp3":
        case "audio/flac":
        case "application/vnd.google-apps.audio":
            return { classes: "audio-icons", materialIcon: "volume_up" };
        case "image/jpeg":
        case "image/png":
        case "image/gif":
        case "image/svg+xml":
        case "image/webp":
        case "image/bmp":
        case "application/vnd.google-apps.photo":
            return { classes: "image-icons", materialIcon: "photo" };
        case "video/mp4":
        case "video/x-matroska":
        case "video/quicktime":
        case "video/webm":
        case "video/avi":
        case "application/vnd.google-apps.video":
            return { classes: "video-icons", materialIcon: "movie" };
        case "application/zip":
        case "application/x-7z-compressed":
        case "application/x-bzip":
        case "application/x-rar-compressed":
        case "application/x-tar":
        case "application/gzip":
            return { classes: "archive-icons", materialIcon: "archive" };
        case "application/pdf":
            return { classes: "pdf-icons", materialIcon: "picture_as_pdf" };
        case "application/msword":
        case "application/vnd.openxmlformats-officedocument.wordprocessingml.document":
        case "application/vnd.google-apps.document":
            return { classes: "document-icons", materialIcon: "description" };
        case "application/vnd.ms-excel":
        case "application/vnd.openxmlformats-officedocument.spreadsheetml.sheet":
        case "application/vnd.google-apps.spreadsheet":
            return { classes: "spreadsheet-icons", materialIcon: "table_chart" };
        case "application/vnd.ms-powerpoint":
        case "application/vnd.openxmlformats-officedocument.presentationml.presentation":
        case "application/vnd.google-apps.presentation":
            return { classes: "presentation-icons", materialIcon: "slideshow" };
        case "text/plain":
        case "text/csv":
        case "application/json":
        case "application/xml":
        case "text/markdown":
            return { classes: "text-icons", materialIcon: "text_snippet" };
        case "application/octet-stream":
        case "application/x-executable":
        case "application/vnd.google-apps.unknown":
            return { classes: "binary-icons", materialIcon: "memory" };
        case "application/vnd.google-apps.folder":
            return { classes: "folder-icons", materialIcon: "folder" };
        case "application/javascript":
        case "application/x-python":
        case "text/html":
        case "text/css":
        case "text/javascript":
        case "application/vnd.google-apps.sites":
            return { classes: "code-icons", materialIcon: "code" };
        default:
            return { classes: "file-icons", materialIcon: "insert_drive_file" };
    }
}
