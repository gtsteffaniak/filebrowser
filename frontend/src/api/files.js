import { fetchURL, adjustedData } from './utils'
import { getApiPath, doubleEncode, getPublicApiPath } from '@/utils/url.js'
import { state } from '@/store'
import { notify } from '@/notify'
import { globalVars } from '@/utils/constants'

// Notify if errors occur
export async function fetchFiles(source, path, content = false, metadata = false) {
  if (!source || source === undefined || source === null) {
    throw new Error('no source provided')
  }
  try {
    const apiPath = getApiPath('api/resources', {
      path: doubleEncode(path),
      source: doubleEncode(source),
      ...(content && { content: 'true' }),
      ...(metadata && { metadata: 'true' })
    })
    const res = await fetchURL(apiPath)
    const data = await res.json()
    const adjusted = adjustedData(data)
    return adjusted
  } catch (err) {
    notify.showError(err.message || 'Error fetching data')
    throw err
  }
}

async function resourceAction(source, path, method, content) {
  if (!source || source === undefined || source === null) {
    throw new Error('no source provided')
  }
  try {
    source = doubleEncode(source)
    path = doubleEncode(path)
    const apiPath = getApiPath('api/resources', { path, source })
    let opts = { method }
    if (content) {
      opts.body = content
    }

    const response = await fetchURL(apiPath, opts)
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
    return response
  } catch (err) {
    notify.showError(err.message || 'Error performing resource action')
    throw err
  }
}

export async function remove(source, path) {
  if (!source || source === undefined || source === null) {
    throw new Error('no source provided')
  }
  try {
    return await resourceAction( source, path, 'DELETE')
  } catch (err) {
    notify.showError(err.message || 'Error deleting resource')
    throw err
  }
}

export async function put(source, path, content = '') {
  if (!source) {
    throw new Error('no source provided')
  }
  // resourceAction already handles error notification, just propagate
  return await resourceAction(source, path, 'PUT', content)
}

export async function download(format, files, shareHash = "") {
  if (format !== 'zip') {
    format = 'tar.gz'
  }
  let fileargs = ''
  for (let file of files) {
    if (shareHash) {
      fileargs += encodeURIComponent(file.path) + '||'
    } else {
      fileargs += encodeURIComponent(file.source) + '::' + encodeURIComponent(file.path) + '||'
    }
  }
  fileargs = fileargs.slice(0, -2) // remove trailing "||"
  const apiPath = getApiPath(shareHash == "" ? 'api/raw' : 'public/api/raw', {
    files: fileargs,
    algo: format,
    hash: shareHash,
    ...(state.share.token && { token: state.share.token }),
    sessionId: state.sessionId
  })
  const url = window.origin + apiPath

  // Create a direct link and trigger the download
  // This allows the browser to handle the download natively with:
  // - Native download progress indicator
  // - Shows up in browser's download menu
  // - Doesn't load entire file into memory first
  const link = document.createElement('a')
  link.href = url
  link.style.display = 'none'
  document.body.appendChild(link)
  link.click()
  
  // Clean up after a short delay
  setTimeout(() => {
    document.body.removeChild(link)
  }, 100)
}

export function post(
  source,
  path,
  content = "",
  overwrite = false,
  onupload,
  headers = {},
  isDir = false
) {
  if (!source || source === undefined || source === null) {
    throw new Error('no source provided')
  }
  try {
    const apiPath = getApiPath("api/resources", {
      path: doubleEncode(path),
      source: doubleEncode(source),
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

export async function moveCopy(
  items,
  action = 'copy',
  overwrite = false,
  rename = false
) {
  let params = {
    overwrite: overwrite,
    action: action,
    rename: rename
  }
  try {
    // Create an array of fetch calls
    let promises = items.map(item => {
      let localParams = {
        ...params,
        destination: doubleEncode(item.toSource) + '::' + doubleEncode(item.to),
        from: doubleEncode(item.fromSource) + '::' + doubleEncode(item.from)
      }
      const apiPath = getApiPath('api/resources', localParams)

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
    notify.showError(err.message || 'Error moving/copying resources')
    throw err // Re-throw the error to propagate it back to the caller
  }
}

export async function checksum(source, path, algo) {
  if (!source || source === undefined || source === null) {
    throw new Error('no source provided')
  }
  try {
    const params = {
      path: doubleEncode(path),
      source: doubleEncode(source),
      checksum: algo
    }
    const apiPath = getApiPath('api/resources', params)
    const res = await fetchURL(apiPath)
    const data = await res.json()
    return data.checksums[algo]
  } catch (err) {
    notify.showError(err.message || 'Error fetching checksum')
    throw err
  }
}

export function getDownloadURL(source, path, inline, useExternal) {
  if (!source || source === undefined || source === null) {
    throw new Error('no source provided')
  }
  try {
    const params = {
      files: encodeURIComponent(source) + '::' + encodeURIComponent(path),
      ...(inline && { inline: 'true' })
    }
    const apiPath = getApiPath('api/raw', params)
    if (globalVars.externalUrl && useExternal) {
      return globalVars.externalUrl + apiPath
    }
    return window.origin + apiPath
  } catch (err) {
    notify.showError(err.message || 'Error getting download URL')
    throw err
  }
}

export function getPreviewURL(source, path, modified) {
  if (!source || source === undefined || source === null) {
    throw new Error('no source provided')
  }
  try {
    const params = {
      path: encodeURIComponent(path),
      key: Date.parse(modified), // Use modified date as cache key
      source: encodeURIComponent(source),
      inline: 'true'
    }
    const apiPath = getApiPath('api/preview', params)
    return window.origin + apiPath
  } catch (err) {
    notify.showError(err.message || 'Error getting preview URL')
    throw err
  }
}

export async function sources() {
  try {
    const apiPath = getApiPath('api/jobs/status/sources')
    const res = await fetchURL(apiPath)
    const data = await res.json()
    // Return empty object if no sources are available - this is not an error
    return data || {}
  } catch (err) {
    // Only show error for actual network/server errors, not for empty sources
    if (err.status && err.status !== 200) {
      notify.showError(err.message || 'Error fetching usage sources')
    }
    throw err
  }
}

export async function GetOfficeConfig(req) {
  const params = {
    path: encodeURIComponent(req.path),
    ...(req.hash && { hash: encodeURIComponent(req.hash) }),
    ...(req.source && { source: encodeURIComponent(req.source) })
  }
  let apiPath = getApiPath('api/onlyoffice/config', params)
  if (req.hash) {
    apiPath = getPublicApiPath('onlyoffice/config', params)
  }
  const res = await fetchURL(apiPath)
  return await res.json()
}