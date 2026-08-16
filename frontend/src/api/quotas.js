import { notify } from "@/notify";
import { getApiPath } from "@/utils/url.js";
import { fetchJSON, fetchURL } from "./utils";

/**
 * @param {string} source
 * @param {string} [path]
 */
export async function get(source, path) {
  try {
    const params = { source };
    if (path) params.path = path;
    return await fetchJSON(getApiPath("quotas", params));
  } catch (/** @type {any} */ err) {
    if (err.status !== 404) {
      notify.showError(err.message || "Error fetching quota");
    }
    throw err;
  }
}

/**
 * @param {Record<string, any>} body
 */
export async function create(body) {
  try {
    return await fetchJSON(getApiPath("quotas"), {
      method: "POST",
      body: JSON.stringify(body),
    });
  } catch (/** @type {any} */ err) {
    notify.showError(err.message || "Error creating quota");
    throw err;
  }
}

/**
 * @param {string} id
 * @param {Record<string, any>} body
 */
export async function update(id, body) {
  try {
    return await fetchJSON(getApiPath(`quotas/${id}`), {
      method: "PATCH",
      body: JSON.stringify(body),
    });
  } catch (/** @type {any} */ err) {
    notify.showError(err.message || "Error updating quota");
    throw err;
  }
}

/**
 * @param {string} id
 */
export async function remove(id) {
  try {
    await fetchURL(getApiPath(`quotas/${id}`), { method: "DELETE" });
  } catch (/** @type {any} */ err) {
    notify.showError(err.message || "Error deleting quota");
    throw err;
  }
}

/**
 * @param {string} username
 * @param {string} source
 */
export async function getUserScopeSnapshot(username, source) {
  try {
    const params = { source };
    return await fetchJSON(getApiPath(`users/${encodeURIComponent(username)}/quota-snapshot`, params));
  } catch (/** @type {any} */ err) {
    notify.showError(err.message || "Error fetching scope quota");
    throw err;
  }
}
