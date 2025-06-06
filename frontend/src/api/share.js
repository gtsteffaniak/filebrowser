import { fetchURL, fetchJSON, adjustedData } from "./utils";
import { state } from '@/store'
import { notify } from "@/notify";
import { getApiPath,removePrefix } from "@/utils/url.js";
import { externalUrl,baseURL } from "@/utils/constants";

export async function list() {
  const apiPath = getApiPath("api/shares");
  return fetchJSON(apiPath);
}

export async function get(path, source) {
  try {
    const params = { path, source };
    const apiPath = getApiPath("api/share",params);
    let data = fetchJSON(apiPath);
    return adjustedData(data, path);
  } catch (err) {
    notify.showError(err.message || "Error fetching data");
    throw err;
  }
}

export async function remove(hash) {
  const params = { hash };
  const apiPath = getApiPath("api/share",params);
  await fetchURL(apiPath, {
    method: "DELETE",
  });
}

export async function create(path, source, password = "", expires = "", unit = "hours") {
  const params = { path: encodeURIComponent(path), source: source };
  const apiPath = getApiPath("api/share",params);
  let body = "{}";
  if (password != "" || expires !== "" || unit !== "hours") {
    body = JSON.stringify({ password: password, expires: expires, unit: unit });
  }
  return fetchJSON(apiPath, {
    method: "POST",
    body: body,
  });
}

export function getShareURL(share) {
  if (externalUrl) {
    const apiPath = getApiPath(`share/${share.hash}`)
    return externalUrl + removePrefix(apiPath, baseURL);
  }
  return window.origin+getApiPath(`share/${share.hash}`);
}

export function getPreviewURL(hash, path) {
  try {
    const params = {
      path: encodeURIComponent(path),
      size: 'small',
      hash: hash,
      inline: 'true'
    }
    const apiPath = getApiPath('api/public/preview', params)
    return window.origin + apiPath
  } catch (err) {
    notify.showError(err.message || 'Error getting preview URL')
    throw err
  }
}
