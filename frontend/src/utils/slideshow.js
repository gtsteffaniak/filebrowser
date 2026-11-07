import * as toolsApi from "@/api/tools.js";
import { resourcesApi } from "@/api";
import { getTypeInfo, getTypeInfoFromExt, isRawImageMimeType } from "@/utils/mimetype.js";
import { removeTrailingSlash } from "@/utils/url.js";
import { getters, state } from "@/store";
import { globalVars } from "@/utils/constants";
import { joinPath } from "@/utils/url.js";
import { slideshowConfig } from "@/utils/slideshowConfig.js";

const RAW_EXT = /\.(cr2|cr3|nef|nrw|arw|dng|orf|rw2|raf|pef|srw|raw)$/i;
const HEIC_EXT = /\.(heic|heif)$/i;

// Classifies a file for the slideshow. Returns { kind, preview } or null when it can't be shown.
// preview is "raw" or "heic" when the image must be fetched as a server-generated preview,
// matching how Preview.vue builds its URL. A preview that fails at load time is skipped.
export function mediaInfo(name, mimeType = "") {
  if (!name) return null;
  if (getters.fileViewingDisabled(name)) return null;

  if (isRawImageMimeType(mimeType) || RAW_EXT.test(name)) {
    return { kind: "image", preview: "raw" };
  }

  const isHeic = HEIC_EXT.test(name) || mimeType === "image/heic" || mimeType === "image/heif";
  if (isHeic) {
    if (state.isSafari) return { kind: "image", preview: null };
    if (globalVars.ffmpegAvailable && globalVars.enableHeicConversion) {
      return { kind: "image", preview: "heic" };
    }
    return null;
  }

  const simple = mimeType ? getTypeInfo(mimeType).simpleType : getTypeInfoFromExt(name).simpleType;
  if (simple === "image") return { kind: "image", preview: null };
  if (simple === "video") return { kind: "video", preview: null };
  return null;
}

// Videos are only part of the slideshow when the user turns "include videos" on.
function isIncluded(info) {
  return !!info && (info.kind === "image" || slideshowConfig.includeVideos);
}

export function countMediaInListing(listing) {
  if (!Array.isArray(listing)) return 0;
  return listing.filter((item) => item && item.type !== "directory" && isIncluded(mediaInfo(item.name, item.type))).length;
}

function nameFromPath(path) {
  const parts = String(path || "").split("/").filter(Boolean);
  return parts[parts.length - 1] || "";
}

// Builds slides for the current folder from the listing already in memory.
export function slidesFromListing(listing, source) {
  return (listing || [])
    .filter((item) => item && item.type !== "directory")
    .map((item) => {
      const info = mediaInfo(item.name, item.type);
      if (!isIncluded(info)) return null;
      return {
        key: `${item.source || source}:${item.path}`,
        source: item.source || source,
        path: item.path,
        name: item.name,
        kind: info.kind,
        preview: info.preview,
        mime: item.type || "",
        modified: item.modified || "",
      };
    })
    .filter(Boolean);
}

// Recursively finds images and videos under a folder via the search API.
// Results are merged, sorted by path, and capped at maxItems.
export async function slidesFromSearch(source, dirPath, maxItems) {
  const perSourceScopes = [{ source, path: dirPath }];
  const queries = slideshowConfig.includeVideos ? ["type:image", "type:video"] : ["type:image"];
  const responses = await Promise.all(
    queries.map((query) =>
      toolsApi.search(dirPath, [source], query, false, { limit: maxItems, perSourceScopes })
    )
  );

  const seen = new Set();
  const slides = [];
  for (const results of responses) {
    if (!Array.isArray(results)) continue;
    for (const result of results) {
      const src = result.source || source;
      const name = nameFromPath(result.path);
      const mime = String(result.type || "").includes("/") ? result.type : "";
      const fullPath = joinPath(dirPath, result.path);
      const key = `${src}:${fullPath}`;
      if (seen.has(key)) continue;
      seen.add(key);
      const info = mediaInfo(name, mime);
      if (!isIncluded(info)) continue;
      // Result paths are relative to the scope that was searched, so join the scope back on.
      slides.push({
        key,
        source: src,
        path: fullPath,
        name,
        kind: info.kind,
        preview: info.preview,
        mime,
        modified: result.modified || "",
      });
    }
  }

  slides.sort((a, b) => a.path.localeCompare(b.path, undefined, { numeric: true, sensitivity: "base" }));
  const truncated = slides.length > maxItems;
  return { slides: slides.slice(0, maxItems), truncated };
}

// Fisher–Yates shuffle, returns a new array.
export function shuffled(items) {
  const out = items.slice();
  for (let i = out.length - 1; i > 0; i--) {
    const j = Math.floor(Math.random() * (i + 1));
    [out[i], out[j]] = [out[j], out[i]];
  }
  return out;
}

// Resolves the URL for a slide. Mirrors the URL logic in Preview.vue's `raw`.
export function slideUrl(slide, viewToken) {
  const isShare = getters.isShare();

  if (slide.kind === "image" && slide.preview) {
    if (isShare) {
      if (!viewToken) return null;
      return resourcesApi.getPreviewURLPublic(removeTrailingSlash(slide.path), "original", viewToken);
    }
    return `${resourcesApi.getPreviewURL(slide.source, slide.path, slide.modified)}&size=original`;
  }

  if (!viewToken) return null;
  const shareInfo = isShare ? state.shareInfo : null;
  return resourcesApi.getViewURL(slide.source, slide.path, viewToken, shareInfo, false, slide.mime || slide.name);
}
