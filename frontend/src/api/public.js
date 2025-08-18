import { fetchURL, fetchJSON, adjustedData } from "./utils";
import { notify } from "@/notify";
import { getApiPath } from "@/utils/url.js";
import { externalUrl } from "@/utils/constants";
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
  const apiPath = getApiPath("public/api/shared", params);
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
export function getDownloadURL(share, files) {
  const params = {
    files: files,
    hash: share.hash,
    token: share.token,
    ...(share.inline && { inline: 'true' })
  }
  const apiPath = getApiPath("public/api/raw", params);
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
    const apiPath = getApiPath('public/api/preview', params)
    return window.origin + apiPath
  } catch (/** @type {any} */ err) {
    notify.showError(err.message || 'Error getting preview URL')
    throw err
  }
}

// ============================================================================
// SHARE MANAGEMENT API (permission-based authentication)  
// ============================================================================

// List all shares
export async function list() {
  const apiPath = getApiPath("public/shares");
  return fetchJSON(apiPath);
}

// Get share information
/**
 * @param {string} path
 * @param {string} source
 * @returns {Promise<any>}
 */
export async function get(path, source) {
  try {
    const params = { path: encodeURIComponent(path), source: encodeURIComponent(source) };
    const apiPath = getApiPath("public/share", params);
    let data = await fetchJSON(apiPath);
    return adjustedData(data);
  } catch (/** @type {any} */ err) {
    notify.showError(err.message || "Error fetching data");
    throw err;
  }
}

// Remove/delete a share
/**
 * @param {string} hash
 * @returns {Promise<void>}
 */
export async function remove(hash) {
  const params = { hash };
  const apiPath = getApiPath("public/share", params);
  await fetchURL(apiPath, {
    method: "DELETE",
  });
}

// Create a new share
/**
 * @param {string} path
 * @param {string} source
 * @param {string} password
 * @param {string} expires
 * @param {string} unit
 * @param {boolean} disableAnonymous
 * @param {boolean} allowUpload
 * @param {number | string} maxBandwidth
 * @param {number | string} downloadsLimit
 * @param {string} shareTheme
 * @param {boolean} disableFileViewer
 * @param {boolean} disableThumbnails
 * @param {string[]} allowedUsernames
 * @param {string} hash
 * @param {boolean} keepAfterExpiration
 * @returns {Promise<Share>}
 */
export async function create(path, source, password = "", expires = "", unit = "hours", disableAnonymous, allowUpload, maxBandwidth, downloadsLimit, shareTheme, disableFileViewer, disableThumbnails, allowedUsernames = [], hash = "", keepAfterExpiration = false) {
  const params = { path: encodeURIComponent(path), source: encodeURIComponent(source) };
  const apiPath = getApiPath("public/share", params);
  let body = "{}";
  if (password != "" || expires !== "" || unit !== "hours" || disableAnonymous || allowUpload || maxBandwidth !== "" || downloadsLimit !== "" || shareTheme !== "default" || disableFileViewer || disableThumbnails || (allowedUsernames && allowedUsernames.length > 0) || hash !== "" || keepAfterExpiration) {
    body = JSON.stringify({
      password: password,
      expires: expires,
      unit: unit,
      disableAnonymous: disableAnonymous,
      allowUpload: allowUpload,
      maxBandwidth: Number(maxBandwidth) || 0,
      downloadsLimit: Number(downloadsLimit) || 0,
      shareTheme: shareTheme,
      disableFileViewer: disableFileViewer,
      disableThumbnails: disableThumbnails,
      allowedUsernames: allowedUsernames,
      hash: hash,
      keepAfterExpiration: keepAfterExpiration,
    });
  }
  return fetchJSON(apiPath, {
    method: "POST",
    body: body,
  });
}

// Get the shareable URL for a share
/**
 * @param {{ hash: string }} share
 * @returns {string}
 */
export function getShareURL(share) {
  if (externalUrl) {
    const apiPath = getApiPath(`public/share/${share.hash}`)
    return externalUrl + apiPath;
  }
  return window.origin + getApiPath(`public/share/${share.hash}`);
}

/**
 * @typedef {object} Share
 * @property {string} hash
 * @property {string} path
 * @property {string} source
 * @property {number} expire
 * @property {number} downloadsLimit
 * @property {number} maxBandwidth
 * @property {string} shareTheme
 * @property {boolean} disableAnonymous
 * @property {boolean} disableThumbnails
 * @property {boolean} keepAfterExpiration
 * @property {string[]} allowedUsernames
 * @property {string} token
 * @property {boolean} inline
 */
