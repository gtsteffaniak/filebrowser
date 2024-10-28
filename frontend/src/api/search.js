import { fetchURL, removePrefix } from "./utils";
import { notify } from "@/notify";  // Import notify for error handling

export default async function search(base, query) {
  try {
    base = removePrefix(base);
    query = encodeURIComponent(query);

    if (!base.endsWith("/")) {
      base += "/";
    }

    const res = await fetchURL(`/api/search?scope=${base}&query=${query}`, {});
    let data = await res.json();

    return data
  } catch (err) {
    notify.showError(err.message || "Error occurred during search");
    throw err;
  }
}
