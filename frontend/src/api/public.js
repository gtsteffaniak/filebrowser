import { fetchURL, fetchJSON, adjustedData } from "./utils";
import { notify } from "@/notify";
import { getApiPath, removePrefix } from "@/utils/url.js";
import { externalUrl, baseURL } from "@/utils/constants";
import { state } from "@/store";

// ============================================================================
// PUBLIC API ENDPOINTS (hash-based authentication)
// ============================================================================

// Fetch public share data
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
    error.status = response.status;
    throw error;
  }
  let data = await response.json()
  console.log("fetchPub adjusted",data, `/public/share/${hash}${path}`)
  const adjusted = adjustedData(data, `/public/share/${hash}${path}`);
  console.log("fetchPub adjusted2",adjusted)
  return adjusted
}

// Generate a download URL
export function getDownloadURL(share, files) {
  const params = {
    path: share.path,
    files: files,
    hash: share.hash,
    token: share.token,
    ...(share.inline && { inline: 'true' })
  }
  const apiPath = getApiPath("public/api/raw", params);
  return window.origin + apiPath
}

// Generate a preview URL for public shares
export function getPreviewURL(path) {
  try {
    const params = {
      path: encodeURIComponent(path),
      size: 'small',
      hash: state.share.hash,
      inline: 'true',
      ...(state.share.token && { token: state.share.token })
    }
    const apiPath = getApiPath('public/api/preview', params)
    return window.origin + apiPath
  } catch (err) {
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
export async function get(path, source) {
  try {
    const params = { path: encodeURIComponent(path), source: encodeURIComponent(source) };
    const apiPath = getApiPath("public/share", params);
    let data = fetchJSON(apiPath);
    return adjustedData(data, path);
  } catch (err) {
    notify.showError(err.message || "Error fetching data");
    throw err;
  }
}

// Remove/delete a share
export async function remove(hash) {
  const params = { hash };
  const apiPath = getApiPath("public/share", params);
  await fetchURL(apiPath, {
    method: "DELETE",
  });
}

// Create a new share
export async function create(path, source, password = "", expires = "", unit = "hours") {
  const params = { path: encodeURIComponent(path), source: encodeURIComponent(source) };
  const apiPath = getApiPath("public/share", params);
  let body = "{}";
  if (password != "" || expires !== "" || unit !== "hours") {
    body = JSON.stringify({ password: password, expires: expires, unit: unit });
  }
  return fetchJSON(apiPath, {
    method: "POST",
    body: body,
  });
}

// Get the shareable URL for a share
export function getShareURL(share) {
  if (externalUrl) {
    const apiPath = getApiPath(`public/share/${share.hash}`)
    return externalUrl + removePrefix(apiPath, baseURL);
  }
  return window.origin + getApiPath(`public/share/${share.hash}`);
}
