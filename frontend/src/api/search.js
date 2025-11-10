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
