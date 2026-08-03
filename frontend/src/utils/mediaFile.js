const MEDIA_EXTENSIONS = new Set([
  '.mp3', '.mp4', '.m4a', '.flac', '.wav', '.ogg', '.oga', '.opus',
  '.webm', '.mkv', '.avi', '.mov', '.wmv', '.m4v', '.aac', '.wma',
])

/** @param {string | undefined | null} typeOrName MIME type or file name */
export function isMediaFile(typeOrName) {
  if (!typeOrName || typeof typeOrName !== 'string') {
    return false
  }
  if (typeOrName.startsWith('audio/') || typeOrName.startsWith('video/')) {
    return true
  }
  const dot = typeOrName.lastIndexOf('.')
  if (dot < 0) {
    return false
  }
  return MEDIA_EXTENSIONS.has(typeOrName.slice(dot).toLowerCase())
}

/** @param {{ type?: string, name?: string, path?: string } | null | undefined} req */
export function isMediaRequest(req) {
  if (!req) {
    return false
  }
  return isMediaFile(req.type) || isMediaFile(req.name) || isMediaFile(req.path)
}
