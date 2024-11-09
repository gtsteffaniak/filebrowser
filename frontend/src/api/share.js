import { fetchURL, fetchJSON, createURL } from "./utils";

export async function list() {
  return fetchJSON("/api/shares");
}

export async function get(path, hash) {
  const params = { path, hash };
  const url = createURL(`api/share`, params, false);
  return fetchJSON(url);
}

export async function remove(hash) {
  const params = { hash };
  const url = createURL(`api/share`, params, false);
  await fetchURL(url, {
    method: "DELETE",
  });
}

export async function create(path, password = "", expires = "", unit = "hours") {
  const params = { path };
  const url = createURL(`api/share`, params, false);
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
