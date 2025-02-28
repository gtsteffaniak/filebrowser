import { fetchURL, adjustedData } from './utils'
import { getApiPath,extractSourceFromPath } from '@/utils/url.js'
import { state } from '@/store'
import { notify } from '@/notify'
import { externalUrl } from '@/utils/constants'

// Notify if errors occur
export async function fetchFiles(url, content = false) {
  try {
    const result = extractSourceFromPath(url)
    const apiPath = getApiPath('api/resources', {
      path: encodeURIComponent(result.path),
      source: result.source,
      ...(content && { content: 'true' })
    })
    const res = await fetchURL(apiPath)
    const data = await res.json()
    const adjusted = adjustedData(data, url)
    return adjusted
  } catch (err) {
    notify.showError(err.message || 'Error fetching data')
    throw err
  }
}

async function resourceAction(url, method, content) {
  try {
    const result = extractSourceFromPath(url)
    let source = result.source
    let path = result.path
    let opts = { method }
    if (content) {
      opts.body = content
    }
    path = encodeURIComponent(path)
    const apiPath = getApiPath('api/resources', { path: path, source: source })
    const res = await fetchURL(apiPath, opts)
    return res
  } catch (err) {
    notify.showError(err.message || 'Error performing resource action')
    throw err
  }
}

export async function remove(url) {
  try {
    return await resourceAction(url, 'DELETE')
  } catch (err) {
    notify.showError(err.message || 'Error deleting resource')
    throw err
  }
}

export async function put(url, content = '') {
  try {
    return await resourceAction(url, 'PUT', content)
  } catch (err) {
    notify.showError(err.message || 'Error putting resource')
    throw err
  }
}

export function download(format, files) {
  if (format !== 'zip') {
    format = 'tar.gz'
  }
  try {
    let fileargs = ''
    if (files.length === 1) {
      const result = extractSourceFromPath(decodeURI(files[0]))
      fileargs = result.source + "::" + result.path + ',|'
    } else {
      for (let file of files) {
        const result = extractSourceFromPath(decodeURI(file))
        fileargs += result.source + "::" + result.path + ',|'
      }
    }
    fileargs = fileargs.slice(0, -2); // remove trailing ",|"
    const apiPath = getApiPath('api/raw', {
      files: encodeURIComponent(fileargs),
      algo: format
    })
    const url = window.origin + apiPath

    // Create a temporary <a> element to trigger the download
    const link = document.createElement('a')
    link.href = url
    link.setAttribute('download', '') // Ensures it triggers a download
    document.body.appendChild(link)
    link.click()
    document.body.removeChild(link) // Clean up
  } catch (err) {
    notify.showError(err.message || 'Error downloading files')
  }
}

export async function post(url, content = '', overwrite = false, onupload) {
  try {
    const result = extractSourceFromPath(url)
    let bufferContent
    if (
      content instanceof Blob &&
      !['http:', 'https:'].includes(window.location.protocol)
    ) {
      bufferContent = await new Response(content).arrayBuffer()
    }

    const apiPath = getApiPath('api/resources', {
      path: result.path,
      source: result.source,
      override: overwrite
    })
    return new Promise((resolve, reject) => {
      let request = new XMLHttpRequest()
      request.open('POST', apiPath, true)
      request.setRequestHeader('X-Auth', state.jwt)

      if (typeof onupload === 'function') {
        request.upload.onprogress = event => {
          if (event.lengthComputable) {
            const percentComplete = Math.round(
              (event.loaded / event.total) * 100
            )
            onupload(percentComplete) // Pass the percentage to the callback
          }
        }
      }

      request.onload = () => {
        if (request.status === 200) {
          resolve(request.responseText)
        } else if (request.status === 409) {
          reject(request.status)
        } else {
          reject(request.responseText)
        }
      }
      request.send(bufferContent || content)
    })
  } catch (err) {
    notify.showError(err.message || 'Error posting resource')
    throw err
  }
}

export async function moveCopy (
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
      let topath = decodeURI(item.to)
      let frompath = decodeURI(item.from)
      const resultfrom = extractSourceFromPath(frompath)
      const resultto = extractSourceFromPath(topath)

      // Properly declare variables
      let toPath = encodeURIComponent(resultto.path)
      let fromPath = encodeURIComponent(resultfrom.path)

      // Ensure 'source' is correctly referenced
      let localParams = { ...params, destination: toPath, from: fromPath, source: resultfrom.source }
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

export async function checksum(url, algo) {
  try {
    const result = extractSourceFromPath(url)
    const params = {
      path: encodeURIComponent(result.path),
      source: result.source,
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

export function getDownloadURL(path, inline, useExternal) {
  try {
    const result = extractSourceFromPath(decodeURI(path))
    const params = {
      files: encodeURIComponent(result.source + "==" +result.path),
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

export function getPreviewURL(source, path, size, modified) {
  try {
    const params = {
      path: encodeURIComponent(path),
      size: size,
      key: Date.parse(modified),
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

export function getSubtitlesURL(path) {
  const result = extractSourceFromPath(decodeURI(path))
  const params = {
    inline: true,
    files: result.source + "::" +result.path,
  }
  const apiPath = getApiPath('api/raw', params)
  return window.origin + apiPath
}

export async function usage(source) {
  try {
    const apiPath = getApiPath('api/usage', { source: source })
    const res = await fetchURL(apiPath)
    return await res.json()
  } catch (err) {
    notify.showError(err.message || 'Error fetching usage data')
    throw err
  }
}
