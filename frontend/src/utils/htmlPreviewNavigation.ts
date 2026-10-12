// Sandboxed srcdoc frames have an opaque ("null") origin. Check the exact
// WindowProxy as well as the destination; never authorize messages by origin alone.
export function previewNavigationFile(
  event: MessageEvent,
  frameWindow: Window | null,
  origin: string,
  viewPath: string,
  source: string,
  shareHash: string = "",
): string | null {
  if (!frameWindow || event.source !== frameWindow || event.origin !== "null") return null;
  const data = event.data;
  if (!data || data.type !== "filebrowser-html-navigation" || typeof data.href !== "string") return null;
  try {
    const url = new URL(data.href);
    if (url.origin !== origin || url.pathname !== viewPath || url.username || url.password) return null;
    if (shareHash) {
      if (url.searchParams.get("hash") !== shareHash || url.searchParams.has("source")) return null;
    } else if (url.searchParams.get("source") !== source || url.searchParams.has("hash")) {
      return null;
    }
    const files = url.searchParams.getAll("file");
    if (files.length !== 1) return null;
    const file = files[0];
    if (!file || !file.startsWith("/") || !/\.html?$/i.test(file) || /[\x00-\x1f\\]/.test(file)) return null;
    if (file.split("/").some((part) => part === "." || part === "..")) return null;
    return file;
  } catch {
    return null;
  }
}
