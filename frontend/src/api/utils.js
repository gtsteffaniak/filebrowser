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
    res = await fetch(`${baseURL}${url}`, {
      headers: {
        "X-Auth": state.jwt,
        "sessionId": state.sessionId,
        "userScope": state.user.scope,
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
  if (res.status === 200) {
    return res.json();
  } else {
    notify.showError("unable to fetch : " + url + "status" + res.status);
    throw new Error(res.status);
  }
}

export function removePrefix(url) {
  url = url.split("/").splice(2).join("/");
  if (url === "") url = "/";
  if (url[0] !== "/") url = "/" + url;
  return url;
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
