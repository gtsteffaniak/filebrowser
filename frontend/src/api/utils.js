import { state } from "@/store";
import { renew, logout } from "@/utils/auth";
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
    let message = e;
    if (e == "TypeError: Failed to fetch") {
      message = "Failed to connect to the server, is it still running?";
    }
    const error = new Error(message);
    throw error;
  }

  if (auth && res.headers.get("X-Renew-Token") === "true") {
    await renew(state.jwt);
  }

  if (res.status < 200 || res.status > 299) {
    let error = new Error(await res.text());
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

export function adjustedData(data, url) {
  data.url = url;
  if (data.type === "directory") {
    if (!data.url.endsWith("/")) data.url += "/";

    // Combine folders and files into items
    data.items = [...(data.folders || []), ...(data.files || [])];

    data.items = data.items.map((item, index) => {
      item.url = `${data.url}${item.name}`;
      if (item.type === "directory") {
        item.url += "/";
      }
      return item;
    });
  }
  if (data.files) {
    data.files = []
  }
  if (data.folders) {
    data.folders = []
  }
  return data;
}
