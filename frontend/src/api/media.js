import { fetchURL, adjustedData } from "./utils";
import { notify } from "@/notify";
import { getApiPath, getPublicApiPath } from "@/utils/url.js";
import { state } from "@/store";

// GET /api/media/subtitles
export async function getSubtitleContent(source, path, subtitleName, embedded = false) {
  try {
    const apiPath = getApiPath('media/subtitles', {
      source: source,
      path: path,
      name: subtitleName,
      embedded: embedded.toString()
    })
    const res = await fetchURL(apiPath)
    const content = await res.text()
    return content
  } catch (err) {
    notify.showError(err.message || `Error fetching subtitle ${subtitleName}`)
    throw err
  }
}

// GET /api/media/metadata — same directory shape as resources with metadata=true (adjusted to items[]).
/** @returns {Promise<object>} directory resource (adjustedData) */
export async function fetchDirectoryMediaMetadata(source, path) {
  const apiPath = getApiPath("media/metadata", { source, path });
  const res = await fetchURL(apiPath);
  const data = await res.json();
  return adjustedData(data);
}

// GET /public/api/media/metadata
/** @returns {Promise<object>} directory resource (adjustedData) */
export async function fetchDirectoryMediaMetadataPublic(path, hash, password = "") {
  const params = {
    path,
    hash,
    ...(state.shareInfo.token && { token: state.shareInfo.token }),
  };
  const apiPath = getPublicApiPath("media/metadata", params);
  const response = await fetch(apiPath, {
    headers: { "X-SHARE-PASSWORD": password || "" },
  });
  if (!response.ok) {
    const error = new Error(response.statusText);
    let data = null;
    try {
      data = await response.json();
    } catch (e) {
      // ignore
    }
    if (data?.message) {
      error.message = data.message;
    }
    /** @type {any} */ (error).status = response.status;
    throw error;
  }
  const data = await response.json();
  return adjustedData(data);
}