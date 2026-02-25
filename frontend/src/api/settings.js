import { fetchURL, fetchJSON } from "./utils";
import { getApiPath } from "@/utils/url.js";
import { notify } from "@/notify";


export function get(property="") {
  const path = getApiPath("settings", { property });
  return fetchJSON(path);
}

export async function update(settings) {
  await fetchURL("api/settings", {
    method: "PUT",
    body: JSON.stringify(settings),
  });
}

export function config(showFull = false, showComments = false) {
  const params = {};
  if (showFull) params.full = "true";
  if (showComments) params.comments = "true";
  const path = getApiPath("settings/config", params);
  return fetchURL(path);
}

export async function sources() {
  try {
    const apiPath = getApiPath('settings/sources')
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
