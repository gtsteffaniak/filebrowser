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

export function isRawImageMimeType(mimeType) {
  return typeof mimeType === "string" && RAW_IMAGE_MIME_TYPES.has(mimeType);
}

// Extensions browsers render natively in <img> (no server-side conversion).
const IMAGE_FILE_EXTENSIONS = new Set([
  ".jpg", ".jpeg", ".png", ".gif", ".webp", ".svg", ".bmp", ".ico", ".avif",
  ".heic", ".heif",
]);

export function isImageFilePath(path) {
  if (typeof path !== "string" || path === "") {
    return false;
  }
  const withoutSuffix = path.split("?")[0].split("#")[0];
  const dot = withoutSuffix.lastIndexOf(".");
  if (dot === -1) {
    return false;
  }
  return IMAGE_FILE_EXTENSIONS.has(withoutSuffix.slice(dot).toLowerCase());
}

const RICH_TEXT_PREVIEW_MIME_TYPES = new Set([
  "text/markdown",
  "text/x-markdown",
  "text/html",
  "application/xhtml+xml",
]);

export function isRichTextPreviewMimeType(mimeType) {
  return typeof mimeType === "string" && RICH_TEXT_PREVIEW_MIME_TYPES.has(mimeType);
}

export function isHtmlMimeType(mimeType) {
  return mimeType === "text/html" || mimeType === "application/xhtml+xml";
}

