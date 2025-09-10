import { adjustedData } from "./utils";
import { notify } from "@/notify";
import { getApiPath, getPublicApiPath } from "@/utils/url.js";
import { globalVars } from "@/utils/constants";
import { state } from "@/store";

// ============================================================================
// PUBLIC API ENDPOINTS (hash-based authentication)
// ============================================================================

// Fetch public share data
/**
 * @param {string} path
 * @param {string} hash
 * @param {string} password
 * @param {boolean} content
 * @returns {Promise<any>}
 */
export async function fetchPub(path, hash, password = "", content = false) {
  const params = {
    path,
    hash,
    ...(content && { content: 'true' }),
    ...(state.share.token && { token: state.share.token })
  }
  const apiPath = getPublicApiPath("resources", params);
  const response = await fetch(apiPath, {
    headers: {
      "X-SHARE-PASSWORD": password ? encodeURIComponent(password) : "",
    },
  });

  if (!response.ok) {
    const error = new Error(response.statusText);
    // attempt to marshal json response
    let data = null;
    try {
      data = await response.json()
    } catch (e) {
      // ignore
    }
    if (data) {
      error.message = data.message;
    }
    (/** @type {any} */ (error)).status = response.status;
    throw error;
  }
  let data = await response.json()
  const adjusted = adjustedData(data);
  return adjusted
}

// Generate a download URL
/**
 * @param {{ path: string; hash: string; token: string; inline?: boolean }} share
 * @param {string[]} files
 * @returns {string}
 */
export function getDownloadURL(share, files, inline=false) {
  // Join files array with || delimiter and then URL encode
  const filesParam = Array.isArray(files) ? files.join('||') : files;
  const params = {
    files: encodeURIComponent(filesParam),
    hash: share.hash,
    token: share.token,
    ...(inline && { inline: 'true' })
  }
  const apiPath = getPublicApiPath("raw", params);
  return window.origin + apiPath
}

// Generate a preview URL for public shares
/**
 * @param {string} path
 * @returns {string}
 */
export function getPreviewURL(path,size="small") {
  try {
    const params = {
      path: encodeURIComponent(path),
      size: size,
      hash: state.share.hash,
      inline: 'true',
      ...(state.share.token && { token: state.share.token })
    }
    const apiPath = getPublicApiPath('preview', params)
    return window.origin + apiPath
  } catch (/** @type {any} */ err) {
    notify.showError(err.message || 'Error getting preview URL')
    throw err
  }
}

// Get the shareable URL for a share
/**
 * @param {{ hash: string }} share
 * @returns {string}
 */
export function getShareURL(share) {
  if (globalVars.externalUrl) {
    const apiPath = getApiPath(`public/share/${share.hash}`)
    return globalVars.externalUrl + apiPath;
  }
  return window.origin + getApiPath(`public/share/${share.hash}`);
}
