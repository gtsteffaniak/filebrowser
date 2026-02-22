import { fetchURL, adjustedData } from './utils'
import { getApiPath, getPublicApiPath } from '@/utils/url.js'
import { state, mutations } from '@/store'
import { notify } from '@/notify'
import { globalVars } from '@/utils/constants'
import { downloadManager } from '@/utils/downloadManager'

// Notify if errors occur
export async function fetchFiles(source, path, content = false, metadata = false) {
  if (!source || source === undefined || source === null) {
    throw new Error('no source provided')
  }
  try {
    const apiPath = getApiPath('api/resources', {
      path: path,
      source: source,
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

export async function getItems(source, path, only = "") {
  if (!source || source === undefined || source === null) {
    throw new Error('no source provided')
  }
  try {
    const apiPath = getApiPath('api/resources/items', {
      path: path,
      source: source,
      ...(only && { only: only }),
    })
    const res = await fetchURL(apiPath)
    const data = await res.json()
    return data
  } catch (err) {
    notify.showError(err.message || 'Error fetching items')
    throw err
  }
}

async function resourceAction(source, path, method, content) {
  if (!source || source === undefined || source === null) {
    throw new Error('no source provided')
  }
  try {
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

export async function bulkDelete(items) {
  if (!items || !Array.isArray(items) || items.length === 0) {
    throw new Error('items array is required and must not be empty')
  }
  try {
    const apiPath = getApiPath('api/resources/bulk')
    const response = await fetchURL(apiPath, {
      method: 'DELETE',
      headers: {
        'Content-Type': 'application/json',
      },
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
    // Only show notification and re-throw if it's a real error (not 200/207)
    if (err.status && err.status !== 200 && err.status !== 207) {
      notify.showError(err.message || 'Error performing bulk delete')
    }
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
  // Check if chunked download should be used (single file only)
  const downloadChunkSizeMb = state.user?.fileLoading?.downloadChunkSizeMb || 0
  const sizeThreshold = downloadChunkSizeMb * 1024 * 1024;

  const useChunkedDownload =
    downloadChunkSizeMb > 0 &&
    files.length === 1 &&
    !files[0].isDir &&
    files[0].size &&
    files[0].size >= sizeThreshold

  if (useChunkedDownload) {
    // Use chunked download for large single files
    return await downloadChunked(files[0], shareHash)
  }

  // Normal download (archive or small files)
  if (format !== 'zip') {
    format = 'tar.gz'
  }

  // For non-share downloads, validate single source and build file list
  let source = null
  let filePaths = []

  if (shareHash) {
    // For shares, no source parameter needed, just paths
    filePaths = files.map(file => file.path)
  } else {
    // Validate all files are from the same source
    for (let file of files) {
      if (!file.source) {
        throw new Error('File source is required for downloads')
      }
      if (source === null) {
        source = file.source
      } else if (source !== file.source) {
        throw new Error('All files must be from the same source for downloads')
      }
      filePaths.push(file.path)
    }
  }

  const params = {
    file: filePaths, // Array of file paths - getApiPath will create repeated parameters
    algo: format,
    ...(shareHash && { hash: shareHash }),
    ...(!shareHash && source && { source: source }),
    ...(state.shareInfo.token && { token: state.shareInfo.token }),
    sessionId: state.sessionId
  }

  const apiPath = getApiPath(shareHash == "" ? 'api/raw' : 'public/api/raw', params)
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

async function downloadChunked(file, shareHash = "") {
  const chunkSizeMb = state.user?.fileLoading?.downloadChunkSizeMb || 0

  if (chunkSizeMb === 0) {
    throw new Error("Chunked download is disabled (chunk size is 0)")
  }
  const chunkSize = chunkSizeMb * 1024 * 1024 // Convert MB to bytes
  const fileSize = file.size

  // Extract filename from path if name is not available
  const fileName = file.name || (file.path ? file.path.split('/').pop() : 'download')

  // Add to download manager
  const downloadId = downloadManager.add(file, shareHash)

  downloadManager.setStatus(downloadId, "downloading")

  // Show download prompt if not already shown (it should already be shown by downloadFiles, but check to be safe)
  const hasDownloadPrompt = state.hovers && state.hovers.some(h => h.name === 'download');

  if (!hasDownloadPrompt) {
    mutations.showHover({ name: 'download' })
  }

  const params = {
    file: file.path,
    ...(shareHash && { hash: shareHash }),
    ...(!shareHash && file.source && { source: file.source }),
    ...(state.shareInfo.token && { token: state.shareInfo.token }),
    sessionId: state.sessionId
  }

  const apiPath = getApiPath(shareHash == "" ? 'api/raw' : 'public/api/raw', params)
  const baseUrl = window.origin + apiPath

  const download = downloadManager.findById(downloadId)
  const abortController = new AbortController()
  download.abortController = abortController

  try {
    // Download file in chunks
    const chunks = []
    let offset = 0
    let loaded = 0

    while (offset < fileSize) {

      const download = downloadManager.findById(downloadId);
      if (download && download.status === "cancelled") {
        // Silently handle cancellation - don't throw error
        return;
      }

      const end = Math.min(offset + chunkSize - 1, fileSize - 1)
      const rangeHeader = `bytes=${offset}-${end}`

      const response = await fetch(baseUrl, {
        headers: {
          'Range': rangeHeader
        },
        credentials: 'same-origin',
        signal: abortController.signal
      })

      if (!response.ok && response.status !== 206) {
        throw new Error(`Failed to download chunk: ${response.statusText}`)
      }

      // Track progress within the chunk using ReadableStream
      const expectedChunkSize = end - offset + 1;

      const reader = response.body.getReader();
      const chunkParts = [];
      let chunkLoaded = 0;
      let lastProgressUpdate = 0;
      const progressUpdateInterval = Math.max(50000, expectedChunkSize / 50); // Update every ~2% of chunk or 50KB

      try {
        let reading = true;
        while (reading) {
          const { done, value } = await reader.read();
          if (done) {
            reading = false;
            break;
          }

          chunkParts.push(value);
          chunkLoaded += value.length;

          // Calculate progress: only count up to expected chunk size to avoid over-counting
          const chunkProgress = Math.min(chunkLoaded, expectedChunkSize);
          const totalLoaded = offset + chunkProgress;

          // Update progress in real-time, but throttle updates for performance
          if (chunkLoaded - lastProgressUpdate >= progressUpdateInterval || chunkLoaded >= expectedChunkSize) {
            downloadManager.updateProgress(downloadId, totalLoaded, fileSize);
            lastProgressUpdate = chunkLoaded;
          }
        }
      } catch (readError) {
        // If read was aborted, check if download was cancelled
        const download = downloadManager.findById(downloadId);
        if (readError.name === 'AbortError' || (download && download.status === "cancelled")) {
          downloadManager.setStatus(downloadId, "cancelled");
          return; // Silently handle cancellation
        }
        throw readError; // Re-throw other errors
      }

      // Combine chunk parts into single ArrayBuffer
      const chunk = new Uint8Array(chunkLoaded);
      let position = 0;
      for (const part of chunkParts) {
        chunk.set(part, position);
        position += part.length;
      }

      // Only use the expected chunk size portion if server returned more (handles Range header issues)
      const chunkToUse = chunk.byteLength > expectedChunkSize
        ? chunk.slice(0, expectedChunkSize).buffer
        : chunk.buffer;

      chunks.push(chunkToUse)
      // Always use expected chunk size for progress to avoid double-counting
      loaded += expectedChunkSize

      // Final progress update for this chunk
      downloadManager.updateProgress(downloadId, loaded, fileSize)

      offset = end + 1
    }

    // Combine all chunks into a single blob
    const blob = new Blob(chunks, { type: 'application/octet-stream' })
    const blobUrl = URL.createObjectURL(blob)

    // Trigger download
    const link = document.createElement('a')
    link.href = blobUrl
    link.download = fileName
    link.style.display = 'none'
    document.body.appendChild(link)
    link.click()

    // Mark as completed
    downloadManager.setStatus(downloadId, "completed")
    downloadManager.updateProgress(downloadId, fileSize, fileSize)

    // Clean up
    setTimeout(() => {
      document.body.removeChild(link)
      URL.revokeObjectURL(blobUrl)
    }, 100)
  } catch (error) {
    // Check if download was cancelled by user
    const download = downloadManager.findById(downloadId);
    if (error.name === 'AbortError' || (download && download.status === "cancelled")) {
      downloadManager.setStatus(downloadId, "cancelled")
      // Don't throw error or show notification for user-initiated cancellation
      return;
    }
    downloadManager.setError(downloadId, error.message || 'Download failed')
    notify.showError(`Chunked download failed: ${error.message}`)
    throw error
  }
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
      path: path,
      source: source,
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
  if (!items || !Array.isArray(items) || items.length === 0) {
    throw new Error('items array is required and must not be empty')
  }

  try {
    // Build request body with proper format
    const requestBody = {
      items: items.map(item => ({
        fromSource: item.fromSource,
        fromPath: item.from,
        toSource: item.toSource,
        toPath: item.to
      })),
      action: action,
      overwrite: overwrite,
      rename: rename
    }

    const apiPath = getApiPath('api/resources')
    // We use fetch directly here instead of fetchURL because fetchURL throws on non-2xx status,
    // consuming the response body as text. We need to parse the JSON response for 500/207 errors.
    // We need to manually add headers that fetchURL adds.
    const headers = {
      'Content-Type': 'application/json',
      'sessionId': state.sessionId
    }

    const response = await fetch(apiPath, {
      method: 'PATCH',
      headers: headers,
      body: JSON.stringify(requestBody),
      credentials: 'same-origin'
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
    // For 500 errors with response data, don't show notification here
    // Let the caller handle it with the attached error data
    // Only show notification for unexpected errors (no status or not 500)
    if (!err.status || (err.status !== 500 && err.status !== 200 && err.status !== 207)) {
      notify.showError(err.message || 'Error moving/copying resources')
    }
    throw err
  }
}

export async function checksum(source, path, algo) {
  if (!source || source === undefined || source === null) {
    throw new Error('no source provided')
  }
  try {
    const params = {
      path: path,
      source: source,
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
      source: source,
      file: path,
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
      path: path,
      key: Date.parse(modified), // Use modified date as cache key
      source: source,
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
    path: req.path,
    ...(req.hash && { hash: req.hash }),
    ...(req.source && { source: req.source })
  }
  let apiPath = getApiPath('api/onlyoffice/config', params)
  if (req.hash) {
    apiPath = getPublicApiPath('onlyoffice/config', params)
  }
  const res = await fetchURL(apiPath)
  return await res.json()
}

export async function getSubtitleContent(source, path, subtitleName, embedded = false) {
  try {
    const apiPath = getApiPath('api/media/subtitles', {
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