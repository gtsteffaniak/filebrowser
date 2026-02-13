import { fetchURL } from "./utils";
import { notify } from "@/notify";  // Import notify for error handling
import { getApiPath } from "@/utils/url.js";

export default async function search(base, sources, query, largest = false) {
  try {
    // Ensure sources is an array
    const sourcesArray = Array.isArray(sources) ? sources : [sources];
    
    const params = {
      query: query,
      sources: sourcesArray.join(",")
    };

    // Only include scope if searching a single source
    // When multiple sources are specified, scope is always the user's scope for each source
    if (sourcesArray.length === 1 && base) {
      if (!base.endsWith("/")) {
        base += "/";
      }
      params.scope = base;
    }

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
      scope: base,
      source: source,
      minSizeMb: minSizeMb.toString()
    };

    if (useChecksum) {
      params.useChecksum = "true";
    }

    const apiPath = getApiPath("api/duplicates", params);
    const res = await fetchURL(apiPath);
    const data = await res.json();

    // Return both the data and metadata about completeness
    // Backend returns: { groups: [...], incomplete: bool, reason: string }
    return {
      groups: data.groups || data, // Handle both new format and legacy format
      incomplete: data.incomplete || false,
      reason: data.reason || ""
    };
  } catch (err) {
    notify.showError(err.message || "Error occurred while finding duplicates");
    throw err;
  }
}
