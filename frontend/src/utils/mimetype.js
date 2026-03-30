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
            materialSymbol: "file",
            simpleType: "file",
        };
    }
    if (mimeType === "directory" || mimeType === "application/vnd.google-apps.folder") {
        return {
            classes: "primary-icons material-symbols",
            materialSymbol: "folder",
            simpleType: "directory",
        };
    }

    if (EBOOK_MIME_TYPES.has(mimeType)) {
        return {
            classes: "brown-icons material-symbols-outlined",
            materialSymbol: "menu_book",
            simpleType: "ebook",
        };
    }

    if (mimeType.startsWith("image/gif")) {
        return {
            classes: "coral-icons material-symbols-outlined",
            materialSymbol: "gif",
            simpleType: "image",
        };
    }

    if (mimeType.startsWith("image/")) {
        return {
            classes: "coral-icons material-symbols-outlined",
            materialSymbol: "image",
            simpleType: "image",
        };
    }

    if (mimeType.startsWith("audio/") || mimeType === "application/vnd.google-apps.audio") {
        return {
            classes: "plum-icons material-symbols-outlined",
            materialSymbol: "volume_up",
            simpleType: "audio",
        };
    }

    if (mimeType.startsWith("video/") || mimeType === "application/vnd.google-apps.video") {
        return {
            classes: "skyblue-icons material-symbols-outlined",
            materialSymbol: "movie",
            simpleType: "video",
        };
    }

    if (mimeType == "file_download") {
        return {
            classes: "material-symbols",
            materialSymbol: "file_download",
            simpleType: "file_download",
        };
    }

    if (mimeType.startsWith("font/") || mimeType === "application/vnd.oasis.opendocument.formula-template") {
        return {
            classes: "gray-icons material-symbols-outlined",
            materialSymbol: "format_color_text",
            simpleType: "font",
        };
    }

    // archives
    if (mimeType === "application/zip" || mimeType === "application/x-7z-compressed" ||
        mimeType === "application/x-bzip" || mimeType === "application/x-rar-compressed" ||
        mimeType === "application/x-tar" || mimeType === "application/gzip" ||
        mimeType === "application/x-xz" || mimeType === "application/x-zip-compressed" ||
        mimeType === "application/x-compressed" || mimeType === "application/x-gzip") {
        return {
            classes: "tan-icons material-symbols",
            materialSymbol: "archive",
            simpleType: "archive",
        };
    }

    if (mimeType === "application/pdf") {
        return {
            classes: "red-icons material-symbols-outlined",
            materialSymbol: "picture_as_pdf",
            simpleType: "document",
        };
    }

    // documents: doc, docx, rtf, odt
    if (mimeType === "application/msword" || mimeType === "application/vnd.openxmlformats-officedocument.wordprocessingml.document" ||
        mimeType === "application/vnd.google-apps.document" || mimeType === "text/richtext" ||
        mimeType === "application/vnd.oasis.opendocument.text") {
        return {
            classes: "deep-blue-icons material-symbols-outlined",
            materialSymbol: "docs",
            simpleType: "document",
        };
    }

    // spreadsheets: xls, xlsx, ods, csv
    if (mimeType === "application/vnd.ms-excel" || 
        mimeType === "application/vnd.openxmlformats-officedocument.spreadsheetml.sheet" ||
        mimeType === "application/vnd.google-apps.spreadsheet" || mimeType === "application/excel" ||
        mimeType === "application/vnd.oasis.opendocument.spreadsheet" || mimeType === "text/csv") {
        return {
            classes: "green-icons material-symbols-outlined",
            materialSymbol: "table",
            simpleType: "document",
        };
    }

    // Presentations: ppt, pptx, odp
    if (mimeType === "application/vnd.ms-powerpoint" ||
        mimeType === "application/vnd.openxmlformats-officedocument.presentationml.presentation" ||
        mimeType === "application/vnd.google-apps.presentation" || mimeType === "application/mspowerpoint" ||
        mimeType === "application/vnd.oasis.opendocument.presentation") {
        return {
            classes: "red-icons material-symbols-outlined",
            materialSymbol: "slideshow",
            simpleType: "document",
        };
    }

    if (mimeType === "application/json" || mimeType === "application/json5") {
        return {
            classes: "brown-icons material-symbols-outlined",
            materialSymbol: "file_json",
            simpleType: "text",
        };
    }

    if (mimeType === "application/javascript" || mimeType === "text/javascript") {
        return {
            classes: "yellow-icons material-symbols-outlined",
            materialSymbol: "javascript",
            simpleType: "text",
        };
    }

    if (mimeType === "text/vue") {
        return {
            classes: "light-green-icons material-symbols-outlined",
            materialSymbol: "code",
            simpleType: "text",
        };
    }

    if (mimeType === "application/x-python" || mimeType === "application/vnd.google-apps.sites" ||
        mimeType === "text/x-scriptphyton") {
        return {
            classes: "yellow-icons material-symbols-outlined",
            materialSymbol: "code",
            simpleType: "text",
        };
    }

    if (mimeType === "text/markdown" || mimeType === "text/x-markdown" || mimeType === "text/x-rmarkdown" ||
        mimeType === "text/x-quarto") {
        return {
            classes: "skyblue-icons material-symbols-outlined",
            materialSymbol: "markdown",
            simpleType: "text",
        };
    }

    if (mimeType === "text/html" || mimeType === "application/xhtml+xml") {
        return {
            classes: "orange-icons material-symbols-outlined",
            materialSymbol: "html",
            simpleType: "text",
        };
    }

    if (mimeType === "text/xml") {
        return {
            classes: "deep-orange-icons material-symbols-outlined",
            materialSymbol: "code_xml",
            simpleType: "text",
        };
    }

    if (mimeType === "text/css" || mimeType === "text/x-scss" || mimeType === "text/x-sass") {
        return {
            classes: "lightblue-icons material-symbols-outlined",
            materialSymbol: "css",
            simpleType: "text",
        };
    }

    if (mimeType === "text/tab-separated-values") {
        return {
            classes: "light-green-icons material-symbols-outlined",
            materialSymbol: "tsv",
            simpleType: "text",
        };
    }

    if (mimeType === "text/x-java-source") {
        return {
            classes: "brown-icons material-symbols-outlined",
            materialSymbol: "local_cafe",
            simpleType: "text",
        };
    }

    // bash, sh
    if (mimeType === "text/x-scriptsh" || mimeType === "text/x-shellscript") {
        return {
            classes: "light-green-icons material-symbols-outlined",
            materialSymbol: "terminal_2",
            simpleType: "text",
        };
    }

    // lua
    if (mimeType === "text/x-lua") {
        return {
            classes: "blue-icons material-symbols-outlined",
            materialSymbol: "blur_circular",
            simpleType: "text",
        };
    }

    // C
    if (mimeType === "text/x-c") {
        return {
            classes: "blue-icons material-symbols-outlined",
            materialSymbol: "copyright",
            simpleType: "text",
        };
    }

    // rs
    if (mimeType === "text/x-rust" || mimeType === "text/rust") {
        return {
            classes: "deep-orange-icons material-symbols-outlined",
            materialSymbol: "game_button_r",
            simpleType: "text",
        };
    }

    // cs
    if (mimeType === "text/x-csharp" || mimeType === "text/csharp") {
        return {
            classes: "purple-icons material-symbols-outlined",
            materialSymbol: "tag",
            simpleType: "text",
        };
    }

    // Subtitle files: srt, vtt, ass, ssa
    if (mimeType === "text/subtitle-srt" || mimeType === "text/subtitle-ass" ||
        mimeType === "text/subtitle-vtt" || mimeType === "text/subtitle-ssa") {
        return {
            classes: "blue-icons material-symbols-outlined",
            materialSymbol: "closed_caption",
            simpleType: "text",
        };
    }

    // Playlist files: m3u, m3u8, pls, xspf
    if (mimeType === "text/x-mpegurl" || mimeType === "text/x-mpegURL" ||
        mimeType === "text/x-scpls" || mimeType === "application/xspf+xml") {
        return {
            classes: "coral-icons material-symbols-outlined",
            materialSymbol: "playlist_play",
            simpleType: "text",
        };
    }

    // Contact file: vcf
    if (mimeType === "text/x-vcard") {
        return {
            classes: "deep-orange-icons material-symbols-outlined",
            materialSymbol: "contacts",
            simpleType: "text",
        };
    }

    // config: ini, conf
    if (mimeType === "text/config-file" || mimeType === "text/ini") {
        return {
            classes: "tan-icons material-symbols-outlined",
            materialSymbol: "settings",
            simpleType: "text",
        };
    }

    // Go
    if (mimeType === "text/x-go") {
        return {
            classes: "skyblue-icons material-symbols-outlined",
            materialSymbol: "code",
            simpleType: "text",
        };
    }

    // Kt
    if (mimeType === "text/x-kotlin") {
        return {
            classes: "orange-icons material-symbols-outlined",
            materialSymbol: "code",
            simpleType: "text",
        };
    }

    // Typescrpt: ts, tsx
    if (mimeType === "text/x-typescript") {
        return {
            classes: "deep-blue-icons material-symbols-outlined",
            materialSymbol: "title",
            simpleType: "text",
        };
    }

    // Perl: pm, pl
    if (mimeType === "text/x-scriptperl" || mimeType === "text/x-scriptperl-module") {
        return {
            classes: "deep-blue-icons material-symbols-outlined",
            materialSymbol: "chess_knight",
            simpleType: "text",
        };
    }

    // Pascal: pas, p, pp 
    if (mimeType === "text/x-pascal" || mimeType === "text/pascal") {
        return {
            classes: "deep-blue-icons material-symbols-outlined",
            materialSymbol: "local_parking",
            simpleType: "text",
        };
    }

    // zig
    if (mimeType === "text/x-zig" || mimeType === "text/zig") {
        return {
            classes: "yellow-icons material-symbols-outlined",
            materialSymbol: "electric_bolt",
            simpleType: "text",
        };
    }

    // Elixir: ex, exs
    if (mimeType === "text/x-elixir") {
        return {
            classes: "purple-icons material-symbols-outlined",
            materialSymbol: "water_drop",
            simpleType: "text",
        };
    }

    // Nixos: nix
    if (mimeType === "text/nix-lang") {
        return {
            classes: "blue-icons material-symbols-outlined",
            materialSymbol: "snowflake",
            simpleType: "text",
        };
    }

    // calendar: ics
    if (mimeType === "text/calendar") {
        return {
            classes: "tan-icons material-symbols-outlined",
            materialSymbol: "calendar_month",
            simpleType: "text",
        };
    }

    // Temporary files: tmp, temp
    if (mimeType === "text/tmp") {
        return {
            classes: "gray-icons material-symbols-outlined",
            materialSymbol: "hourglass",
            simpleType: "text",
        };
    }

   // =================== //
   // Apllication mimes   //
   // =================== //
    if (mimeType === "application/octet-stream" || mimeType === "application/x-executable" ||
        mimeType === "application/mac-binary" || mimeType === "application/vnd.google-apps.unknown" ||
        mimeType === "application/x-msdownload" || mimeType === "application/x-application" ||
        mimeType === "application/x-efi" || mimeType === "application/x-installer" ||
        mimeType === "application/vnd.microsoft.portable-executable") {
        return {
            classes: "gray-icons material-symbols-outlined",
            materialSymbol: "memory",
            simpleType: "binary",
        };
    }

    // Android: APK
    if (mimeType === "application/vnd.android.package-archive") {
        return {
            classes: "light-green-icons material-symbols-outlined",
            materialSymbol: "android",
            simpleType: "archive",
        };
    }

    // Images: dmg, iso, qcow2, img, cue, vmdk...
    if (mimeType === "application/x-disk-image" || mimeType === "application/x-iso-image" ||
        mimeType === "application/x-apple-diskimage" || mimeType === "application/x-cd-image" ||
        mimeType === "application/vnd.efi.iso" || mimeType === "application/x-qcow2" ||
        mimeType === "application/x-vmdk" || mimeType === "application/x-qemu-disk" ||
        mimeType === "application/vnd.efi.img" || mimeType === "application/x-cue" ||
        mimeType === "application/x-vmdk-disk") {
        return {
            classes: "lightgray-icons material-symbols",
            materialSymbol: "album",
            simpleType: "binary",
        };
    }

    // backup, bak
    if (mimeType === "application/backup") {
        return {
            classes: "gray-icons material-symbols-outlined",
            materialSymbol: "save",
            simpleType: "text",
        };
    }

    // ruby
    if (mimeType === "application/x-ruby") {
        return {
            classes: "red-icons material-symbols",
            materialSymbol: "diamond",
            simpleType: "text",
        };
    }

    // PHP
    if (mimeType === "application/x-php") {
        return {
            classes: "blue-icons material-symbols-outlined",
            materialSymbol: "php",
            simpleType: "text",
        };
    }

    // vector: ps, eps, ai
    if (mimeType === "application/postscript") {
        return {
            classes: "orange-icons material-symbols-outlined",
            materialSymbol: "format_shapes",
            simpleType: "text",
        };
    }

    // databases: db, sqlite, sql 
    if (mimeType === "application/x-db" || mimeType === "application/sql" ||
        mimeType === "application/vnd.sqlite3") {
        return {
            classes: "blue-icons material-symbols-outlined",
            materialSymbol: "database",
            simpleType: "text",
        };
    }

    // yaml, yml
    if (mimeType === "application/yaml") {
        return {
            classes: "orange-icons material-symbols-outlined",
            materialSymbol: "data_object",
            simpleType: "text",
        };
    }

    // toml
    if (mimeType === "application/toml" || mimeType === "text/toml") {
        return {
            classes: "red-icons material-symbols-outlined",
            materialSymbol: "developer_mode_tv",
            simpleType: "text",
        };
    }

    // cad: dwg, dxf
    if (mimeType === "application/acad" || mimeType === "application/dxf") {
        return {
            classes: "red-icons material-symbols-outlined",
            materialSymbol: "architecture",
            simpleType: "binary",
        };
    }

    // map related: geojson, map, kmz, gpx, kml...
    if (mimeType === "application/x-shapefile" || mimeType === "application/geo+json" || 
        mimeType === "application/vnd.google-earth.kml+xml" || mimeType === "application/vnd.google-earth.kmz" ||
        mimeType === "application/gpx+xml" || mimeType === "application/x-navimap") {
        return {
            classes: "green-icons material-symbols-outlined",
            materialSymbol: "map",
            simpleType: "binary",
        };
    }

    // xcf, figma, fig
    if (mimeType === "application/x-xcf" || mimeType === "application/x-figma" ||
        mimeType === "application/x-sketch") {
        return {
            classes: "plum-icons material-symbols-outlined",
            materialSymbol: "brush",
            simpleType: "binary",
        };
    }

    // powershell (windows): ps, ps1, ps2, ps3, cmd, bat
    if (mimeType === "application/x-powershell" || mimeType === "application/x-msdos-program") {
        return {
            classes: "deep-blue-icons material-symbols-outlined",
            materialSymbol: "terminal",
            simpleType: "text",
        };
    }

    // flutter: dart, flutter
    if (mimeType === "application/vnd.dart" || mimeType === "text/flutter") {
        return {
            classes: "lightblue-icons material-symbols-outlined",
            materialSymbol: "flutter",
            simpleType: "text",
        };
    }

    // assembly: wasm, asm
    if (mimeType === "application/wasm" || mimeType === "text/x-asm") {
        return {
            classes: "deep-blue-icons material-symbols-outlined",
            materialSymbol: "memory",
            simpleType: "text",
        };
    }

    // packages: deb, pkg, rpm
    if (mimeType === "application/x-debian-package" || mimeType === "application/x-newton-compatible-pkg") {
        return {
            classes: "brown-icons material-symbols-outlined",
            materialSymbol: "package_2",
            simpleType: "archive",
        };
    }

    // keys: key, pem, pub
    if (mimeType === "application/x-x509-ca-cert" || mimeType === "application/vnd.apple.keynote" || 
        mimeType === "application/vnd.ms-publisher") {
        return {
            classes: "deep-orange-icons material-symbols-outlined",
            materialSymbol: "key",
            simpleType: "text",
        };
    }

    // certificates: crt, cer
    if (mimeType === "application/pkix-cert") {
        return {
            classes: "tan-icons material-symbols-outlined",
            materialSymbol: "license",
            simpleType: "text",
        };
    }

    // torrents: torrent
    if (mimeType === "application/x-bittorrent") {
        return {
            classes: "light-green-icons material-symbols-outlined",
            materialSymbol: "format_underlined",
            simpleType: "blob",
        };
    }


    if (mimeType === "invalid_link") {
        return {
            classes: "lightgray-icons material-symbols",
            materialSymbol: "link_off",
            simpleType: "invalid_link",
        };
    }

    // 3D model formats
    if (mimeType.startsWith("model/") || mimeType === "application/vnd.google-earth.kmz") {
        return {
            classes: "purple-icons material-symbols-outlined",
            materialSymbol: "view_in_ar",
            simpleType: "3d-model",
        };
    }

    if (mimeType.startsWith("text/")) {
        return {
            classes: "white-icons material-symbols",
            materialSymbol: "description",
            simpleType: "text",
        };
    }

    // Default fallback
    return {
        classes: "lightgray-icons material-symbols",
        materialSymbol: "description",
        simpleType: "blob",
    };
}
