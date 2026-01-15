import { fetchJSON } from './utils'
import { getApiPath } from '@/utils/url.js'

/**
 * @param {string} source
 * @param {string} path
 * @returns {Promise<any>}
 */
export async function get(source, path) {
  const apiPath = getApiPath('api/access', { source, path }, true)
  return fetchJSON(apiPath)
}
/**
 * @param {string} source
 * @returns {Promise<any>}
 */
export async function getAll(source) {
  const apiPath = getApiPath('api/access', { source }, true)
  return fetchJSON(apiPath)
}
/**
 * @returns {Promise<{groups: string[]}>}
 */
export async function getGroups() {
  const apiPath = getApiPath('api/access/groups', {}, true)
  return fetchJSON(apiPath)
}
/**
 * @param {string} source
 * @param {string} path
 * @param {object} body
 * @returns {Promise<any>}
 */
export async function add(source, path, body) {
  const apiPath = getApiPath('api/access', { source, path }, true);
  return fetchJSON(apiPath, {
    method: 'POST',
    body: JSON.stringify(body),
    headers: { 'Content-Type': 'application/json' }
  });
}
/**
 * @param {string} source
 * @param {string} path
 * @param {{ allow: boolean; ruleCategory: string; value: string; cascade?: boolean; }} body
 * @returns {Promise<any>}
 */
export async function del(source, path, body) {
  const ruleType = body.allow ? 'allow' : 'deny';
  const { ruleCategory, value, cascade } = body;
  const params = { source, path, ruleType, ruleCategory, value };
  if (cascade) {
    params.cascade = 'true';
  }
  const apiPath = getApiPath('api/access', params, true);
  return fetchJSON(apiPath, {
    method: 'DELETE'
  });
}
/**
 * @param {string} source
 * @param {string} oldPath
 * @param {string} newPath
 * @returns {Promise<any>}
 */
export async function updatePath(source, oldPath, newPath) {
  const apiPath = getApiPath('api/access', {}, true);
  return fetchJSON(apiPath, {
    method: 'PATCH',
    body: JSON.stringify({ source, oldPath, newPath }),
    headers: { 'Content-Type': 'application/json' }
  });
}
