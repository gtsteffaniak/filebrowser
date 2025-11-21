
export function getFileExtension(filename) {
  if (typeof filename !== 'string') {
    return ''
  }
  const firstDotIndex = filename.indexOf('.')

  // If no dot exists, return an empty string
  if (firstDotIndex === -1) {
    return ""
  }

  // Default: Get everything after the first dot
  const firstDotExtension = filename.substring(firstDotIndex)

  if (firstDotExtension === '.') {
    return ""
  }
  // If it's 7 or fewer characters (including the dot), return it
  if (firstDotExtension.length <= 7) {
    return firstDotExtension
  }

  // Otherwise, return everything after the last dot
  const lastDotIndex = filename.lastIndexOf('.')
  return filename.substring(lastDotIndex)
}

export function removePrefix(filename, prefix = "") {
  if (filename === undefined) {
    return ""
  }
  if (filename.startsWith(prefix)) {
    filename = filename.slice(prefix.length);
  }
  return filename;
}
