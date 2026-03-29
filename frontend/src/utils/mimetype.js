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

const EBOOK_MIME_TYPES = new Set([
  "application/epub+zip", "application/x-mobipocket-ebook", "application/vnd.amazon.ebook",
  "application/x-fictionbook+xml", "application/x-fb2", "application/x-cbr", "application/x-cbz",
  "application/x-cb7", "application/x-cbt", "application/vnd.comicbook+zip",
  "application/vnd.comicbook-rar", "application/x-kindle", "application/x-azw"
]);

export function isRawImageMimeType(mimeType) {
  return typeof mimeType === "string" && RAW_IMAGE_MIME_TYPES.has(mimeType);
}

export function getTypeInfo(mimeType) {
    if (!mimeType) {
        return {
            classes: "material-symbols",
            materialIcon: "file",
            simpleType: "file",
        };
    }
    if (mimeType === "directory" || mimeType === "application/vnd.google-apps.folder") {
        return {
            classes: "primary-icons material-symbols",
            materialIcon: "folder",
            simpleType: "directory",
        };
    }

    if (EBOOK_MIME_TYPES.has(mimeType)) {
        return {
            classes: "brown-icons material-symbols-outlined",
            materialIcon: "menu_book",
            simpleType: "ebook",
        };
    }

    if (mimeType.startsWith("image/gif")) {
        return {
            classes: "purple-icons material-symbols-outlined",
            materialIcon: "gif",
            simpleType: "image",
        };
    }

    if (mimeType.startsWith("image/")) {
        return {
            classes: "coral-icons material-symbols-outlined",
            materialIcon: "image",
            simpleType: "image",
        };
    }

    if (mimeType.startsWith("audio/") || mimeType === "application/vnd.google-apps.audio") {
        return {
            classes: "plum-icons material-symbols-outlined",
            materialIcon: "volume_up",
            simpleType: "audio",
        };
    }

    if (mimeType.startsWith("video/") || mimeType === "application/vnd.google-apps.video") {
        return {
            classes: "skyblue-icons material-symbols-outlined",
            materialIcon: "movie",
            simpleType: "video",
        };
    }

    if (mimeType == "file_download") {
        return {
            classes: "material-icons",
            materialIcon: "file_download",
            simpleType: "file_download",
        };
    }

    if (mimeType.startsWith("font/") || mimeType === "application/vnd.oasis.opendocument.formula-template") {
        return {
            classes: "gray-icons material-symbols-outlined",
            materialIcon: "format_color_text",
            simpleType: "font",
        };
    }

    if (mimeType === "application/zip" || mimeType === "application/x-7z-compressed" ||
        mimeType === "application/x-bzip" || mimeType === "application/x-rar-compressed" ||
        mimeType === "application/x-tar" || mimeType === "application/gzip" ||
        mimeType === "application/x-xz" || mimeType === "application/x-zip-compressed" ||
        mimeType === "application/x-compressed" || mimeType === "application/x-gzip") {
        return {
            classes: "tan-icons material-symbols",
            materialIcon: "archive",
            simpleType: "archive",
        };
    }

    if (mimeType === "application/pdf") {
        return {
            classes: "red-icons material-symbols-outlined",
            materialIcon: "picture_as_pdf",
            simpleType: "document",
        };
    }

    if (mimeType === "application/msword" || mimeType === "application/vnd.openxmlformats-officedocument.wordprocessingml.document" ||
        mimeType === "application/vnd.google-apps.document" || mimeType === "text/rtf" ||
        mimeType === "application/vnd.oasis.opendocument.text") {
        return {
            classes: "deep-blue-icons material-symbols-outlined",
            materialIcon: "docs",
            simpleType: "document",
        };
    }

    if (
        mimeType === "application/vnd.ms-excel" ||
        mimeType === "application/vnd.openxmlformats-officedocument.spreadsheetml.sheet" ||
        mimeType === "application/vnd.google-apps.spreadsheet" || mimeType === "application/excel" ||
        mimeType === "application/vnd.oasis.opendocument.spreadsheet" || mimeType === "text/csv") {
        return {
            classes: "green-icons material-symbols-outlined",
            materialIcon: "border_all",
            simpleType: "document",
        };
    }

    if (mimeType === "application/vnd.ms-powerpoint" ||
        mimeType === "application/vnd.openxmlformats-officedocument.presentationml.presentation" ||
        mimeType === "application/vnd.google-apps.presentation" || mimeType === "application/mspowerpoint" ||
        mimeType === "application/vnd.oasis.opendocument.presentation") {
        return {
            classes: "red-icons material-symbols-outlined",
            materialIcon: "slideshow",
            simpleType: "document",
        };
    }

    if (mimeType === "application/json") {
        return {
            classes: "brown-icons material-symbols-outlined",
            materialIcon: "file_json",
            simpleType: "text",
        };
    }

    if (mimeType === "application/xml") {
        return {
            classes: "yellow-icons material-symbols",
            materialIcon: "code_xml",
            simpleType: "text",
        };
    }

    if (mimeType === "application/javascript" || mimeType === "text/javascript") {
        return {
            classes: "yellow-icons material-symbols-outlined",
            materialIcon: "javascript",
            simpleType: "text",
        };
    }

    if (mimeType === "text/vue") {
        return {
            classes: "light-green-icons material-symbols-outlined",
            materialIcon: "code",
            simpleType: "text",
        };
    }

    if (mimeType === "application/x-python" || mimeType === "application/vnd.google-apps.sites" ||
        mimeType === "text/x-scriptphyton") {
        return {
            classes: "yellow-icons material-symbols-outlined",
            materialIcon: "code",
            simpleType: "text",
        };
    }

    if (mimeType === "text/markdown" || mimeType === "text/x-markdown" || mimeType === "text/x-rmarkdown" ||
        mimeType === "text/x-quarto") {
        return {
            classes: "skyblue-icons material-symbols-outlined",
            materialIcon: "markdown",
            simpleType: "text",
        };
    }

    if (mimeType === "text/html" || mimeType === "application/xhtml+xml") {
        return {
            classes: "orange-icons material-symbols-outlined",
            materialIcon: "html",
            simpleType: "text",
        };
    }

    if (mimeType === "text/xml") {
        return {
            classes: "deep-orange-icons material-symbols-outlined",
            materialIcon: "code_xml",
            simpleType: "text",
        };
    }

    if (mimeType === "text/css" || mimeType === "text/x-scss" || mimeType === "text/x-sass") {
        return {
            classes: "lightblue-icons material-symbols-outlined",
            materialIcon: "css",
            simpleType: "text",
        };
    }

    if (mimeType === "text/tab-separated-values") {
        return {
            classes: "light-green-icons material-symbols-outlined",
            materialIcon: "tsv",
            simpleType: "text",
        };
    }

    if (mimeType === "text/x-java-source") {
        return {
            classes: "brown-icons material-symbols-outlined",
            materialIcon: "local_cafe",
            simpleType: "text",
        };
    }

    if (mimeType === "text/x-scriptsh" || mimeType === "text/x-shellscript") {
        return {
            classes: "light-green-icons material-symbols-outlined",
            materialIcon: "terminal_2",
            simpleType: "text",
        };
    }

    if (mimeType === "text/x-lua") {
        return {
            classes: "blue-icons material-symbols-outlined",
            materialIcon: "blur_circular",
            simpleType: "text",
        };
    }

    if (mimeType === "text/richtext" || mimeType === "application/rtf") {
        return {
            classes: "purple-icons material-symbols-outlined",
            materialIcon: "text_fields",
            simpleType: "text",
        };
    }

    if (mimeType === "text/x-c") {
        return {
            classes: "blue-icons material-symbols-outlined",
            materialIcon: "copyright",
            simpleType: "text",
        };
    }

    if (mimeType === "text/x-rust" || mimeType === "text/rust") {
        return {
            classes: "deep-orange-icons material-symbols-outlined",
            materialIcon: "game_button_r",
            simpleType: "text",
        };
    }

    if (mimeType === "text/x-csharp" || mimeType === "text/csharp") {
        return {
            classes: "purple-icons material-symbols-outlined",
            materialIcon: "code",
            simpleType: "text",
        };
    }

    if (mimeType === "text/subtitle-srt" || mimeType === "text/subtitle-ass" ||
        mimeType === "text/subtitle-vtt" || mimeType === "text/subtitle-ssa") {
        return {
            classes: "blue-icons material-symbols-outlined",
            materialIcon: "closed_caption",
            simpleType: "text",
        };
    }

    if (mimeType === "text/x-mpegurl" || mimeType === "text/x-mpegURL" ||
        mimeType === "text/x-scpls" || mimeType === "application/xspf+xml") {
        return {
            classes: "coral-icons material-symbols-outlined",
            materialIcon: "playlist_play",
            simpleType: "text",
        };
    }

    if (mimeType === "text/x-vcard") {
        return {
            classes: "deep-orange-icons material-symbols-outlined",
            materialIcon: "contacts",
            simpleType: "text",
        };
    }

    if (mimeType === "text/config-file") {
        return {
            classes: "tan-icons material-symbols-outlined",
            materialIcon: "settings",
            simpleType: "text",
        };
    }

   // Apllication formats

    if (mimeType === "application/octet-stream" || mimeType === "application/x-executable" ||
        mimeType === "application/mac-binary" || mimeType === "application/vnd.google-apps.unknown" ||
        mimeType === "application/x-msdownload" || mimeType === "application/x-application" ||
        mimeType === "application/x-efi" || mimeType === "application/x-installer" ||
        mimeType === "application/vnd.microsoft.portable-executable" || 
        mimeType == "application/x-newton-compatible-pkg") {
        return {
            classes: "gray-icons material-symbols-outlined",
            materialIcon: "memory",
            simpleType: "binary",
        };
    }

    if (mimeType === "application/vnd.android.package-archive") {
        return {
            classes: "light-green-icons material-symbols-outlined",
            materialIcon: "android",
            simpleType: "archive",
        };
    }

    if (mimeType === "application/x-disk-image" || mimeType === "application/x-iso-image" ||
        mimeType === "application/x-apple-diskimage" || mimeType === "application/x-cd-image" ||
        mimeType === "application/vnd.efi.iso" || mimeType === "application/x-qcow2" ||
        mimeType === "application/x-vmdk") {
        return {
            classes: "lightgray-icons material-symbols",
            materialIcon: "album",
            simpleType: "binary",
        };
    }

    if (mimeType === "application/backup") {
        return {
            classes: "gray-icons material-symbols-outlined",
            materialIcon: "save",
            simpleType: "text",
        };
    }

    if (mimeType === "application/x-ruby") {
        return {
            classes: "red-icons material-symbols",
            materialIcon: "diamond",
            simpleType: "text",
        };
    }

    if (mimeType === "application/x-php") {
        return {
            classes: "blue-icons material-symbols-outlined",
            materialIcon: "php",
            simpleType: "text",
        };
    }

    if (mimeType === "application/postscript") {
        return {
            classes: "orange-icons material-symbols-outlined",
            materialIcon: "format_shapes",
            simpleType: "text",
        };
    }

    if (mimeType === "application/x-db" || mimeType === "application/sql" ||
        mimeType === "application/vnd.sqlite3") {
        return {
            classes: "blue-icons material-symbols-outlined",
            materialIcon: "database",
            simpleType: "text",
        };
    }

    if (mimeType === "application/yaml") {
        return {
            classes: "orange-icons material-symbols-outlined",
            materialIcon: "data_object",
            simpleType: "text",
        };
    }

    if (mimeType === "application/toml" || mimeType === "text/toml") {
        return {
            classes: "red-icons material-symbols-outlined",
            materialIcon: "developer_mode_tv",
            simpleType: "text",
        };
    }

    if (mimeType === "application/acad" || mimeType === "application/dxf") {
        return {
            classes: "red-icons material-symbols-outlined",
            materialIcon: "architecture",
            simpleType: "binary",
        };
    }

    if (mimeType === "application/x-shapefile" || mimeType === "application/geo+json" || 
        mimeType === "application/vnd.google-earth.kml+xml" || mimeType === "application/vnd.google-earth.kmz" ||
        mimeType === "application/gpx+xml") {
        return {
            classes: "green-icons material-symbols-outlined",
            materialIcon: "map",
            simpleType: "binary",
        };
    }

    if (mimeType === "application/x-xcf" || mimeType === "application/x-figma" ||
        mimeType === "application/x-sketch") {
        return {
            classes: "plum-icons material-symbols-outlined",
            materialIcon: "brush",
            simpleType: "binary",
        };
    }

    if (mimeType === "invalid_link") {
        return {
            classes: "lightgray-icons material-symbols",
            materialIcon: "link_off",
            simpleType: "invalid_link",
        };
    }

    // 3D model formats
    if (mimeType.startsWith("model/") || mimeType === "application/vnd.google-earth.kmz") {
        return {
            classes: "purple-icons material-symbols-outlined",
            materialIcon: "view_in_ar",
            simpleType: "3d-model",
        };
    }

    if (mimeType.startsWith("text/")) {
        return {
            classes: "white-icons material-symbols",
            materialIcon: "description",
            simpleType: "text",
        };
    }

    // Default fallback
    return {
        classes: "lightgray-icons material-symbols",
        materialIcon: "description",
        simpleType: "blob",
    };
}
