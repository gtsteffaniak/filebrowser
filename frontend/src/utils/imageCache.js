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
    console.log('[ImageCache] Set: Skipping - missing path or url', { path: !!path, url: !!url });
    return;
  }

  // If we're at capacity, remove the oldest entry (first in Map)
  if (imageCache.size >= MAX_CACHE_SIZE) {
    const firstKey = imageCache.keys().next().value;
    imageCache.delete(firstKey);
    console.log('[ImageCache] Cache at capacity, removed oldest entry. Size:', imageCache.size);
  }

  const key = getCacheKey(source, path, size, modified);
  const wasAlreadyCached = imageCache.has(key);
  imageCache.set(key, url);
  
  const sourceStr = source || 'share';
  if (!wasAlreadyCached) {
    console.log(`[ImageCache] Set: ✅ Added {source: '${sourceStr}', path: '${path}', size: '${size}', modified: '${modified}'}`, {
      key,
      url: url.substring(0, 100) + (url.length > 100 ? '...' : ''),
      cacheSize: imageCache.size
    });
  } else {
    console.log(`[ImageCache] Set: Already cached {source: '${sourceStr}', path: '${path}', size: '${size}', modified: '${modified}'}`, { key });
  }
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
  if (!path) {
    console.log('[ImageCache] Get: No path provided');
    return null;
  }

  const sourceStr = source || 'share';
  const largeKey = getCacheKey(source, path, 'large', modified);
  const smallKey = getCacheKey(source, path, 'small', modified);
  
  console.log('[ImageCache] Get: Checking cache', {
    source: sourceStr,
    path,
    modified,
    largeKey,
    smallKey,
    hasLarge: imageCache.has(largeKey),
    hasSmall: imageCache.has(smallKey),
    cacheSize: imageCache.size
  });

  // Check for large first
  if (imageCache.has(largeKey)) {
    const url = imageCache.get(largeKey);
    console.log(`[ImageCache] Get: ✅ Found large for {source: '${sourceStr}', path: '${path}', modified: '${modified}'}`, { url });
    return url;
  }

  // Check for small
  if (imageCache.has(smallKey)) {
    const url = imageCache.get(smallKey);
    console.log(`[ImageCache] Get: ✅ Found small for {source: '${sourceStr}', path: '${path}', modified: '${modified}'}`, { url });
    return url;
  }

  // Debug: show first few cache keys to help diagnose
  const cacheKeys = Array.from(imageCache.keys()).slice(0, 5);
  console.log(`[ImageCache] Get: ❌ Not found for {source: '${sourceStr}', path: '${path}', modified: '${modified}'}`, {
    searchedLargeKey: largeKey,
    searchedSmallKey: smallKey,
    sampleCacheKeys: cacheKeys
  });
  return null;
}

/**
 * Clear the image cache (useful for testing or memory management)
 */
export function clearImageCache() {
  imageCache.clear();
  console.log('[ImageCache] Cache cleared');
}

/**
 * Get the current cache size (for debugging/monitoring)
 * @returns {number} - Number of entries in the cache
 */
export function getCacheSize() {
  return imageCache.size;
}
