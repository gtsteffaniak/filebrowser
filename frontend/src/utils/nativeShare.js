/* eslint-env browser */
import { resourcesApi } from "@/api";
import { notify } from "@/notify";
import { getters, state } from "@/store";

export function canNativeShare() {
  return (
    typeof navigator !== "undefined" &&
    typeof navigator.share === "function" &&
    window.isSecureContext
  );
}

function buildDownloadUrl(item) {
  if (getters.isShare()) {
    return resourcesApi.getDownloadURLPublic(state.shareInfo, [item.path], false);
  }
  const source = item.source || state.req?.source;
  return resourcesApi.getDownloadURL(source, item.path, false, false);
}

/**
 * Share a single file via the Web Share API (OS share sheet).
 */
export async function nativeShareFile(item) {
  if (!canNativeShare()) {
    notify.showError("Sharing is not supported on this device");
    return;
  }

  const filename = item.name || item.path?.split("/").pop() || "file";
  const url = buildDownloadUrl(item);

  try {
    const response = await fetch(url, { credentials: "same-origin" });
    if (!response.ok) {
      const detail = response.statusText ? ` (${response.statusText})` : "";
      throw new Error(`Download failed (${response.status}${detail})`);
    }

    const blob = await response.blob();
    const type = blob.type || item.type || "application/octet-stream";
    const file = new File([blob], filename, { type });
    const shareData = { files: [file], title: filename };

    if (typeof navigator.canShare === "function" && navigator.canShare(shareData)) {
      await navigator.share(shareData);
      return;
    }

    await navigator.share({ url, title: filename });
  } catch (e) {
    if (e?.name === "AbortError") {
      return;
    }
    notify.showError(e.message || "Failed to share file");
  }
}
