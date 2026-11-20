/**
 * Shared directory check for listing/selection items. Keeps download prompt selection,
 * chunked dispatch, and archive naming in agreement.
 */
export function isDirectoryItem(item) {
  return item?.isDir === true || item?.type === "directory";
}
