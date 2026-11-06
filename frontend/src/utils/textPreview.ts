import { state } from "@/store/index.ts";
import type { TextPreviewPayload } from "@/utils/textPreviewRender.ts";

// Text previews are small (about 16KB), so the frontend keeps a bounded in-memory cache.
// Each key includes the modified time, so a changed file is fetched again on next view.
const MAX_CACHE_ENTRIES = 200;

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
 * Non-text/* mimetypes that text-extension files resolve to (JSON, YAML, SQL, shell, ...).
 * Keep in sync with textPreviewMimeTypes in backend/pkg/indexing/iteminfo/conditions.go.
 * Do not add types shared with binaries (e.g. application/x-msdownload for .exe).
 */
const TEXT_PREVIEW_MIME_TYPES = new Set<string>([
  "application/json",
  "application/yaml",
  "application/x-yaml",
  "application/sql",
  "application/x-sql",
  "application/x-sh",
  "application/x-php",
  "application/x-powershell",
  "application/x-tex",
  "application/toml",
  "application/x-ruby",
]);

/**
 * True when an item's mimetype is text/* or on the text allowlist above. The backend uses
 * the same rule, so an item the listing marks as text is the one the text preview endpoint
 * renders. Binary types such as sqlite, postscript, wasm and backup never reach the endpoint.
 */
export function isTextPreviewableType(mimetype: string | null | undefined): boolean {
  const base = (mimetype ?? "").split(";")[0];
  return base.startsWith("text/") || TEXT_PREVIEW_MIME_TYPES.has(base);
}

// Text fetches are queued so a folder full of text files does not fire dozens of
// requests at once. Mirrors previewRequests.ts, but returns parsed JSON.
const MAX_CONCURRENT_LOADS = 8;
let activeLoads = 0;
const queue: Array<() => void> = [];

interface QueueEntry {
  start: () => void;
  started: boolean;
}

function runQueued<T>(task: () => Promise<T>, signal: AbortSignal | undefined): Promise<T> {
  return new Promise<T>((resolve, reject) => {
    const entry: QueueEntry = { start: () => {}, started: false };
    const onAbort = () => {
      // Drop the still-queued entry so an unmounted tile cannot hold a slot. When the
      // request has already started, the abort signal rejects the in-flight fetch instead.
      if (entry.started) {
        return;
      }
      const i = queue.indexOf(entry.start);
      if (i !== -1) {
        queue.splice(i, 1);
      }
      reject(abortError());
    };
    entry.start = () => {
      entry.started = true;
      signal?.removeEventListener("abort", onAbort);
      activeLoads++;
      task()
        .then(resolve, reject)
        .finally(() => {
          activeLoads--;
          const next = queue.shift();
          if (next) next();
        });
    };
    if (signal?.aborted) {
      reject(abortError());
      return;
    }
    signal?.addEventListener("abort", onAbort, { once: true });
    if (activeLoads < MAX_CONCURRENT_LOADS) {
      entry.start();
    } else {
      queue.push(entry.start);
    }
  });
}

function abortError(): DOMException {
  return new DOMException("Aborted", "AbortError");
}

async function requestTextPreview(url: string, signal: AbortSignal | undefined): Promise<TextPreviewPayload> {
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
  }, signal);
}

interface TextPreviewCacheEntry {
  promise: Promise<TextPreviewPayload>;
  controller: AbortController;
  /** Number of mounted consumers (tile and/or popup) that still need this request. */
  refs: number;
  /** True once the request has resolved or rejected and is safe to keep without a consumer. */
  settled?: boolean;
}

const cache = new Map<string, TextPreviewCacheEntry>();

/**
 * Fetches a text preview, reusing a cached result for the same URL and modified time.
 * Failed requests are not cached, so a later view can retry. Each call takes a
 * reference; the matching releaseTextPreview call lets a still-pending request be
 * aborted once every consumer has unmounted.
 */
export function fetchTextPreview(url: string, modified: string | null): Promise<TextPreviewPayload> {
  const key = textPreviewCacheKey(url, modified);
  const existing = cache.get(key);
  if (existing !== undefined) {
    existing.refs++;
    return existing.promise;
  }
  const controller = new AbortController();
  const entry: TextPreviewCacheEntry = {
    promise: new Promise<TextPreviewPayload>(() => {}),
    controller,
    refs: 1,
    settled: false,
  };
  // Mark settled as soon as the request finishes so a later release never aborts a
  // result that is already available (the popup reopen / scroll-back case).
  entry.promise = requestTextPreview(url, controller.signal).finally(() => {
    entry.settled = true;
  });
  cache.set(key, entry);
  if (cache.size > MAX_CACHE_ENTRIES) {
    const oldestKey = cache.keys().next().value;
    if (oldestKey !== undefined) {
      cache.get(oldestKey)?.controller.abort();
      cache.delete(oldestKey);
    }
  }
  entry.promise.catch(() => {
    if (cache.get(key) === entry) {
      cache.delete(key);
    }
  });
  return entry.promise;
}

/**
 * Drops the reference taken by fetchTextPreview. When the last consumer releases, the
 * in-flight request is aborted and removed, so queued work for unmounted tiles does not
 * delay previews for the folder the user is actually looking at. Requests still shared
 * by an active consumer are kept.
 */
export function releaseTextPreview(url: string, modified: string | null): void {
  const key = textPreviewCacheKey(url, modified);
  const entry = cache.get(key);
  if (entry === undefined) {
    return;
  }
  entry.refs--;
  // Only abort a request that has not finished. A settled result stays cached so the
  // popup can reopen (and virtualization can scroll back) without re-fetching.
  if (entry.refs <= 0 && !entry.settled) {
    entry.controller.abort();
    cache.delete(key);
  }
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
  for (const entry of cache.values()) {
    entry.controller.abort();
  }
  cache.clear();
  renderedCache.clear();
}
