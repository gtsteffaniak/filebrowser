/**
 * Pure helpers for resolving external asset references (textures, .mtl files,
 * glTF buffers/images) emitted by 3D loaders into FileBrowser file paths.
 * Kept free of store/DOM imports so they can be unit tested directly.
 */

const EXTERNAL_URL_RE = /^https?:\/\//i;
const WINDOWS_DRIVE_RE = /^[a-zA-Z]:/;
const API_FILE_URL_RE = /\/api\/(media\/stream|resources\/(view|download))\?/;
// glTF external buffers (.bin) always sit beside the model file, never in a textures folder.
const GLTF_BUFFER_RE = /\.bin$/i;

function safeDecode(value) {
  try {
    return decodeURIComponent(value);
  } catch {
    return value;
  }
}

function parentDirOf(filePath) {
  // Viewer paths can carry a trailing slash (e.g. "/models/ship.glb/"). Strip it first, or the
  // "parent" becomes the model file itself and every relative texture resolves under it.
  const clean = filePath.replace(/\/+$/, "");
  const index = clean.lastIndexOf("/");
  return index <= 0 ? "" : clean.slice(0, index);
}

function readFileParam(url) {
  const queryIndex = url.indexOf("?");
  if (queryIndex === -1) {
    return null;
  }
  const query = url.slice(queryIndex + 1).split("#")[0];
  return new URLSearchParams(query).get("file");
}

/**
 * Resolve a bare filename (no directory part) against the model's directory.
 * Uses the sibling listing when known; otherwise falls back to `textures/`.
 */
function resolveBareFilename(filename, modelDir, parentDirItems) {
  if (GLTF_BUFFER_RE.test(filename)) {
    // Buffers resolve beside the model even when the sibling listing is unavailable.
    return `${modelDir}/${filename}`;
  }
  if (!Array.isArray(parentDirItems)) {
    return `${modelDir}/textures/${filename}`;
  }
  if (parentDirItems.some((item) => item.name === filename && item.type !== "directory")) {
    return `${modelDir}/${filename}`;
  }
  const texturesDir = parentDirItems.find(
    (item) => item.type === "directory" && item.name.toLowerCase() === "textures",
  );
  if (texturesDir) {
    return `${modelDir}/${texturesDir.name}/${filename}`;
  }
  return `${modelDir}/${filename}`;
}

/**
 * Map a loader-emitted URL to the FileBrowser file path it refers to.
 *
 * @param {string} url Reference as emitted by the loader (may be encoded).
 * @param {{ modelPath?: string, parentDirItems?: Array<{name: string, type?: string}> }} [context]
 * @returns {string|null} Absolute file path inside the source, or null when the
 *   URL must not be rewritten (blob:, data:, external http(s), or unresolvable).
 */
export function resolveAssetFilePath(url, { modelPath = "", parentDirItems } = {}) {
  if (!url || url.startsWith("blob:") || url.startsWith("data:")) {
    return null;
  }

  if (API_FILE_URL_RE.test(url)) {
    const file = readFileParam(url);
    if (!file) {
      return null;
    }
    return file.startsWith("/") ? file : `/${file}`;
  }

  const cleanUrl = safeDecode(url.split(/[?#]/)[0]);
  let ref;
  if (cleanUrl.includes("/api/resources/")) {
    ref = cleanUrl.split("/api/resources/").pop() || "";
  } else if (EXTERNAL_URL_RE.test(cleanUrl)) {
    return null;
  } else {
    ref = cleanUrl;
  }

  // Windows exporters (FBX, OBJ/MTL, 3DS) often embed absolute drive paths such as
  // C:\art\diffuse.png. Those directories never exist on the server, so only the
  // filename is meaningful.
  ref = ref.replace(/\\/g, "/");
  const isDriveAbsolute = WINDOWS_DRIVE_RE.test(ref);
  if (isDriveAbsolute) {
    ref = ref.replace(WINDOWS_DRIVE_RE, "");
  }

  // A leading slash marks a source-root-relative path (loaders such as three.js expand external
  // references like "/models/mesh.bin" this way). It must not be re-anchored under the model dir.
  const isSourceRootRelative = ref.startsWith("/") && !isDriveAbsolute;
  ref = ref.replace(/^\/+/, "");
  const filename = ref.split("/").filter(Boolean).pop();
  if (!filename) {
    return null;
  }

  const modelDir = parentDirOf(modelPath);
  if (isDriveAbsolute || !ref.includes("/")) {
    return resolveBareFilename(filename, modelDir, parentDirItems);
  }

  if (isSourceRootRelative) {
    return resolveRelativeTo("/", ref);
  }

  return resolveRelativeTo(modelPath, ref);
}

function resolveRelativeTo(baseFilePath, relativeRef) {
  const parts = `${parentDirOf(baseFilePath)}/${relativeRef}`.split("/").filter(Boolean);
  const resolved = [];
  for (const part of parts) {
    if (part === ".") {
      continue;
    }
    if (part === "..") {
      resolved.pop();
      continue;
    }
    resolved.push(part);
  }
  return `/${resolved.join("/")}`;
}

/**
 * True when a sibling file with this name exists. When the listing is unknown,
 * returns true so the caller still attempts the request.
 */
export function hasSiblingFile(parentDirItems, name) {
  if (!Array.isArray(parentDirItems)) {
    return true;
  }
  const lower = name.toLowerCase();
  return parentDirItems.some(
    (item) => item.type !== "directory" && item.name?.toLowerCase() === lower,
  );
}
