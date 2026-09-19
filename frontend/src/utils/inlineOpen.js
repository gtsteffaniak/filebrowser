/**
 * Open a URL in a new tab for inline viewing (e.g. PDF via /resources/view).
 * Uses a transient <a target="_blank"> without a download attribute so the browser
 * can display the response inline instead of forcing a download.
 */
export function openUrlInNewTab(url) {
  if (!url) {
    return;
  }
  const anchor = document.createElement("a");
  anchor.href = url;
  anchor.target = "_blank";
  anchor.rel = "noopener noreferrer";
  document.body.appendChild(anchor);
  anchor.click();
  anchor.remove();
}
