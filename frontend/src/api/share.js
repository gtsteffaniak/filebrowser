import { fetchURL, fetchJSON, adjustedData } from "./utils";
import { notify } from "@/notify";
import { getApiPath } from "@/utils/url.js";


// ============================================================================
// SHARE MANAGEMENT API (permission-based authentication)
// ============================================================================

// List all shares
export async function list() {
  try {
  const apiPath = getApiPath("api/shares");
    return await fetchJSON(apiPath);
  } catch (/** @type {any} */ err) {
    notify.showError(err.message || "Error listing shares");
    throw err;
  }
}

// Get share information
/**
 * @param {string} path
 * @param {string} source
 * @returns {Promise<any>}
 */
export async function get(path, source) {
  try {
    const params = { path, source };
    const apiPath = getApiPath("api/share", params);
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
  try {
    const params = { hash };
    const apiPath = getApiPath("api/share", params);
    await fetchURL(apiPath, {
      method: "DELETE",
    });
  } catch (/** @type {any} */ err) {
    notify.showError(err.message || "Error deleting share");
    throw err;
  }
}

// Create a new share
/**
 * @param {Record<string, any>} bodyObj
 * @returns {Promise<Share>}
 */
export async function create(bodyObj = {}) {
  try {
    const apiPath = getApiPath("api/share");
    return await fetchJSON(apiPath, {
    method: "POST",
    body: JSON.stringify(bodyObj || {}),
  });
  } catch (/** @type {any} */ err) {
    notify.showError(err.message || "Error creating share");
    throw err;
  }
}

// Update share path
/**
 * @param {string} hash
 * @param {string} newPath
 * @returns {Promise<Share>}
 */
export async function updatePath(hash, newPath) {
  try {
    const apiPath = getApiPath("api/share");
    return await fetchJSON(apiPath, {
      method: "PATCH",
      body: JSON.stringify({ hash, path: newPath }),
      headers: { 'Content-Type': 'application/json' }
    });
  } catch (/** @type {any} */ err) {
    notify.showError(err.message || "Error updating share path");
    throw err;
  }
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
 * @property {string} viewMode
 * @property {string} token
 * @property {boolean} inline
 */
