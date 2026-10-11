import { notify } from "@/notify/index.ts";
import { state } from "@/store/index.ts";
import { getApiPath, getPublicApiPath } from "@/utils/url.js";
import { adjustedData, fetchURL } from "./utils.ts";
import {
  getDirMetadataEntry,
  setCachedDirMetadata,
  setDirMetadataEntryPromise,
  stemOf,
  takePendingDirChanges,
} from "@/utils/metadataCache.ts";

// GET /api/media/subtitles or /public/api/media/subtitles
export async function getSubtitleContent(source, path, subtitleName, embedded = false) {
  try {
    const hash = state.shareInfo?.hash || null
    const baseParams = {
      path,
      name: subtitleName,
      ...(embedded && { embedded: 'true' }),
    }

    let res
    if (hash) {
      const apiPath = getPublicApiPath('media/subtitles', {
        hash,
        ...baseParams,
      })
      const sharePassword = localStorage.getItem(`sharepass:${hash}`) || ''
      res = await fetchURL(apiPath, {
        headers: { 'X-SHARE-PASSWORD': encodeURIComponent(sharePassword) },
      })
    } else {
      const apiPath = getApiPath('media/subtitles', {
        source,
        ...baseParams,
      })
      res = await fetchURL(apiPath)
    }

    const content = await res.text()
    return content
  } catch (err) {
    notify.showError(err.message || `Error fetching subtitle ${subtitleName}`)
    throw err
  }
}

// GET /api/media/lyrics
export async function getLyrics(source, path) {
    const apiPath = getApiPath('media/lyrics', {
        source: source,
        path: path,
    });
    const res = await fetchURL(apiPath);
    const data = await res.json();
    return { lyrics: data.lyrics || "", format: data.format || "" };
}

async function publicResponseError(response) {
  const error = new Error(response.statusText);
  try {
    const data = await response.json();
    if (data?.message) {
      error.message = data.message;
    }
  } catch (_e) {
    // keep the status text
  }
  /** @type {any} */ (error).status = response.status;
  return error;
}

// GET /public/api/media/lyrics
export async function getLyricsPublic(path, hash, password = "") {
  const params = {
    path,
    hash,
  };
  const apiPath = getPublicApiPath("media/lyrics", params);
  const response = await fetch(apiPath, {
    headers: { "X-SHARE-PASSWORD": encodeURIComponent(password || "") },
  });
  if (!response.ok) {
    throw await publicResponseError(response);
  }
  const data = await response.json();
  return { lyrics: data.lyrics || "", format: data.format || "" };
}

// GET /api/media/metadata — directory or file with metadata; optional albumArt for embedded cover extraction.
/** @param {boolean} albumArt when true, request embedded album art in audio metadata */
/** @returns {Promise<object>} resource (adjustedData) */
export async function fetchDirectoryMediaMetadata(source, path, albumArt = false) {
  const apiPath = getApiPath("media/metadata", {
    source,
    path,
    ...(albumArt ? { albumArt: "true" } : {}),
  });
  const res = await fetchURL(apiPath);
  const data = await res.json();
  return adjustedData(data);
}

// GET /public/api/media/metadata
/** @param {boolean} albumArt when true, request embedded album art in audio metadata */
/** @returns {Promise<object>} resource (adjustedData) */
export async function fetchDirectoryMediaMetadataPublic(path, hash, password = "", albumArt = false) {
  const params = {
    path,
    hash,
    ...(albumArt ? { albumArt: "true" } : {}),
  };
  const apiPath = getPublicApiPath("media/metadata", params);
  const response = await fetch(apiPath, {
    headers: { "X-SHARE-PASSWORD": encodeURIComponent(password || "") },
  });
  if (!response.ok) {
    throw await publicResponseError(response);
  }
  const data = await response.json();
  return adjustedData(data);
}

const MAX_ITEM_REFRESHES = 15;

