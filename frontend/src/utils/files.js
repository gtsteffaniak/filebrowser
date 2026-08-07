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

export function formatDuration(seconds) {
  if (!Number.isFinite(seconds) || seconds < 0) {
    return '';
  }
  const totalSeconds = Math.floor(seconds);
  const hours = Math.floor(totalSeconds / 3600);
  const minutes = Math.floor((totalSeconds % 3600) / 60);
  const secs = totalSeconds % 60;
  if (hours > 0) {
    return `${hours}:${String(minutes).padStart(2, '0')}:${String(secs).padStart(2, '0')}`;
  }
  return `${minutes}:${String(secs).padStart(2, '0')}`;
}

export function getTypeFromMime(mimeType) {
  if (typeof mimeType !== 'string') return '';
  const parts = mimeType.split('/');
  return parts.length === 2 ? parts[1].toLowerCase() : '';
}

export function removeExtension(filename) {
  if (typeof filename !== 'string') return '';
  const lastDot = filename.lastIndexOf('.');
  if (lastDot === -1) return filename;
  if (lastDot === 0) return filename;
  return filename.substring(0, lastDot);
}
