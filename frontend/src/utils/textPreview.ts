import { state } from "@/store/index.ts";
import { globalVars } from "@/utils/constants.js";
import type { TextPreviewPayload } from "@/utils/textPreviewRender.ts";

// Text previews are small (about 16KB), so the frontend keeps a bounded in-memory cache.
// Each key includes the modified time, so a changed file is fetched again on next view.
const MAX_CACHE_ENTRIES = 200;
const cache = new Map<string, Promise<TextPreviewPayload>>();

/** Tiles narrower than this show an indicator instead of rendered markdown/HTML/CSV. */
export const MIN_TEXT_PREVIEW_WIDTH_PX = 60;

/**
 * Lowercase file extension (no dot) of the file a text preview URL points at, or null.
 * Used to pick a highlight.js language.
 */
export function languageFromPreviewUrl(url: string): string | null {
  const path = new URL(url, window.location.href).searchParams.get("path") ?? "";
  return languageFromPath(path);
}

/** Lowercase file extension (no dot) of a path, or null. A leading dot marks a dotfile. */
export function languageFromPath(path: string | null | undefined): string | null {
  if (!path) return null;
  const name = path.slice(path.lastIndexOf("/") + 1);
  const dot = name.lastIndexOf(".");
  if (dot <= 0) return null;
  return name.slice(dot + 1).toLowerCase();
}

/**
 * Every extension the text preview endpoint supports, from the backend's kind map.
 * Built once per page rather than once per listing item.
 */
let textPreviewExtSet: Set<string> | null = null;
export function getTextPreviewExtensions(): Set<string> {
  if (textPreviewExtSet === null) {
    const groups = (globalVars.textPreviewExtensions ?? {}) as Record<string, string[]>;
    textPreviewExtSet = new Set(Object.values(groups).flat());
  }
  return textPreviewExtSet;
}

/** True when the filename has an extension the text preview endpoint supports. */
export function isTextPreviewableName(filename: string | null | undefined): boolean {
  const ext = languageFromPath(filename);
  return ext !== null && getTextPreviewExtensions().has(ext);
}

// Text fetches are queued so a folder full of text files does not fire dozens of
// requests at once. Mirrors previewRequests.ts, but returns parsed JSON.
const MAX_CONCURRENT_LOADS = 8;
let activeLoads = 0;
const queue: Array<() => void> = [];

function runQueued<T>(task: () => Promise<T>): Promise<T> {
  return new Promise<T>((resolve, reject) => {
    const start = () => {
      activeLoads++;
      task()
        .then(resolve, reject)
        .finally(() => {
          activeLoads--;
          const next = queue.shift();
          if (next) next();
        });
    };
    if (activeLoads < MAX_CONCURRENT_LOADS) {
      start();
    } else {
      queue.push(start);
    }
  });
}

function abortError(): DOMException {
  return new DOMException("Aborted", "AbortError");
}

async function requestTextPreview(url: string, signal?: AbortSignal): Promise<TextPreviewPayload> {
  if (signal?.aborted) {
    throw abortError();
  }
  return runQueued(async () => {
    if (signal?.aborted) {
      throw abortError();
    }
    const res = await fetch(url, {
      credentials: "same-origin",
      headers: { sessionId: state.sessionId },
      ...(signal && { signal }),
    });
    if (!res.ok) {
      throw new Error(`Text preview request failed: ${res.status}`);
    }
    return (await res.json()) as TextPreviewPayload;
  });
}

/**
 * Fetches a text preview, reusing a cached result for the same URL and modified time.
 * Failed requests are not cached, so a later view can retry.
 */
export function fetchTextPreview(url: string, modified: string | null): Promise<TextPreviewPayload> {
  const key = `${url}|${modified ?? ""}`;
  const cached = cache.get(key);
  if (cached !== undefined) {
    // Re-insert to mark as most recently used.
    cache.delete(key);
    cache.set(key, cached);
    return cached;
  }
  const request = requestTextPreview(url);
  cache.set(key, request);
  if (cache.size > MAX_CACHE_ENTRIES) {
    const oldest = cache.keys().next().value;
    if (oldest !== undefined) {
      cache.delete(oldest);
    }
  }
  request.catch(() => {
    if (cache.get(key) === request) {
      cache.delete(key);
    }
  });
  return request;
}

// Final sanitized/rendered HTML is the expensive part of a text preview. Virtualization
// unmounts off-window tiles and the popup unmounts on mouse-out, so without this every
// scroll-back and hover would re-run markdown parsing + DOMPurify.
const MAX_RENDERED_ENTRIES = 200;
const renderedCache = new Map<string, string>();

/** Cache key for a rendered text preview, matching fetchTextPreview's key. */
export function textPreviewCacheKey(url: string, modified: string | null): string {
  return `${url}|${modified ?? ""}`;
}

/** Returns the cached rendered HTML for a key, marking it as most recently used. */
export function getRenderedTextPreview(key: string): string | undefined {
  const value = renderedCache.get(key);
  if (value !== undefined) {
    renderedCache.delete(key);
    renderedCache.set(key, value);
  }
  return value;
}

/** Stores rendered HTML for a key, evicting the least recently used entry past the limit. */
export function setRenderedTextPreview(key: string, html: string): void {
  renderedCache.delete(key);
  renderedCache.set(key, html);
  if (renderedCache.size > MAX_RENDERED_ENTRIES) {
    const oldest = renderedCache.keys().next().value;
    if (oldest !== undefined) {
      renderedCache.delete(oldest);
    }
  }
}

/** Test helper: clears the in-memory text preview caches. */
export function clearTextPreviewCache(): void {
  cache.clear();
  renderedCache.clear();
}
