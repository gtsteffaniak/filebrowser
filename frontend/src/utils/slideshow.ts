import * as toolsApi from "@/api/tools.js";
import { resourcesApi } from "@/api/index.js";
import { getTypeInfo, getTypeInfoFromExt, isRawImageMimeType } from "@/utils/mimetype.js";
import { joinPath, removeTrailingSlash } from "@/utils/url.js";
import { getters, state } from "@/store/index.ts";
import { globalVars } from "@/utils/constants.ts";
import { slideshowConfig } from "@/utils/config/slideshowConfig.ts";
import type { ShareInfoObject } from "@/store/types.ts";

const RAW_EXT = /\.(cr2|cr3|nef|nrw|arw|dng|orf|rw2|raf|pef|srw|raw)$/i;
const HEIC_EXT = /\.(heic|heif)$/i;

export type SlideshowMediaKind = "image" | "video";
export type SlideshowPreviewMode = "raw" | "heic" | null;

export interface SlideshowMediaInfo {
  kind: SlideshowMediaKind;
  preview: SlideshowPreviewMode;
}

export interface SlideshowSlide {
  key: string;
  source: string;
  path: string;
  name: string;
  kind: SlideshowMediaKind;
  preview: SlideshowPreviewMode;
  mime: string;
  modified: string;
}

export interface SlideshowListingItem {
  name: string;
  path: string;
  type?: string;
  source?: string;
  modified?: string;
}

interface SearchScopeResult {
  source?: string;
  path: string;
  type?: string;
  modified?: string;
}

// Classifies a file for the slideshow. Returns null when it can't be shown.
// preview is "raw" or "heic" when the image must be fetched as a server-generated preview,
// matching how Preview.vue builds its URL. A preview that fails at load time is skipped.
export function mediaInfo(name: string, mimeType = ""): SlideshowMediaInfo | null {
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

function isIncluded(info: SlideshowMediaInfo | null): info is SlideshowMediaInfo {
  return !!info && (info.kind === "image" || slideshowConfig.includeVideos);
}

export function countMediaInListing(listing: SlideshowListingItem[] | null | undefined): number {
  if (!Array.isArray(listing)) return 0;
  return listing.filter(
    (item) => item && item.type !== "directory" && isIncluded(mediaInfo(item.name, item.type ?? "")),
  ).length;
}

function nameFromPath(path: string): string {
  const parts = String(path || "")
    .split("/")
    .filter(Boolean);
  return parts.at(-1) || "";
}

/** Builds slides for the current folder from the listing already in memory. */
export function slidesFromListing(
  listing: SlideshowListingItem[] | null | undefined,
  source: string,
): SlideshowSlide[] {
  return (listing || [])
    .filter((item) => item && item.type !== "directory")
    .map((item): SlideshowSlide | null => {
      const info = mediaInfo(item.name, item.type ?? "");
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
    .filter((slide): slide is SlideshowSlide => slide !== null);
}

export interface SlidesFromSearchResult {
  slides: SlideshowSlide[];
  truncated: boolean;
}

/** Recursively finds images and videos under a folder via the search API. */
export async function slidesFromSearch(
  source: string,
  dirPath: string,
  maxItems: number,
): Promise<SlidesFromSearchResult> {
  const perSourceScopes = [{ source, path: dirPath }];
  const queries = slideshowConfig.includeVideos ? ["type:image", "type:video"] : ["type:image"];
  const responses = await Promise.all(
    queries.map((query) =>
      toolsApi.search(dirPath, [source], query, false, { limit: maxItems, perSourceScopes }),
    ),
  );

  const seen = new Set<string>();
  const slides: SlideshowSlide[] = [];
  let truncated = false;
  for (const results of responses) {
    if (!Array.isArray(results)) continue;
    if (results.length >= maxItems) {
      truncated = true;
    }
    for (const result of results as SearchScopeResult[]) {
      const src = result.source || source;
      const name = nameFromPath(result.path);
      const mime = String(result.type || "").includes("/") ? (result.type ?? "") : "";
      const fullPath = joinPath(dirPath, result.path);
      const key = `${src}:${fullPath}`;
      if (seen.has(key)) continue;
      seen.add(key);
      const info = mediaInfo(name, mime);
      if (!isIncluded(info)) continue;
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
  if (slides.length > maxItems) {
    truncated = true;
  }
  return { slides: slides.slice(0, maxItems), truncated };
}

/** Fisher–Yates shuffle; returns a new array. */
export function shuffled<T>(items: T[]): T[] {
  const out = items.slice();
  for (let i = out.length - 1; i > 0; i--) {
    const j = Math.floor(Math.random() * (i + 1));
    const atI = out[i];
    const atJ = out[j];
    if (atI === undefined || atJ === undefined) continue;
    out[i] = atJ;
    out[j] = atI;
  }
  return out;
}

/** Resolves the URL for a slide. Mirrors the URL logic in Preview.vue's `raw`. */
export function slideUrl(slide: SlideshowSlide, viewToken: string | null | undefined): string | null {
  const isShare = getters.isShare();

  if (slide.kind === "image" && slide.preview) {
    if (isShare) {
      if (!viewToken) return null;
      return resourcesApi.getPreviewURLPublic(removeTrailingSlash(slide.path), "original", viewToken);
    }
    return `${resourcesApi.getPreviewURL(slide.source, slide.path, slide.modified)}&size=original`;
  }

  if (!viewToken) return null;
  const shareInfo: ShareInfoObject | null = isShare ? state.shareInfo : null;
  return resourcesApi.getViewURL(slide.source, slide.path, viewToken, shareInfo, false, slide.mime || slide.name);
}
