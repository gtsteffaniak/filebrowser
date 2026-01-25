import { adjustedData } from "./utils";
import { notify } from "@/notify";
import { getPublicApiPath, encodedPath } from "@/utils/url.js";
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
 * @param {boolean} metadata
 * @returns {Promise<any>}
 */
export async function fetchPub(path, hash, password = "", content = false, metadata = false) {
  path = encodedPath(path);
  const params = {
    path: path,
    hash,
    ...(content && { content: 'true' }),
    ...(metadata && { metadata: 'true' }),
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
  // Handle array of files for repeated 'file' parameters
  const fileArray = Array.isArray(files) ? files : [files]
  const filePaths = fileArray.map(file => encodeURIComponent(file))
  
  const params = {
    file: filePaths, // Array of file paths - getPublicApiPath will create repeated parameters
    hash: share.hash,
    token: share.token,
    ...(inline && { inline: 'true' })
  }
  const apiPath = getPublicApiPath("raw", params)
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

export function post(
  hash,
  path,
  content = "",
  overwrite = false,
  onupload,
  headers = {},
  isDir = false
) {
  if (!hash || hash === undefined || hash === null) {
    throw new Error('no hash provided')
  }
  let sharePassword = localStorage.getItem("sharepass:" + hash);
  if (sharePassword) {
    headers["X-SHARE-PASSWORD"] = sharePassword;
  }
  try {
    const apiPath = getPublicApiPath("resources", {
      targetPath: encodeURIComponent(path),
      hash: hash,
      override: overwrite,
      ...(isDir && { isDir: 'true' })
    });

    const request = new XMLHttpRequest();
    request.open("POST", apiPath, true);

    for (const header in headers) {
      request.setRequestHeader(header, headers[header]);
    }
    if (typeof onupload === "function") {
      request.upload.onprogress = (event) => {
        if (event.lengthComputable) {
          const percentComplete = Math.round(
            (event.loaded / event.total) * 100
          );
          onupload(percentComplete); // Pass the percentage to the callback
        }
      };
    }

    const promise = new Promise((resolve, reject) => {
      request.onload = () => {
        if (request.status >= 200 && request.status < 300) {
          resolve(request.responseText);
        } else if (request.status === 409) {
          const error = new Error("conflict");
          error.response = { status: request.status, responseText: request.responseText };
          reject(error);
        } else {
          // Parse error message from response
          let errorMessage = "Upload failed";
          try {
            const errorData = JSON.parse(request.responseText);
            errorMessage = errorData.message || errorMessage;
          } catch (e) {
            // If parsing fails, use responseText or default message
            errorMessage = request.responseText || errorMessage;
          }
          
          const error = new Error(errorMessage);
          error.status = request.status;
          
          // Show notification for upload errors
          notify.showError(errorMessage);
          
          reject(error);
        }
      };

      request.onerror = () => {
        const error = new Error("Network error");
        notify.showError("Network error during upload");
        reject(error);
      };
      
      request.onabort = () => {
        const error = new Error("Upload aborted");
        notify.showError("Upload was aborted");
        reject(error);
      };

      if (
        content instanceof Blob &&
        !["http:", "https:"].includes(window.location.protocol)
      ) {
        new Response(content).arrayBuffer()
          .then(buffer => request.send(buffer))
          .catch(err => reject(err));
      } else {
        request.send(content);
      }
    });

    promise.xhr = request;
    return promise;
  } catch (err) {
    notify.showError(err.message || "Error posting resource");
    // We are returning a promise, so we should return a rejected promise on error.
    return Promise.reject(err);
  }
}

async function resourceAction(hash, path, method, content, token = "") {
  try {
    let headers = {};
    let sharePassword = localStorage.getItem("sharepass:" + hash);
    if (sharePassword) {
      headers["X-SHARE-PASSWORD"] = sharePassword;
    }
    path = encodeURIComponent(path)
    const apiPath = getPublicApiPath('resources', { path, hash: hash, token: token })
    const response = await fetch(apiPath, {
      method,
      body: content,
      headers,
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
    return response;
  } catch (err) {
    notify.showError(err.message || 'Error performing resource action')
    throw err
  }
}

export async function bulkDelete(items) {
  if (!items || !Array.isArray(items) || items.length === 0) {
    throw new Error('items array is required and must not be empty')
  }
  
  const hash = state.shareInfo?.hash;
  if (!hash) {
    throw new Error('share hash is required')
  }
  
  const params = {
    hash: hash,
    ...(state.share.token && { token: state.share.token }),
    sessionId: state.sessionId
  }
  const apiPath = getPublicApiPath("resources/bulk/delete", params)
  const baseUrl = window.origin + apiPath

  try {
    const response = await fetch(baseUrl, {
      method: 'POST',
      headers: {
        'Content-Type': 'application/json',
      },
      credentials: 'same-origin',
      body: JSON.stringify(items),
    })

    const data = await response.json()

    // 200 = all succeeded, 207 = partial success (some succeeded, some failed)
    // Both are valid responses that should be returned, not thrown as errors
    if (response.status === 200 || response.status === 207) {
      return data
    }

    // For other error status codes, throw an error
    const error = new Error(data.message || response.statusText)
    error.status = response.status
    throw error
  } catch (err) {
    // If the request fails completely, return all items as failed
    if (err.status && err.status !== 200 && err.status !== 207) {
      // Real error - return all as failed
      return {
        succeeded: [],
        failed: items.map(item => ({
          source: item.source || '',
          path: item.path,
          message: err.message || 'Delete failed',
        })),
      }
    }
    // Re-throw if it's not a handled error
    throw err
  }
}

export async function put(hash, path, content = '') {
  // resourceAction already handles error notification, just propagate
  return await resourceAction(hash, path, 'PUT', content)
}

export async function moveCopy(
  hash,
  items,
  action = 'copy',
  overwrite = false,
  rename = false
) {
  if (!items || !Array.isArray(items) || items.length === 0) {
    throw new Error('items array is required and must not be empty')
  }

  try {
    // Build request body with proper format
    // For public shares, fromSource and toSource are not needed (always the share's source)
    const requestBody = {
      items: items.map(item => ({
        fromPath: item.from,
        toPath: item.to
      })),
      action: action,
      overwrite: overwrite,
      rename: rename
    }

    const apiPath = getPublicApiPath('resources', { hash: hash })
    const response = await fetch(apiPath, {
      method: 'PATCH',
      headers: {
        'Content-Type': 'application/json',
        ...(state.share.token && { 'X-Auth-Token': state.share.token })
      },
      body: JSON.stringify(requestBody),
    })

    const data = await response.json()

    // 200 = all succeeded, 207 = partial success (some succeeded, some failed)
    if (response.status === 200 || response.status === 207) {
      return data
    }

    // For other error status codes (like 500), preserve the response data
    const error = new Error(data.message || response.statusText)
    error.status = response.status
    // Attach the response data so the caller can access failed items
    if (data.failed) {
      error.failed = data.failed
    }
    if (data.succeeded) {
      error.succeeded = data.succeeded
    }
    throw error
  } catch (err) {
    // Only show notification and re-throw if it's a real error (not 200/207)
    if (err.status && err.status !== 200 && err.status !== 207) {
      console.error(err.message || 'Error moving/copying resources')
    }
    throw err
  }
}

export async function getShareInfo(hash) {
  try {
    const apiPath = getPublicApiPath('shareinfo', { hash: hash })
    const response = await fetch(apiPath)
    return response.json()
  } catch (err) {
    notify.showError(err.message || 'Error getting share info')
    throw err
  }
}