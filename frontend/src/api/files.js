import { fetchURL, adjustedData } from './utils'
import { getApiPath, doubleEncode } from '@/utils/url.js'
import { mutations } from '@/store'
import { notify } from '@/notify'
import { externalUrl } from '@/utils/constants'

// Notify if errors occur
export async function fetchFiles(source, path, content = false) {
  try {
    const apiPath = getApiPath('api/resources', {
      path: doubleEncode(path),
      source: doubleEncode(source),
      ...(content && { content: 'true' })
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
  try {
    source = doubleEncode(source)
    path = doubleEncode(path)
    const apiPath = getApiPath('api/resources', { path, source })
    let opts = { method }
    if (content) {
      opts.body = content
    }
    const res = await fetchURL(apiPath, opts)
    return res
  } catch (err) {
    notify.showError(err.message || 'Error performing resource action')
    throw err
  }
}

export async function remove(source, path) {
  try {
    return await resourceAction( source, path, 'DELETE')
  } catch (err) {
    notify.showError(err.message || 'Error deleting resource')
    throw err
  }
}

export async function put(source, path, content = '') {
  try {
    return await resourceAction(source, path, 'PUT', content)
  } catch (err) {
    notify.showError(err.message || 'Error putting resource')
    throw err
  }
}

export function download(format, files, shareHash = "") {
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
    hash: shareHash
  })
  const url = window.origin + apiPath

  // Create a temporary <a> element to trigger the download
  const link = document.createElement('a')
  link.href = url
  link.setAttribute('download', '') // Ensures it triggers a download
  document.body.appendChild(link)
  link.click()
  document.body.removeChild(link) // Clean up
}

export function post(
  source,
  path,
  content = "",
  overwrite = false,
  onupload,
  headers = {}
) {
  try {
    const apiPath = getApiPath("api/resources", {
      path: doubleEncode(path),
      source: doubleEncode(source),
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
    setTimeout(() => {
      notify.showSuccess(
        action === 'copy' ? 'Resources copied successfully' : 'Resources moved successfully'
      )
    }, 125);
    setTimeout(() => {
      mutations.setReload(true);
    }, 125);
  } catch (err) {
    notify.showError(err.message || 'Error moving/copying resources')
    throw err // Re-throw the error to propagate it back to the caller
  }
}

export async function checksum(source, path, algo) {
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
  try {
    const params = {
      files: encodeURIComponent(source) + '::' + encodeURIComponent(path),
      ...(inline && { inline: 'true' })
    }
    const apiPath = getApiPath('api/raw', params)
    if (externalUrl && useExternal) {
      return externalUrl + apiPath
    }
    return window.origin + apiPath
  } catch (err) {
    notify.showError(err.message || 'Error getting download URL')
    throw err
  }
}

export function getPreviewURL(source, path, modified) {
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
    return await res.json()
  } catch (err) {
    notify.showError(err.message || 'Error fetching usage sources')
    throw err
  }
}
