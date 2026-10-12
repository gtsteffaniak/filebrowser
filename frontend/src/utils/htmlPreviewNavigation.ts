// Sandboxed srcdoc frames have an opaque ("null") origin. Check the exact
// WindowProxy as well as the destination; never authorize messages by origin alone.
export function previewNavigationFile(
  event: MessageEvent,
  frameWindow: Window | null,
  origin: string,
  viewerPrefix: string,
): string | null {
  if (!frameWindow || event.source !== frameWindow || event.origin !== "null") return null;
  const data = event.data;
  if (data?.type !== "filebrowser-html-navigation" || typeof data.href !== "string") return null;
  try {
    const url = new URL(data.href);
    const prefix = viewerPrefix.endsWith("/") ? viewerPrefix : `${viewerPrefix}/`;
    if (url.origin !== origin || !url.pathname.startsWith(prefix) || url.username || url.password) return null;
    if (url.search || url.hash) return null;
    const file = `/${decodeURIComponent(url.pathname.slice(prefix.length))}`;
    // biome-ignore lint/suspicious/noControlCharactersInRegex: Reject control characters in navigation paths.
    if (!file?.startsWith("/") || !/\.html?$/i.test(file) || /[\x00-\x1f\\]/.test(file)) return null;
    if (file.split("/").some((part) => part === "." || part === "..")) return null;
    return file;
  } catch {
    return null;
  }
}
