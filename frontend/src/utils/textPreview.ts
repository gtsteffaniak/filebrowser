import { state } from "@/store/index.ts";
import type { TextPreviewPayload } from "@/utils/textPreviewRender.ts";

// Text previews are small (about 16KB), so the frontend keeps a bounded in-memory cache.
// Each key includes the modified time, so a changed file is fetched again on next view.
const MAX_CACHE_ENTRIES = 200;
const cache = new Map<string, Promise<TextPreviewPayload>>();

/**
 * Derives the text preview URL from an image preview thumbnail URL. Works for both
 * /api/resources/preview and /public/api/resources/preview, so auth and share params carry over.
 */
export function textPreviewUrlFor(thumbnailUrl: string): string {
  return thumbnailUrl.replace(/\/resources\/preview\?/, "/resources/preview/text?");
}

/**
 * Lowercase file extension (no dot) of the file a text preview URL points at, or null.
 * Used to pick a highlight.js language.
 */
export function languageFromPreviewUrl(url: string): string | null {
  const path = new URL(url, window.location.href).searchParams.get("path") ?? "";
  const name = path.slice(path.lastIndexOf("/") + 1);
  // A leading dot marks a dotfile (".env"), which has no extension.
  const dot = name.lastIndexOf(".");
  if (dot <= 0) return null;
  return name.slice(dot + 1).toLowerCase();
}

async function requestTextPreview(url: string): Promise<TextPreviewPayload> {
  const res = await fetch(url, {
    credentials: "same-origin",
    headers: { sessionId: state.sessionId },
  });
  if (!res.ok) {
    throw new Error(`Text preview request failed: ${res.status}`);
  }
  return (await res.json()) as TextPreviewPayload;
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

/** Test helper: clears the in-memory text preview cache. */
export function clearTextPreviewCache(): void {
  cache.clear();
}
