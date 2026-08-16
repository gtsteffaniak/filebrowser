import { notify } from "@/notify";
import { getApiPath } from "@/utils/url.js";
import { fetchJSON, fetchURL } from "./utils";

/**
 * @param {string} source
 * @param {string} [path]
 * @param {string} [username]
 */
export async function get(source, path, username) {
  try {
    const params = { source };
    if (path) params.path = path;
    if (username) params.username = username;
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
 * @param {string} source
 * @param {string} path
 * @param {Record<string, any>} body
 * @param {string} [username]
 */
export async function update(source, path, body, username) {
  try {
    const payload = { source, path, ...body };
    if (username) payload.username = username;
    return await fetchJSON(getApiPath("quotas"), {
      method: "PATCH",
      body: JSON.stringify(payload),
    });
  } catch (/** @type {any} */ err) {
    notify.showError(err.message || "Error updating quota");
    throw err;
  }
}

/**
 * @param {string} source
 * @param {string} path
 * @param {string} [username]
 */
export async function remove(source, path, username) {
  try {
    const params = { source, path };
    if (username) params.username = username;
    await fetchURL(getApiPath("quotas", params), { method: "DELETE" });
  } catch (/** @type {any} */ err) {
    notify.showError(err.message || "Error deleting quota");
    throw err;
  }
}
