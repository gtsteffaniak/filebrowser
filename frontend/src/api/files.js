import { fetchURL, adjustedData } from './utils'
import { getApiPath, extractSourceFromPath,removePrefix } from '@/utils/url.js'
import { state } from '@/store'
import { notify } from '@/notify'
import { externalUrl,baseURL,serverHasMultipleSources } from '@/utils/constants'

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

export async function put(path,source, content = '') {
  try {
    if (serverHasMultipleSources) {
      path = `/files/${source}${path}`
    } else {
      path = `/files${path}`
    }
    return await resourceAction(path, 'PUT', content)
  } catch (err) {
    notify.showError(err.message || 'Error putting resource')
    throw err
  }
}

export function download(format, files) {
  if (format !== 'zip') {
    format = 'tar.gz'
  }

  let fileargs = ''
  if (files.length === 1) {
    const result = extractSourceFromPath(decodeURI(files[0]))
    fileargs = result.source + '::' + result.path + '||'
  } else {
    for (let file of files) {
      const result = extractSourceFromPath(decodeURI(file))
      fileargs += result.source + '::' + result.path + '||'
    }
  }
  fileargs = fileargs.slice(0, -2) // remove trailing "||"
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
      const fromResult = extractSourceFromPath(item.from)
      const toResult = extractSourceFromPath(item.to)
      let localParams = {
        ...params,
        destination: toResult.source + '::' + toResult.path,
        from: fromResult.source + '::' + fromResult.path
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

export function getDownloadURL(source, path, inline, useExternal) {
  try {
    const params = {
      files: source + '::' + encodeURIComponent(path),
      ...(inline && { inline: 'true' })
    }
    const apiPath = getApiPath('api/raw', params)
    if (externalUrl && useExternal) {

      return externalUrl + removePrefix(apiPath, baseURL)
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
    return await res.json()
  } catch (err) {
    notify.showError(err.message || 'Error fetching usage sources')
    throw err
  }
}
