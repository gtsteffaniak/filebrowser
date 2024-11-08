import { fetchURL, fetchJSON, removePrefix, createURL } from "./utils";

export async function list() {
  return fetchJSON("/api/shares");
}

export async function get(url,hash) {
  url = removePrefix(url);
  return fetchJSON(`/api/public/share?path=${url}&hash=${hash}`);
}

export async function remove(hash) {
  await fetchURL(`/api/public/share?hash=${hash}`, {
    method: "DELETE",
  });
}

export async function create(url, password = "", expires = "", unit = "hours") {
  url = removePrefix(url);
  url = `/api/public/share?path=${url}`;
  expires = String(expires);
  if (expires !== "") {
    url += `&expires=${expires}&unit=${unit}`;
  }
  let body = "{}";
  if (password != "" || expires !== "" || unit !== "hours") {
    body = JSON.stringify({ password: password, expires: expires, unit: unit });
  }
  return fetchJSON(url, {
    method: "POST",
    body: body,
  });
}

export function getShareURL(share) {
  return createURL("share/" + share.hash, {}, false);
}
