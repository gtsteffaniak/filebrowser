import { fetchURL } from './utils'
import { getApiPath, getPublicApiPath } from '@/utils/url.js'
import { notify } from '@/notify'

// GET /api/office/config or /public/api/onlyoffice/config
export async function getConfig(req) {
  try {
    const params = {
      path: req.path,
      ...(req.hash && { hash: req.hash }),
      ...(req.source && { source: req.source })
    }
    
    let apiPath
    if (req.hash) {
      apiPath = getPublicApiPath('onlyoffice/config', params)
    } else {
      apiPath = getApiPath('api/office/config', params)
    }
    
    const res = await fetchURL(apiPath)
    return await res.json()
  } catch (err) {
    notify.showError(err.message || 'Error fetching OnlyOffice configuration')
    throw err
  }
}

// POST /api/office/callback or /public/api/onlyoffice/callback
export async function callback(params, hash = null) {
  try {
    let apiPath
    if (hash) {
      apiPath = getPublicApiPath('onlyoffice/callback', { hash, ...params })
    } else {
      apiPath = getApiPath('api/office/callback', params)
    }
    
    const res = await fetchURL(apiPath, {
      method: 'POST',
      headers: { 'Content-Type': 'application/json' },
      body: JSON.stringify(params)
    })
    return await res.json()
  } catch (err) {
    notify.showError(err.message || 'Error sending OnlyOffice callback')
    throw err
  }
}