async function fetchWholeDirectoryMetadata({ isShare, source, hash, path, albumArt }, password) {
  const payload = isShare
    ? await fetchDirectoryMediaMetadataPublic(path, hash, password, albumArt)
    : await fetchDirectoryMediaMetadata(source, path, albumArt);
  return new Map(
    (payload?.items || []).filter((i) => i.metadata).map((i) => [i.name, i.metadata])
  );
}

async function fetchItemMetadata({ isShare, source, hash, path, albumArt }, password, name) {
  const base = !path || path === "/" ? "" : path.replace(/\/+$/, "");
  const itemPath = `${base}/${name}`;
  const payload = isShare
    ? await fetchDirectoryMediaMetadataPublic(itemPath, hash, password, albumArt)
    : await fetchDirectoryMediaMetadata(source, itemPath, albumArt);
  return payload && payload.type !== "directory" && payload.metadata ? payload.metadata : null;
}

// Applies the per-item changes recorded since the folder was cached, fetching only the changed items.
function resolvePendingChanges(entry, { removed, dirty, stems }, cacheOpts, password) {
  const base = entry.promise;
  const promise = (async () => {
    const map = new Map(await base);
    for (const name of removed) {
      map.delete(name);
    }
    const toRefresh = new Set(dirty);
    if (stems.size) {
      for (const name of map.keys()) {
        if (stems.has(stemOf(name))) {
          toRefresh.add(name);
        }
      }
    }
    if (toRefresh.size > MAX_ITEM_REFRESHES) {
      return fetchWholeDirectoryMetadata(cacheOpts, password);
    }
    try {
      await Promise.all(
        [...toRefresh].map(async (name) => {
          const metadata = await fetchItemMetadata(cacheOpts, password, name);
          if (metadata) {
            map.set(name, metadata);
          } else {
            map.delete(name);
          }
        })
      );
    } catch (_e) {
      return fetchWholeDirectoryMetadata(cacheOpts, password);
    }
    return map;
  })();
  setDirMetadataEntryPromise(entry, promise);
  return promise;
}

/**
 * Fetches media metadata, and caches it by directory, shared for all the callers to avoid unnecessary requests.
 * Items changed (deleted, uploaded, moved) are patched individually instead of refetch the whole dir
 * @param {string} path - directory path
 * @param {{isShare?: boolean, source?: string, hash?: string, password?: string, albumArt?: boolean}} opts
 * @returns {Promise<Map<string, object>>}
 */
export async function getDirectoryMetadataMap(path, opts = {}) {
  const { isShare = false, source, hash, password = "", albumArt = false } = opts;
  const cacheOpts = { isShare, source, hash, path, albumArt };
  const entry = albumArt ? null : getDirMetadataEntry(cacheOpts);
  if (entry) {
    const pending = takePendingDirChanges(entry);
    return pending ? resolvePendingChanges(entry, pending, cacheOpts, password) : entry.promise;
  }
  const promise = fetchWholeDirectoryMetadata(cacheOpts, password);
  if (!albumArt) {
    setCachedDirMetadata(cacheOpts, promise);
  }
  return promise;
}

// GET /api/media/stream — audio/video bytes via viewToken (range-based, not download-metered).
export function getStreamURL(source, path, viewToken) {
  if (!source || source === undefined || source === null) {
    throw new Error('no source provided')
  }
  if (!viewToken) {
    throw new Error('view token required')
  }
  try {
    const params = {
      source: source,
      file: path,
      viewToken: viewToken,
      sessionId: state.sessionId,
    }
    const apiPath = getApiPath('media/stream', params)
    return window.origin + apiPath
  } catch (err) {
    notify.showError(err.message || 'Error getting stream URL')
    throw err
  }
}

// GET /public/api/media/stream
export function getStreamURLPublic(share, files, viewToken) {
  if (!viewToken) {
    throw new Error('view token required')
  }
  const fileArray = Array.isArray(files) ? files : [files]
  const params = {
    file: fileArray,
    hash: share.hash,
    viewToken: viewToken,
    sessionId: state.sessionId,
  }
  const apiPath = getPublicApiPath('media/stream', params)
  return window.origin + apiPath
}
