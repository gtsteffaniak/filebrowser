import { fetchURL, fetchJSON, adjustedData } from "./utils";
import { notify } from "@/notify";
import { getApiPath } from "@/utils/url.js";
import { externalUrl } from "@/utils/constants";

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
  const params = { path: path, source: source };
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
    return externalUrl+getApiPath(`share/${share.hash}`);
  }
  return window.origin+getApiPath(`share/${share.hash}`);
}
