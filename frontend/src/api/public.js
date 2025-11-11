import { adjustedData } from "./utils";
import { notify } from "@/notify";
import { getApiPath, getPublicApiPath, encodedPath, doubleEncode } from "@/utils/url.js";
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
  path = encodedPath(path);
  const params = {
    path: path,
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


export function post(
  hash,
  path,
  content = "",
  overwrite = false,
  onupload,
  headers = {}
) {
  if (!hash || hash === undefined || hash === null) {
    throw new Error('no hash provided')
  }
  try {
    const apiPath = getPublicApiPath("resources", {
      targetPath: doubleEncode(path),
      hash: hash,
      override: overwrite,
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
          reject(new Error(request.responseText || "Upload failed"));
        }
      };

      request.onerror = () => reject(new Error("Network error"));
      request.onabort = () => reject(new Error("Upload aborted"));

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
    path = doubleEncode(path)
    const apiPath = getPublicApiPath('resources', { path, hash: hash, token: token })
    const response = await fetch(apiPath, {
      method,
      body: content,
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

export async function remove(hash, path) {
  try {
    return await resourceAction(hash, path, 'DELETE')
  } catch (err) {
    notify.showError(err.message || 'Error deleting resource')
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
  let params = {
    overwrite: overwrite,
    action: action,
    rename: rename,
    hash: hash
  }
  try {
    // Create an array of fetch calls
    let promises = items.map(item => {
      let localParams = {
        ...params,
        destination: doubleEncode(item.to),
        from: doubleEncode(item.from)
      }
      const apiPath = getPublicApiPath('resources', localParams)

      return fetch(apiPath, { method: 'PATCH' }).then(response => {
        if (!response.ok) {
          return response.text().then(text => {
            throw new Error(
              `Failed to move/copy: ${text || response.statusText}`
            )
          })
        }
        return response
      })
    })

    // Await all promises and ensure errors propagate
    await Promise.all(promises)
  } catch (err) {
    console.error(err.message || 'Error moving/copying resources')
    throw err // Re-throw the error to propagate it back to the caller
  }
}

export async function getShareInfo(hash) {
  try {
    const apiPath = getPublicApiPath('share', { hash: hash })
    const response = await fetch(apiPath)
    return response.json()
  } catch (err) {
    notify.showError(err.message || 'Error getting share info')
    throw err
  }
}