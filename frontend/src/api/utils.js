import { state } from "@/store";
import { renew, logout } from "@/utils/auth";
import { baseURL } from "@/utils/constants";
import { encodePath } from "@/utils/url";
import { notify } from "@/notify";

export async function fetchURL(url, opts, auth = true) {
  opts = opts || {};
  opts.headers = opts.headers || {};

  let { headers, ...rest } = opts;

  let res;
  try {
    let userScope = "";
    if (state.user) {
      userScope = state.user.scope;
    }
    res = await fetch(url, {
      headers: {
        "sessionId": state.sessionId,
        "userScope": userScope,
        ...headers,
      },
      ...rest,
    });
  } catch (e) {
    console.error(e)
    const error = new Error("000 No connection");
    error.status = res.status;
    throw error;
  }

  if (auth && res.headers.get("X-Renew-Token") === "true") {
    console.log(auth,res.headers.get("X-Renew-Token"))
    await renew(state.jwt);
  }

  if (res.status < 200 || res.status > 299) {
    const error = new Error(await res.text());
    error.status = res.status;

    if (auth && res.status == 401) {
      logout();
    }

    throw error;
  }

  return res;
}

export async function fetchJSON(url, opts) {
  const res = await fetchURL(url, opts);
  if (res.status < 300) {
    return res.json();
  } else {
    notify.showError("received status: "+res.status+" on url " + url);
    throw new Error(res.status);
  }
}

export function removePrefix(path, prefix) {
  const combined = baseURL + prefix;
  // Check if path starts with the specified prefix followed by a '/'
  if (path.startsWith(combined + '/')) {
    // Remove the prefix by slicing it off
    path = path.slice(combined.length);
  }
  // Split the path, filter out any empty elements, and remove the first segment
  const parts = path.split('/').filter(Boolean);
  return '/' + parts.slice(1).join('/');
}

export function createURL(endpoint, params = {}) {
  let prefix = baseURL;
  if (!prefix.endsWith("/")) {
    prefix = prefix + "/";
  }
  const url = new URL(prefix + encodePath(endpoint), origin);

  const searchParams = {
    ...params,
  };

  for (const key in searchParams) {
    url.searchParams.set(key, searchParams[key]);
  }

  return url.toString();
}

// get path with parameters
export function getApiPath(path, params = {}) {
  if (path.startsWith("/")) {
    path = path.slice(1);
  }
  path = `${baseURL}${path}`;
  if (Object.keys(params).length > 0) {
    path += "?";
  }
  for (const key in params) {
    if (params[key] === undefined) {
      continue;
    }
    path += `${key}=${params[key]}&`;
  }
  // remove trailing &
  if (path.endsWith("&")) {
    path = path.slice(0, -1);
  }
  return path;
}

export function adjustedData(data, url) {
  data.url = url;
  if (data.type == "directory") {
    if (!data.url.endsWith("/")) data.url += "/";
    data.items = data.items.map((item, index) => {
      item.index = index;
      item.url = `${data.url}${encodeURIComponent(item.name)}`;
      if (item.type == "directory") {
        item.url += "/";
      }
      return item;
    });
  }
  return data
}