// Icon lookup table, this is keyed by mimeType and `getTypeInfoFromExt()`
// (which is by extension, where no mimeType exists)
// Each entry lists mimeType, extensions, and/or a prefix (as a fallback)
// and earlier entries take priority over later ones, so the order matters (basically that stills the same as before)
// Icon.vue uses only mimetype (with getTypeInfo)
const TYPE_TABLE = [
  {
    mimeTypes: ["directory", "application/vnd.google-apps.folder"],
    classes: "primary-icons material-symbols",
    materialSymbol: "folder",
    simpleType: "directory",
  },
  {
    mimeTypes: [
      "application/epub+zip", "application/x-mobipocket-ebook", "application/vnd.amazon.ebook",
      "application/x-fictionbook+xml", "application/x-fb2", "application/x-cbr", "application/x-cbz",
      "application/x-cb7", "application/x-cbt", "application/vnd.comicbook+zip",
      "application/vnd.comicbook-rar", "application/x-kindle", "application/x-azw",
    ],
    extensions: [".epub", ".mobi", ".azw", ".azw3", ".fb2", ".cbr", ".cbz", ".cb7", ".cbt"],
    classes: "brown-icons material-symbols-outlined",
    materialSymbol: "menu_book",
    simpleType: "ebook",
  },
  {
    mimeTypes: ["image/gif"],
    extensions: [".gif"],
    classes: "coral-icons material-symbols-outlined",
    materialSymbol: "gif",
    simpleType: "image",
  },
  {
    prefix: "image/",
    extensions: [".jpg", ".jpeg", ".png", ".webp", ".bmp", ".ico", ".avif", ".heic", ".heif", ".tif", ".tiff", ".svg"],
    classes: "coral-icons material-symbols-outlined",
    materialSymbol: "image",
    simpleType: "image",
  },
  {
    mimeTypes: ["application/vnd.google-apps.audio"],
    prefix: "audio/",
    extensions: [".mp3", ".wav", ".ogg", ".flac", ".aac", ".m4a", ".wma", ".opus"],
    classes: "plum-icons material-symbols-outlined",
    materialSymbol: "audiotrack",
    simpleType: "audio",
  },
  {
    mimeTypes: ["application/vnd.google-apps.video"],
    prefix: "video/",
    extensions: [".mp4", ".mkv", ".mov", ".avi", ".webm", ".flv", ".wmv", ".m4v"],
    classes: "skyblue-icons material-symbols-outlined",
    materialSymbol: "movie",
    simpleType: "video",
  },
  {
    mimeTypes: ["file_download"],
    classes: "material-symbols",
    materialSymbol: "file_download",
    simpleType: "file_download",
  },
  {
    mimeTypes: ["application/vnd.oasis.opendocument.formula-template"],
    prefix: "font/",
    extensions: [".ttf", ".otf", ".woff", ".woff2", ".eot"],
    classes: "gray-icons material-symbols-outlined",
    materialSymbol: "format_color_text",
    simpleType: "font",
  },
  {
    mimeTypes: [
      "application/zip", "application/x-7z-compressed", "application/x-bzip",
      "application/x-rar-compressed", "application/x-tar", "application/gzip",
      "application/x-xz", "application/x-zip-compressed", "application/x-compressed",
      "application/x-gzip",
    ],
    extensions: [".zip", ".7z", ".bz2", ".rar", ".tar", ".gz", ".xz", ".tgz"],
    classes: "tan-icons material-symbols",
    materialSymbol: "archive",
    simpleType: "archive",
  },
  {
    mimeTypes: ["application/pdf"],
    extensions: [".pdf"],
    classes: "red-icons material-symbols-outlined",
    materialSymbol: "picture_as_pdf",
    simpleType: "document",
  },
  {
    // documents: doc, docx, rtf, odt
    mimeTypes: [
      "application/msword", "application/vnd.openxmlformats-officedocument.wordprocessingml.document",
      "application/vnd.google-apps.document", "text/richtext", "application/vnd.oasis.opendocument.text",
    ],
    extensions: [".doc", ".docx", ".rtf", ".odt"],
    classes: "deep-blue-icons material-symbols-outlined",
    materialSymbol: "docs",
    simpleType: "document",
  },
  {
    // spreadsheets: xls, xlsx, ods, csv
    mimeTypes: [
      "application/vnd.ms-excel", "application/vnd.openxmlformats-officedocument.spreadsheetml.sheet",
      "application/vnd.google-apps.spreadsheet", "application/excel",
      "application/vnd.oasis.opendocument.spreadsheet", "text/csv",
    ],
    extensions: [".xls", ".xlsx", ".ods", ".csv"],
    classes: "green-icons material-symbols-outlined",
    materialSymbol: "table",
    simpleType: "document",
  },
  {
    // presentations: ppt, pptx, odp
    mimeTypes: [
      "application/vnd.ms-powerpoint", "application/vnd.openxmlformats-officedocument.presentationml.presentation",
      "application/vnd.google-apps.presentation", "application/mspowerpoint",
      "application/vnd.oasis.opendocument.presentation",
    ],
    extensions: [".ppt", ".pptx", ".odp"],
    classes: "red-icons material-symbols-outlined",
    materialSymbol: "slideshow",
    simpleType: "document",
  },
  {
    mimeTypes: ["application/json", "application/json5"],
    extensions: [".json", ".json5"],
    classes: "brown-icons material-symbols-outlined",
    materialSymbol: "file_json",
    simpleType: "text",
  },
  {
    mimeTypes: ["application/javascript", "text/javascript"],
    extensions: [".js", ".mjs", ".cjs"],
    classes: "yellow-icons material-symbols-outlined",
    materialSymbol: "javascript",
    simpleType: "text",
  },
  {
    mimeTypes: ["text/vue"],
    extensions: [".vue"],
    classes: "light-green-icons material-symbols-outlined",
    materialSymbol: "code",
    simpleType: "text",
  },
  {
    mimeTypes: ["application/x-python", "application/vnd.google-apps.sites", "text/x-scriptphyton"],
    extensions: [".py"],
    classes: "yellow-icons material-symbols-outlined",
    materialSymbol: "code",
    simpleType: "text",
  },
  {
    mimeTypes: ["text/markdown", "text/x-markdown", "text/x-rmarkdown", "text/x-quarto"],
    extensions: [".md", ".markdown", ".rmd", ".qmd"],
    classes: "skyblue-icons material-symbols-outlined",
    materialSymbol: "markdown",
    simpleType: "text",
  },
  {
    mimeTypes: ["text/html", "application/xhtml+xml"],
    extensions: [".html", ".htm", ".xhtml"],
    classes: "orange-icons material-symbols-outlined",
    materialSymbol: "html",
    simpleType: "text",
  },
  {
    mimeTypes: ["text/xml"],
    extensions: [".xml"],
    classes: "deep-orange-icons material-symbols-outlined",
    materialSymbol: "code_xml",
    simpleType: "text",
  },
  {
    mimeTypes: ["text/css", "text/x-scss", "text/x-sass"],
    extensions: [".css", ".scss", ".sass"],
    classes: "lightblue-icons material-symbols-outlined",
    materialSymbol: "css",
    simpleType: "text",
  },
  {
    mimeTypes: ["text/tab-separated-values"],
    extensions: [".tsv"],
    classes: "light-green-icons material-symbols-outlined",
    materialSymbol: "tsv",
    simpleType: "text",
  },
  {
    mimeTypes: ["text/x-java-source"],
    extensions: [".java"],
    classes: "brown-icons material-symbols-outlined",
    materialSymbol: "local_cafe",
    simpleType: "text",
  },
  {
    // bash, sh
    mimeTypes: ["text/x-scriptsh", "text/x-shellscript"],
    extensions: [".sh", ".bash"],
    classes: "light-green-icons material-symbols-outlined",
    materialSymbol: "terminal_2",
    simpleType: "text",
  },
  {
    mimeTypes: ["text/x-lua"],
    extensions: [".lua"],
    classes: "blue-icons material-symbols-outlined",
    materialSymbol: "blur_circular",
    simpleType: "text",
  },
  {
    mimeTypes: ["text/x-c"],
    extensions: [".c", ".h"],
    classes: "blue-icons material-symbols-outlined",
    materialSymbol: "copyright",
    simpleType: "text",
  },
  {
    mimeTypes: ["text/x-rust", "text/rust"],
    extensions: [".rs"],
    classes: "deep-orange-icons material-symbols-outlined",
    materialSymbol: "game_button_r",
    simpleType: "text",
  },
  {
    mimeTypes: ["text/x-csharp", "text/csharp"],
    extensions: [".cs"],
    classes: "purple-icons material-symbols-outlined",
    materialSymbol: "tag",
    simpleType: "text",
  },
  {
    // Subtitle files: srt, vtt, ass, ssa
    mimeTypes: ["text/subtitle-srt", "text/subtitle-ass", "text/subtitle-vtt", "text/subtitle-ssa"],
    extensions: [".srt", ".ass", ".vtt", ".ssa"],
    classes: "blue-icons material-symbols-outlined",
    materialSymbol: "closed_caption",
    simpleType: "text",
  },
  {
    // Playlist files: m3u, m3u8, pls, xspf
    mimeTypes: ["text/x-mpegurl", "text/x-mpegURL", "text/x-scpls", "application/xspf+xml"],
    extensions: [".m3u", ".m3u8", ".pls", ".xspf"],
    classes: "coral-icons material-symbols-outlined",
    materialSymbol: "playlist_play",
    simpleType: "text",
  },
  {
    // Lyric file: .lrc
    mimeTypes: ["text/lyrics"],
    extensions: [".lrc"],
    classes: "coral-icons material-symbols-outlined",
    materialSymbol: "lyrics",
    simpleType: "text",
  },
  {
    // Contact file: vcf
    mimeTypes: ["text/x-vcard"],
    extensions: [".vcf"],
    classes: "deep-orange-icons material-symbols-outlined",
    materialSymbol: "contacts",
    simpleType: "text",
  },
  {
    // config: ini, conf
    mimeTypes: ["text/config-file", "text/ini"],
    extensions: [".ini", ".conf"],
    classes: "tan-icons material-symbols-outlined",
    materialSymbol: "settings",
    simpleType: "text",
  },
  {
    mimeTypes: ["text/x-go"],
    extensions: [".go"],
    classes: "skyblue-icons material-symbols-outlined",
    materialSymbol: "code",
    simpleType: "text",
  },
  {
    mimeTypes: ["text/x-kotlin"],
    extensions: [".kt"],
    classes: "orange-icons material-symbols-outlined",
    materialSymbol: "code",
    simpleType: "text",
  },
  {
    // Typescript: ts, tsx
    mimeTypes: ["text/x-typescript"],
    extensions: [".ts", ".tsx"],
    classes: "deep-blue-icons material-symbols-outlined",
    materialSymbol: "title",
    simpleType: "text",
  },
  {
    // Perl: pm, pl
    mimeTypes: ["text/x-scriptperl", "text/x-scriptperl-module"],
    extensions: [".pm", ".pl"],
    classes: "deep-blue-icons material-symbols-outlined",
    materialSymbol: "chess_knight",
    simpleType: "text",
  },
  {
    // Pascal: pas, p, pp
    mimeTypes: ["text/x-pascal", "text/pascal"],
    extensions: [".pas", ".p", ".pp"],
    classes: "deep-blue-icons material-symbols-outlined",
    materialSymbol: "local_parking",
    simpleType: "text",
  },
  {
    mimeTypes: ["text/x-zig", "text/zig"],
    extensions: [".zig"],
    classes: "yellow-icons material-symbols-outlined",
    materialSymbol: "electric_bolt",
    simpleType: "text",
  },
  {
    // Elixir: ex, exs
    mimeTypes: ["text/x-elixir"],
    extensions: [".ex", ".exs"],
    classes: "purple-icons material-symbols-outlined",
    materialSymbol: "water_drop",
    simpleType: "text",
  },
  {
    // Nixos: nix
    mimeTypes: ["text/nix-lang"],
    extensions: [".nix"],
    classes: "blue-icons material-symbols-outlined",
    materialSymbol: "snowflake",
    simpleType: "text",
  },
  {
    // calendar: ics
    mimeTypes: ["text/calendar"],
    extensions: [".ics"],
    classes: "tan-icons material-symbols-outlined",
    materialSymbol: "calendar_month",
    simpleType: "text",
  },
  {
    // Temporary files: tmp, temp
    mimeTypes: ["text/tmp"],
    extensions: [".tmp", ".temp"],
    classes: "gray-icons material-symbols-outlined",
    materialSymbol: "hourglass",
    simpleType: "text",
  },
  {
    mimeTypes: [
      "application/octet-stream", "application/x-executable", "application/mac-binary",
      "application/vnd.google-apps.unknown", "application/x-msdownload", "application/x-application",
      "application/x-efi", "application/x-installer", "application/vnd.microsoft.portable-executable",
    ],
    classes: "gray-icons material-symbols-outlined",
    materialSymbol: "memory",
    simpleType: "binary",
  },
  {
    // Android: APK
    mimeTypes: ["application/vnd.android.package-archive"],
    extensions: [".apk"],
    classes: "light-green-icons material-symbols-outlined",
    materialSymbol: "android",
    simpleType: "archive",
  },
  {
    // Images: dmg, iso, qcow2, img, cue, vmdk...
    mimeTypes: [
      "application/x-disk-image", "application/x-iso-image", "application/x-apple-diskimage",
      "application/x-cd-image", "application/vnd.efi.iso", "application/x-qcow2",
      "application/x-vmdk", "application/x-qemu-disk", "application/vnd.efi.img",
      "application/x-cue", "application/x-vmdk-disk",
    ],
    extensions: [".dmg", ".iso", ".qcow2", ".img", ".cue", ".vmdk"],
    classes: "lightgray-icons material-symbols",
    materialSymbol: "album",
    simpleType: "binary",
  },
  {
    // backup, bak
    mimeTypes: ["application/backup"],
    extensions: [".bak"],
    classes: "gray-icons material-symbols-outlined",
    materialSymbol: "save",
    simpleType: "text",
  },
  {
    // ruby
    mimeTypes: ["application/x-ruby"],
    extensions: [".rb"],
    classes: "red-icons material-symbols",
    materialSymbol: "diamond",
    simpleType: "text",
  },
  {
    // PHP
    mimeTypes: ["application/x-php"],
    extensions: [".php"],
    classes: "blue-icons material-symbols-outlined",
    materialSymbol: "php",
    simpleType: "text",
  },
  {
    // vector: ps, eps, ai
    mimeTypes: ["application/postscript"],
    extensions: [".ps", ".eps", ".ai"],
    classes: "orange-icons material-symbols-outlined",
    materialSymbol: "format_shapes",
    simpleType: "text",
  },
  {
    // databases: db, sqlite, sql
    mimeTypes: ["application/x-db", "application/sql", "application/vnd.sqlite3"],
    extensions: [".db", ".sql", ".sqlite"],
    classes: "blue-icons material-symbols-outlined",
    materialSymbol: "database",
    simpleType: "text",
  },
  {
    // yaml, yml
    mimeTypes: ["application/yaml"],
    extensions: [".yaml", ".yml"],
    classes: "orange-icons material-symbols-outlined",
    materialSymbol: "data_object",
    simpleType: "text",
  },
  {
    // toml
    mimeTypes: ["application/toml", "text/toml"],
    extensions: [".toml"],
    classes: "red-icons material-symbols-outlined",
    materialSymbol: "developer_mode_tv",
    simpleType: "text",
  },
  {
    // cad: dwg, dxf
    mimeTypes: ["application/acad", "application/dxf"],
    extensions: [".dwg", ".dxf"],
    classes: "red-icons material-symbols-outlined",
    materialSymbol: "architecture",
    simpleType: "binary",
  },
  {
    // map related: geojson, map, kmz, gpx, kml...
    mimeTypes: [
      "application/x-shapefile", "application/geo+json", "application/vnd.google-earth.kml+xml",
      "application/vnd.google-earth.kmz", "application/gpx+xml", "application/x-navimap",
    ],
    extensions: [".geojson", ".kml", ".kmz", ".gpx"],
    classes: "green-icons material-symbols-outlined",
    materialSymbol: "map",
    simpleType: "binary",
  },
  {
    // xcf, figma, fig
    mimeTypes: ["application/x-xcf", "application/x-figma", "application/x-sketch"],
    extensions: [".xcf", ".fig", ".sketch"],
    classes: "plum-icons material-symbols-outlined",
    materialSymbol: "brush",
    simpleType: "binary",
  },
  {
    // powershell (windows): ps1, cmd, bat
    mimeTypes: ["application/x-powershell", "application/x-msdos-program"],
    extensions: [".ps1", ".cmd", ".bat"],
    classes: "deep-blue-icons material-symbols-outlined",
    materialSymbol: "terminal",
    simpleType: "text",
  },
  {
    // flutter: dart
    mimeTypes: ["application/vnd.dart", "text/flutter"],
    extensions: [".dart"],
    classes: "lightblue-icons material-symbols-outlined",
    materialSymbol: "flutter",
    simpleType: "text",
  },
  {
    // assembly: wasm, asm
    mimeTypes: ["application/wasm", "text/x-asm"],
    extensions: [".wasm", ".asm"],
    classes: "deep-blue-icons material-symbols-outlined",
    materialSymbol: "memory",
    simpleType: "text",
  },
  {
    // packages: deb, rpm
    mimeTypes: ["application/x-debian-package", "application/x-newton-compatible-pkg"],
    extensions: [".deb", ".rpm"],
    classes: "brown-icons material-symbols-outlined",
    materialSymbol: "package_2",
    simpleType: "archive",
  },
  {
    // keys: pem, pub
    mimeTypes: ["application/x-x509-ca-cert", "application/vnd.apple.keynote", "application/vnd.ms-publisher"],
    extensions: [".pem", ".pub", ".key"],
    classes: "deep-orange-icons material-symbols-outlined",
    materialSymbol: "key",
    simpleType: "text",
  },
  {
    // certificates: crt, cer
    mimeTypes: ["application/pkix-cert"],
    extensions: [".crt", ".cer"],
    classes: "tan-icons material-symbols-outlined",
    materialSymbol: "license",
    simpleType: "text",
  },
  {
    // torrents: torrent
    mimeTypes: ["application/x-bittorrent"],
    extensions: [".torrent"],
    classes: "light-green-icons material-symbols-outlined",
    materialSymbol: "format_underlined",
    simpleType: "blob",
  },
  {
    mimeTypes: ["invalid_link"],
    classes: "lightgray-icons material-symbols",
    materialSymbol: "link_off",
    simpleType: "invalid_link",
  },
  {
    // 3D model formats
    prefix: "model/",
    extensions: [".glb", ".gltf", ".obj", ".stl", ".fbx", ".3ds", ".blend"],
    classes: "purple-icons material-symbols-outlined",
    materialSymbol: "view_in_ar",
    simpleType: "3d-model",
  },
  {
    // Generic text/* fallback (only reached if nothing more specific matched above)
    prefix: "text/",
    extensions: [".txt"],
    classes: "white-icons material-symbols",
    materialSymbol: "description",
    simpleType: "text",
  },
];

