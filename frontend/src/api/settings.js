import { fetchURL, fetchJSON } from "./utils";
import { getApiPath } from "@/utils/url.js";


export function get(property="") {
  const path = getApiPath("api/settings", { property });
  return fetchJSON(path);
}

export async function update(settings) {
  await fetchURL("api/settings", {
    method: "PUT",
    body: JSON.stringify(settings),
  });
}

export function getConfig(showFull = false, showComments = false) {
  const params = {};
  if (showFull) params.full = "true";
  if (showComments) params.comments = "true";
  const path = getApiPath("api/settings/config", params);
  return fetchURL(path);
}
