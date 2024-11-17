import { fetchURL, removePrefix, getApiPath } from "./utils";
import { notify } from "@/notify";  // Import notify for error handling

export default async function search(base, query) {
  try {
    base = removePrefix(base,"files");
    query = encodeURIComponent(query);

    if (!base.endsWith("/")) {
      base += "/";
    }

    const apiPath = getApiPath("api/search", { scope: base, query: query });
    const res = await fetchURL(apiPath);
    let data = await res.json();

    return data
  } catch (err) {
    notify.showError(err.message || "Error occurred during search");
    throw err;
  }
}
