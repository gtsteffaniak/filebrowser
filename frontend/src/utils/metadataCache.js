// src/utils/metadataCache.js

// maximum number of entries to prevent memory grow
const MAX_CACHE_ITEMS = 500;
const cache = new Map();

/**
 * cache key from source/share scope, path, and albumArt
 * @param {{isShare?: boolean, source?: string, hash?: string, path: string, albumArt?: boolean}} opts
 * @returns {string} - cache key
 */
function getCacheKey({ isShare, source, hash, path, albumArt }) {
  const scopePart = isShare ? `share:${hash}` : `src:${source}`;
  const albumArtPart = albumArt ? 'art' : 'noart';
  const pathPart = !path || path === '/' ? '/' : path.replace(/\/+$/, '');
  return `${scopePart}|${pathPart}|${albumArtPart}`;
}

/**
 * get the cached promise
 * @param {{isShare?: boolean, source?: string, hash?: string, path: string, albumArt?: boolean}} opts
 * @returns {Promise<Map<string, object>>|null} - the cached promise, or null if not cached
 */
export function getCachedDirMetadata(opts) {
  return cache.get(getCacheKey(opts)) || null;
}

/**
 * store a pending/resolved promise for these options
 * @param {{isShare?: boolean, source?: string, hash?: string, path: string, albumArt?: boolean}} opts
 * @param {Promise<Map<string, object>>} promise - the promise to cache
 */
export function setCachedDirMetadata(opts, promise) {
  if (cache.size >= MAX_CACHE_ITEMS) {
    const firstKey = cache.keys().next().value;
    cache.delete(firstKey);
  }

  const key = getCacheKey(opts);
  cache.set(key, promise);
  promise.catch(() => cache.delete(key));
}

/**
 * invalidates cached dir metadata so next call gets fresh data
 * @param {{isShare?: boolean, source?: string, hash?: string, path?: string}} [opts]
 */
export function invalidateDirMetadataCache(opts = {}) {
  const { isShare = false, source, hash, path } = opts;
  if (path === undefined) {
    if (source === undefined && hash === undefined) {
      cache.clear();
      return;
    }
    const scope = isShare ? `share:${hash}|` : `src:${source}|`;
    for (const key of cache.keys()) {
      if (key.startsWith(scope)) cache.delete(key);
    }
    return;
  }
  cache.delete(getCacheKey({ isShare, source, hash, path, albumArt: false }));
  cache.delete(getCacheKey({ isShare, source, hash, path, albumArt: true }));
}
