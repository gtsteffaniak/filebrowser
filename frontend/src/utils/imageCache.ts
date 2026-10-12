import { state } from "@/store/index.ts";
import { removeTrailingSlash } from "@/utils/url.js";

/**
 * Two small caches that work together:
 * 1. URL index that remembers which preview URL was loaded for a source, path, size and modified.
 * 2. Thumbnail blobs which fetch thumbnail data itself, so the images can show it instantly without a request.
*/

/** null for shares */
type Source = string | null | undefined;
type Modified = string | number | null | undefined;
type ThumbnailSize = "small" | "large" | "xlarge";

interface ThumbnailBlob {
  blobUrl: string;
  refs: number;
  bytes: number;
}

// Maximum number of entries in the URL index
const MAX_URL_INDEX_SIZE = 10000;
const MAX_THUMBNAIL_BYTES = 32 * 1024 * 1024;

// key = "source|path|size|modified" value = URL. Insertion order is recency order.
const urlIndex = new Map<string, string>();
const thumbnailBlobs = new Map<string, ThumbnailBlob>(); // least recently used first
let thumbnailBytes = 0;

function evictThumbnailBlobs(): void {
  for (const [key, entry] of thumbnailBlobs) {
    if (thumbnailBytes <= MAX_THUMBNAIL_BYTES) {
      break;
    }
    if (entry.refs === 0) {
      URL.revokeObjectURL(entry.blobUrl);
      thumbnailBlobs.delete(key);
      thumbnailBytes -= entry.bytes;
    }
  }
}

/**
 * Key for a cached thumbnail blob per user
 * @param url the preview URL that was fetched
 * @param modified the file modified date
 */
export function getThumbnailBlobKey(url: string, modified?: Modified): string {
  return `${state.user?.username || ""}|${modified || ""}|${url}`;
}

/**
 * reference to a cached thumbnail blob URL.
 * @returns blob URL, or null if not cached
 */
export function acquireThumbnailBlob(key: string): string | null {
  const entry = thumbnailBlobs.get(key);
  if (!entry) {
    return null;
  }
  entry.refs++;
  thumbnailBlobs.delete(key);
  thumbnailBlobs.set(key, entry);
  return entry.blobUrl;
}

/**
 * Stores a freshly fetched thumbnail and takes a reference to it. If another Icon stored the same
 * thumbnail first, the new blob is revoked and the existing one is returned.
 * @param bytes size of the fetched blob
 * @returns the blob URL to display
 */
export function storeThumbnailBlob(key: string, blobUrl: string, bytes: number): string {
  const existing = thumbnailBlobs.get(key);
  if (existing) {
    if (existing.blobUrl !== blobUrl) {
      URL.revokeObjectURL(blobUrl);
    }
    existing.refs++;
    return existing.blobUrl;
  }
  const size = Math.max(bytes || 0, 1);
  thumbnailBlobs.set(key, { blobUrl, refs: 1, bytes: size });
  thumbnailBytes += size;
  evictThumbnailBlobs();
  return blobUrl;
}

/** Drops a reference taken with acquireThumbnailBlob / storeThumbnailBlob. */
export function releaseThumbnailBlob(key: string | null): void {
  const entry = key ? thumbnailBlobs.get(key) : undefined;
  if (!entry) {
    return;
  }
  if (entry.refs > 0) {
    entry.refs--;
  }
  evictThumbnailBlobs();
}

function urlIndexKey(source: Source, path: string, size: ThumbnailSize, modified: Modified): string {
  return `${source || "share"}|${path}|${size}|${modified || ""}`;
}

/** Mark an image as loaded with its URL. */
export function setImageLoaded(
  source: Source,
  path: string | null | undefined,
  size: ThumbnailSize,
  modified: Modified,
  url: string | null | undefined,
): void {
  if (!path || !url) {
    return;
  }
  const key = urlIndexKey(source, removeTrailingSlash(path), size, modified);
  if (urlIndex.has(key)) {
    urlIndex.delete(key);
  } else if (urlIndex.size >= MAX_URL_INDEX_SIZE) {
    // drop the oldest entry
    const oldest = urlIndex.keys().next().value;
    if (oldest !== undefined) {
      urlIndex.delete(oldest);
    }
  }
  urlIndex.set(key, url);
}

/** Calls with the URL recorded for each size in order until it returns something */
function findBySize<T>(
  source: Source,
  path: string | null | undefined,
  modified: Modified,
  sizes: readonly ThumbnailSize[],
  visit: (url: string) => T | null,
): T | null {
  const cleanPath = path ? removeTrailingSlash(path) : "";
  if (!cleanPath) {
    return null;
  }
  for (const size of sizes) {
    const url = urlIndex.get(urlIndexKey(source, cleanPath, size, modified));
    const found = url ? visit(url) : null;
    if (found !== null) {
      return found;
    }
  }
  return null;
}

/** @returns the cached URL, or null if not cached */
export function getBestCachedImage(source: Source, path: string | null | undefined, modified: Modified): string | null {
  return findBySize(source, path, modified, ["xlarge", "large", "small"], (url) => url);
}

/** When true shows the image on its very first render */
export function hasCachedThumbnailBlob(
  source: Source,
  path: string | null | undefined,
  modified: Modified,
  size: Exclude<ThumbnailSize, "xlarge">,
): boolean {
  const found = findBySize<boolean>(source, path, modified, [size], (url) =>
    thumbnailBlobs.has(getThumbnailBlobKey(url, modified)) ? true : null,
  );
  return found === true;
}

/**
 * Takes a reference to the best thumbnail blob that is still held for this file (large, then small).
 * xlarge is never fetched as a blob, release it with releaseThumbnailBlob(key) when done.
 */
export function acquireBestCachedThumbnailBlob(
  source: Source,
  path: string | null | undefined,
  modified: Modified,
): { key: string; blobUrl: string } | null {
  return findBySize(source, path, modified, ["large", "small"], (url) => {
    const key = getThumbnailBlobKey(url, modified);
    const blobUrl = acquireThumbnailBlob(key);
    return blobUrl ? { key, blobUrl } : null;
  });
}
