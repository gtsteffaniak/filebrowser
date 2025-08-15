import { fetchJSON } from './utils'
import { getApiPath } from '@/utils/url.js'

/**
 * @param {string} source
 * @param {string} path
 * @returns {Promise<any>}
 */
export async function get(source, path) {
  const apiPath = getApiPath('api/access', { source, path })
  return fetchJSON(apiPath)
}
/**
 * @param {string} source
 * @returns {Promise<any>}
 */
export async function getAll(source) {
  const apiPath = getApiPath('api/access', { source })
  return fetchJSON(apiPath)
}
/**
 * @returns {Promise<{groups: string[]}>}
 */
export async function getGroups() {
  const apiPath = getApiPath('api/access/groups')
  return fetchJSON(apiPath)
}
/**
 * @param {string} source
 * @param {string} path
 * @param {object} body
 * @returns {Promise<any>}
 */
export async function add(source, path, body) {
  const apiPath = getApiPath('api/access', { source, path });
  return fetchJSON(apiPath, {
    method: 'POST',
    body: JSON.stringify(body),
    headers: { 'Content-Type': 'application/json' }
  });
}
/**
 * @param {string} source
 * @param {string} path
 * @param {{ allow: boolean; ruleCategory: string; value: string; }} body
 * @returns {Promise<any>}
 */
export async function del(source, path, body) {
  const ruleType = body.allow ? 'allow' : 'deny';
  const { ruleCategory, value } = body;
  const apiPath = getApiPath('api/access', { source, path, ruleType, ruleCategory, value });
  return fetchJSON(apiPath, {
    method: 'DELETE'
  });
}
