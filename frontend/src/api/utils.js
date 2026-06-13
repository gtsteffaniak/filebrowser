import i18n from "@/i18n";
import { state } from "@/store";
import { renew } from "@/utils/auth";

export async function fetchURL(url, opts, auth = true) {
  opts = opts || {};
  opts.headers = opts.headers || {};

  const { headers, ...rest } = opts;

  let res;
  try {
    res = await fetch(url, {
      credentials: 'same-origin', // Ensure cookies are sent with all API requests
      headers: {
        "sessionId": state.sessionId,
        ...headers,
      },
      ...rest,
    });
  } catch (e) {
    let message = e.message;
    if (e instanceof TypeError && e.message === "Failed to fetch") {
      message = i18n.global.t("errors.failedToConnectToServer");
    }
    const error = new Error(message);
    throw error;
  }

  if (auth && res.headers.get("X-Renew-Token") === "true") {
    // Cookie is automatically sent, no need to pass JWT from state
    await renew();
  }

  if (res.status < 200 || res.status > 299) {
    const error = new Error(await res.text());
    error.status = res.status;
    throw error;
  }

  return res;
}

export async function fetchJSON(url, opts) {
  const res = await fetchURL(url, opts);
  if (res.status < 300) {
    return res.json();
  } else {
    throw new Error(res.status);
  }
}

export function adjustedData(data) {
  if (data.type === "directory") {
    const pinnedNames = new Set(data.pinnedItems || []);
    // Combine folders and files into items
    data.items = [...(data.folders || []), ...(data.files || [])];
    data.items = data.items.map((item) => {
      item.source = data.source
      if (item.isShared === undefined) {
        item.isShared = false;
      }
      item.pinned = pinnedNames.has(item.name);
      if (data.path === "/") {
        if (item.type === "directory") {
        item.path = `/${item.name}/`
        } else {
          item.path = `/${item.name}`
        }
      } else {
        if (item.type === "directory") {
          item.path = `${data.path}${item.name}/`
        } else {
          item.path = `${data.path}${item.name}`
        }
      }
      return item;
    });
    delete data.pinnedItems;
  }
  if (data.files) {
    data.files = []
  }
  if (data.folders) {
    data.folders = []
  }
  return data;
}

