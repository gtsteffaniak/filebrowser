import { fetchURL, fetchJSON, createURL, adjustedData } from "./utils";
import { notify } from "@/notify";
import { removePrefix, getApiPath } from "@/utils/url.js";

export async function list() {
  const apiPath = getApiPath("api/shares");
  return fetchJSON(apiPath);
}

export async function get(path, hash) {
  try {
    const params = { path, hash };
    const apiPath = getApiPath("api/share",params);
    let data = fetchJSON(apiPath);
    return adjustedData(data, `api/share${path}`);
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

export async function create(path, password = "", expires = "", unit = "hours") {
  const params = { path };
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
  return createURL("share/" , {}, false);
}
