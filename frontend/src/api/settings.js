import { fetchURL, fetchJSON } from "./utils";
import { getApiPath } from "@/utils/url.js";

const apiPath = getApiPath("api/settings");

export function get() {
  return fetchJSON(apiPath);
}

export async function update(settings) {

  await fetchURL(apiPath, {
    method: "PUT",
    body: JSON.stringify(settings),
  });
}