// Absolute fallback when nothing in TYPE_TABLE matches at all.
const DEFAULT_TYPE_INFO = {
  classes: "lightgray-icons material-symbols",
  materialSymbol: "description",
  simpleType: "blob",
};

// Fallback specifically for a missing/empty mimeType (kept distinct from
// DEFAULT_TYPE_INFO to preserve existing callers relying on this exact shape).
const EMPTY_MIME_TYPE_INFO = {
  classes: "material-symbols",
  materialSymbol: "file",
  simpleType: "file",
};

function toTypeInfo(entry) {
  return { classes: entry.classes, materialSymbol: entry.materialSymbol, simpleType: entry.simpleType };
}

// Build lookup indexes once. Earlier TYPE_TABLE entries win ties (mirrors the
// original if/else-if chain, e.g. "image/gif" before the generic "image/").
const MIME_EXACT_INDEX = new Map();
const EXTENSION_INDEX = new Map();
const PREFIX_RULES = [];

for (const entry of TYPE_TABLE) {
  for (const mimeType of entry.mimeTypes || []) {
    if (!MIME_EXACT_INDEX.has(mimeType)) {
      MIME_EXACT_INDEX.set(mimeType, entry);
    }
  }
  for (const ext of entry.extensions || []) {
    if (!EXTENSION_INDEX.has(ext)) {
      EXTENSION_INDEX.set(ext, entry);
    }
  }
  if (entry.prefix) {
    PREFIX_RULES.push(entry);
  }
}

function extensionOf(filename) {
  if (typeof filename !== "string") {
    return "";
  }
  const dot = filename.lastIndexOf(".");
  if (dot === -1) {
    return "";
  }
  return filename.slice(dot).toLowerCase();
}

export function getTypeInfo(mimeType) {
  if (!mimeType) {
    return EMPTY_MIME_TYPE_INFO;
  }

  const exact = MIME_EXACT_INDEX.get(mimeType);
  if (exact) {
    return toTypeInfo(exact);
  }

  for (const rule of PREFIX_RULES) {
    if (mimeType.startsWith(rule.prefix)) {
      return toTypeInfo(rule);
    }
  }

  return DEFAULT_TYPE_INFO;
}

export function getTypeInfoFromExt(filename) {
  const entry = EXTENSION_INDEX.get(extensionOf(filename));
  return entry ? toTypeInfo(entry) : DEFAULT_TYPE_INFO;
}
