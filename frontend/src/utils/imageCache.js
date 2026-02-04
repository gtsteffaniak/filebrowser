import { removeTrailingSlash } from "@/utils/url";

/**
 * Image Cache Utility
 * Simple getter/setter API for tracking loaded images by source, path, size, and modified date.
 * 
 * Stores the actual URL that was loaded. When getting, prefers large size if available, falls back to small.
 * Includes modified date in cache key so file changes invalidate the cache.
 */

// Maximum number of entries to track (prevents unbounded memory growth)
const MAX_CACHE_SIZE = 10000;

// Map to track loaded images: key = "source|path|size|modified", value = URL
let imageCache = new Map();

/**
 * Generate cache key from source, path, size, and modified date
 * @param {string} source - The source identifier (null for shares)
 * @param {string} path - The file path
 * @param {string} size - The size ('large' or 'small')
 * @param {string|number} modified - The file modified date (used as cache invalidation)
 * @returns {string} - Cache key
 */
function getCacheKey(source, path, size, modified) {
  const sourcePart = source || 'share';
  const modifiedPart = modified || '';
  return `${sourcePart}|${path}|${size}|${modifiedPart}`;
}

/**
 * Mark an image as loaded with its URL
 * @param {string} source - The source identifier (null for shares)
 * @param {string} path - The file path
 * @param {string} size - The size ('large' or 'small')
 * @param {string|number} modified - The file modified date
 * @param {string} url - The URL that was loaded
 */
export function setImageLoaded(source, path, size, modified, url) {
  if (!path || !url) {
    return;
  }
  path = removeTrailingSlash(path);
  // If we're at capacity, remove the oldest entry (first in Map)
  if (imageCache.size >= MAX_CACHE_SIZE) {
    const firstKey = imageCache.keys().next().value;
    imageCache.delete(firstKey);
  }

  const key = getCacheKey(source, path, size, modified);
  imageCache.set(key, url);
}

/**
 * Get the best available cached image URL
 * Prefers large if available, falls back to small
 * @param {string} source - The source identifier (null for shares)
 * @param {string} path - The file path
 * @param {string|number} modified - The file modified date
 * @returns {string|null} - The cached URL, or null if not cached
 */
export function getBestCachedImage(source, path, modified) {
  // strip trailing slash from path
  path = removeTrailingSlash(path);
  if (!path) {
    return null;
  }

  const largeKey = getCacheKey(source, path, 'large', modified);
  const smallKey = getCacheKey(source, path, 'small', modified);

  // Check for large first
  if (imageCache.has(largeKey)) {
    return imageCache.get(largeKey);
  }

  // Check for small
  if (imageCache.has(smallKey)) {
    return imageCache.get(smallKey);
  }
  return null;
}

/**
 * Clear the image cache (useful for testing or memory management)
 */
export function clearImageCache() {
  imageCache.clear();
}

/**
 * Get the current cache size (for debugging/monitoring)
 * @returns {number} - Number of entries in the cache
 */
export function getCacheSize() {
  return imageCache.size;
}
