import { fetchURL, removePrefix } from "./utils";
import url from "../utils/url";
import { notify } from "@/notify";  // Import notify for error handling

export default async function search(base, query) {
  try {
    base = removePrefix(base);
    query = encodeURIComponent(query);

    if (!base.endsWith("/")) {
      base += "/";
    }

    const res = await fetchURL(`/api/search${base}?query=${query}`, {});

    let data = await res.json();

    data = data.map((item) => {
      item.url = `/files${base}` + url.encodePath(item.path);
      return item;
    });

    return data;
  } catch (err) {
    notify.showError(err.message || "Error occurred during search");
    throw err;
  }
}
