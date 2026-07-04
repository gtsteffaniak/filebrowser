import { notify } from "@/notify";
import { state } from "@/store";
import { getApiPath, getPublicApiPath } from "@/utils/url.js";
import { adjustedData, fetchURL } from "./utils";

// GET /api/media/subtitles
export async function getSubtitleContent(source, path, subtitleName, embedded = false) {
  try {
    const apiPath = getApiPath('media/subtitles', {
      source: source,
      path: path,
      name: subtitleName,
      embedded: embedded.toString()
    })
    const res = await fetchURL(apiPath)
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
    return data.lyrics || [];
}

// GET /public/api/media/lyrics
export async function getLyricsPublic(path, hash, password = "") {
    const params = {
        path,
        hash,
        ...(state.shareInfo.token && { token: state.shareInfo.token }),
    };
    const apiPath = getPublicApiPath("media/lyrics", params);
    const response = await fetch(apiPath, {
        headers: { "X-SHARE-PASSWORD": password || "" },
    });
    if (!response.ok) {
        const error = new Error(response.statusText);
        const data = await response.json();
        if (data?.message) {
            error.message = data.message;
        }
        error.status = response.status;
        throw error;
    }
    const data = await response.json();
    return data.lyrics || [];
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
    ...(state.shareInfo.token && { token: state.shareInfo.token }),
  };
  const apiPath = getPublicApiPath("media/metadata", params);
  const response = await fetch(apiPath, {
    headers: { "X-SHARE-PASSWORD": password || "" },
  });
  if (!response.ok) {
    const error = new Error(response.statusText);
    let data = null;
    try {
      data = await response.json();
    } catch (_e) {
      // ignore
    }
    if (data?.message) {
      error.message = data.message;
    }
    /** @type {any} */ (error).status = response.status;
    throw error;
  }
  const data = await response.json();
  return adjustedData(data);
}

const dirMetadataCache = new Map();
function dirMetadataCacheKey({ isShare, source, hash, path, albumArt }) {
  const scope = isShare ? `share:${hash}` : `src:${source}`;
  return `${scope}:${path}:${albumArt ? "art" : "noart"}`;
}

/**
 * Fetches directory media metadata, and caches it by directory, shared for all the callers
 * to avoid unnecessary requests.
 * @param {string} path - directory path
 * @param {{isShare?: boolean, source?: string, hash?: string, password?: string, albumArt?: boolean}} opts
 * @returns {Promise<Map<string, object>>}
 */
export async function getDirectoryMetadataMap(path, opts = {}) {
  const { isShare = false, source, hash, password = "", albumArt = false } = opts;
  const key = dirMetadataCacheKey({ isShare, source, hash, path, albumArt });
  let pending = dirMetadataCache.get(key);
  if (!pending) {
    pending = (async () => {
      const payload = isShare
        ? await fetchDirectoryMediaMetadataPublic(path, hash, password, albumArt)
        : await fetchDirectoryMediaMetadata(source, path, albumArt);
      return new Map(
        (payload?.items || []).filter((i) => i.metadata).map((i) => [i.name, i.metadata])
      );
    })();
    dirMetadataCache.set(key, pending);
    pending.catch(() => dirMetadataCache.delete(key));
  }
  return pending;
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
    token: share.token,
    viewToken: viewToken,
    sessionId: state.sessionId,
  }
  const apiPath = getPublicApiPath('media/stream', params)
  return window.origin + apiPath
}
