import { state } from "@/store";
import { renew, logout } from "@/utils/auth";
import { baseURL } from "@/utils/constants";
import { encodePath } from "@/utils/url.js";
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

export function createURL(endpoint, params = {}) {
  let prefix = baseURL;
  if (!prefix.endsWith("/")) {
    prefix = prefix + "/";
  }
  const url = new URL(prefix + endpoint, origin);

  const searchParams = {
    ...params,
  };

  for (const key in searchParams) {
    url.searchParams.set(key, searchParams[key]);
  }

  return url.toString();
}

export function adjustedData(data, url) {
  data.url = url;
  if (data.type == "directory") {
    if (!data.url.endsWith("/")) data.url += "/";
    data.items = data.items.map((item, index) => {
      item.index = index;
      item.url = `${data.url}${item.name}`;
      if (item.type == "directory") {
        item.url += "/";
      }
      return item;
    });
  }
  return data
}