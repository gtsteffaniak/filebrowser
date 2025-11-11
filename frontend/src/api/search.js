import { fetchURL } from "./utils";
import { notify } from "@/notify";  // Import notify for error handling
import { getApiPath } from "@/utils/url.js";

export default async function search(base, source, query, largest = false) {
  try {
    query = encodeURIComponent(query);
    if (!base.endsWith("/")) {
      base += "/";
    }
    const params = {
      scope: encodeURIComponent(base),
      query: query,
      source: encodeURIComponent(source)
    };

    if (largest) {
      params.largest = "true";
    }

    const apiPath = getApiPath("api/search", params);
    const res = await fetchURL(apiPath);
    let data = await res.json();

    return data
  } catch (err) {
    notify.showError(err.message || "Error occurred during search");
    throw err;
  }
}

export async function findDuplicates(base, source, minSizeMb, useChecksum = false) {
  try {
    if (!base.endsWith("/")) {
      base += "/";
    }
    const params = {
      scope: encodeURIComponent(base),
      source: encodeURIComponent(source),
      minSizeMb: minSizeMb.toString()
    };

    if (useChecksum) {
      params.useChecksum = "true";
    }

    const apiPath = getApiPath("api/duplicates", params);
    const res = await fetchURL(apiPath);
    const data = await res.json();

    return data;
  } catch (err) {
    notify.showError(err.message || "Error occurred while finding duplicates");
    throw err;
  }
}
