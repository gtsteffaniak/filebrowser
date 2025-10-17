
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
        mimeType === "application/x-apple-diskimage"
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

    // Default fallback
    return {
        classes: "lightgray-icons material-icons",
        materialIcon: "description",
        simpleType: "blob",
    };
}