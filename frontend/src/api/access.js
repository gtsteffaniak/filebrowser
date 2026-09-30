import { getApiPath } from '@/utils/url.js'
import { fetchJSON } from './utils'

/**
 * @param {string} source
 * @param {string} path
 * @returns {Promise<any>}
 */
export async function get(source, path) {
  const apiPath = getApiPath('access', { source, path })
  return fetchJSON(apiPath)
}
/**
 * @param {string} source
 * @returns {Promise<any>}
 */
export async function getAll(source) {
  const apiPath = getApiPath('access', { source })
  return fetchJSON(apiPath)
}
/**
 * @returns {Promise<{groups: string[]}>}
 */
export async function getGroups() {
  const apiPath = getApiPath('access/groups', {})
  return fetchJSON(apiPath)
}
/**
 * @param {string} username
 * @returns {Promise<{groups: string[]}>}
 */
export async function getUserGroups(username) {
  const apiPath = getApiPath('access/groups', { user: username })
  return fetchJSON(apiPath)
}
/**
 * @param {string} group
 * @param {string} username
 * @returns {Promise<any>}
 */
export async function addUserToGroup(group, username) {
  const apiPath = getApiPath('access/group', { group, user: username })
  return fetchJSON(apiPath, { method: 'POST' })
}
/**
 * @param {string} group
 * @param {string} username
 * @returns {Promise<any>}
 */
export async function removeUserFromGroup(group, username) {
  const apiPath = getApiPath('access/group', { group, user: username })
  return fetchJSON(apiPath, { method: 'DELETE' })
}
/**
 * @returns {Promise<{groups: string[], members: Record<string, string[]>}>}
 */
export async function getGroupsWithMembers() {
  const apiPath = getApiPath('access/groups', { members: 'true' })
  return fetchJSON(apiPath)
}
/**
 * Creates the group if needed and replaces its full member list.
 * @param {string} group
 * @param {string[]} members
 * @returns {Promise<any>}
 */
export async function saveGroup(group, members) {
  const apiPath = getApiPath('access/group', {})
  return fetchJSON(apiPath, {
    method: 'PUT',
    body: JSON.stringify({ group, members }),
  })
}
/**
 * Deletes a group and removes it from every access rule.
 * @param {string} group
 * @returns {Promise<any>}
 */
export async function deleteGroup(group) {
  const apiPath = getApiPath('access/group', { group })
  return fetchJSON(apiPath, { method: 'DELETE' })
}
/**
 * @param {string} source
 * @param {string} path
 * @param {object} body
 * @returns {Promise<any>}
 */
export async function add(source, path, body) {
  const apiPath = getApiPath('access', { source, path });
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
  const apiPath = getApiPath('access', params);
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
  const apiPath = getApiPath('access', {});
  return fetchJSON(apiPath, {
    method: 'PATCH',
    body: JSON.stringify({ source, oldPath, newPath }),
    headers: { 'Content-Type': 'application/json' }
  });
}
