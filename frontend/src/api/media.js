import { fetchURL } from "./utils";
import { notify } from "@/notify";
import { getApiPath } from "@/utils/url.js";

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