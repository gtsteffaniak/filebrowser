import { notify } from "@/notify/index.ts"
import { getApiPath } from '@/utils/url.js'
import { fetchURL } from "./utils.ts"

// GET /api/wopi/session: the editor URL and the access token the browser
// form-posts to it.
export async function getSession(req) {
  try {
    const apiPath = getApiPath('wopi/session', { source: req.source, path: req.path })
    const res = await fetchURL(apiPath, {})
    return await res.json()
  } catch (err) {
    notify.showError(err.message || 'Error opening the document editor')
    throw err
  }
}
