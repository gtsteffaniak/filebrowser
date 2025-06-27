import { fetchURL, fetchJSON, adjustedData } from './utils'
import { notify } from '@/notify'
import { getApiPath, removePrefix } from '@/utils/url.js'
import { externalUrl, baseURL } from '@/utils/constants'

export async function get(source, path) {
  const apiPath = getApiPath('api/access', { source, path })
  return fetchJSON(apiPath)
}
export async function getAll(source) {
  const apiPath = getApiPath('api/access', { source })
  return fetchJSON(apiPath)
}
export async function add(source, path, body) {
  const apiPath = getApiPath('api/access', { source, path });
  return fetchJSON(apiPath, {
    method: 'POST',
    body: JSON.stringify(body),
    headers: { 'Content-Type': 'application/json' }
  });
}
export async function del(source, path, body) {
  const apiPath = getApiPath('api/access', { source, path });
  return fetchJSON(apiPath, {
    method: 'DELETE',
    body: JSON.stringify(body),
    headers: { 'Content-Type': 'application/json' }
  });
}
