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

function canShareFiles() {
  if (typeof navigator.canShare !== "function") {
    return false;
  }
  try {
    const probe = new File(["x"], "share-probe.txt", { type: "text/plain" });
    return navigator.canShare({ files: [probe] });
  } catch {
    return false;
  }
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
    if (canShareFiles()) {
      const response = await fetch(url, { credentials: "same-origin" });
      if (!response.ok) {
        const detail = response.statusText ? ` (${response.statusText})` : "";
        throw new Error(`Download failed (${response.status}${detail})`);
      }

      const blob = await response.blob();
      const type = blob.type || item.type || "application/octet-stream";
      const file = new File([blob], filename, { type });
      await navigator.share({ files: [file], title: filename });
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
