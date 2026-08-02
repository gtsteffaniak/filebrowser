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

/**
 * Checks whether the OS/browser share sheet can accept the given File.
 * navigator.canShare({ files }) validates the specific file being shared
 * (name, MIME type, size) — many platforms only allow a safe subset
 * (images, audio, video, plain text, PDF) and reject everything else.
 */
function canShareFile(file) {
  if (typeof navigator.canShare !== "function") {
    return false;
  }
  try {
    return navigator.canShare({ files: [file] });
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
    // Only attempt to share the file's actual bytes if navigator.canShare
    // exists at all. We must check the real file (fetched below), not a
    // synthetic stand-in: this previously probed with a fixed text/plain
    // file, which always reported success even for file types (e.g.
    // .xlsx, .docx, .zip) that the OS share sheet then silently rejected
    // once navigator.share() was actually called, surfacing as an opaque
    // "permission denied" error to the user (see #2659).
    if (typeof navigator.canShare === "function") {
      const response = await fetch(url, { credentials: "same-origin" });
      if (!response.ok) {
        const detail = response.statusText ? ` (${response.statusText})` : "";
        throw new Error(`Download failed (${response.status}${detail})`);
      }

      const blob = await response.blob();
      const type = blob.type || item.type || "application/octet-stream";
      const file = new File([blob], filename, { type });

      if (canShareFile(file)) {
        await navigator.share({ files: [file], title: filename });
        return;
      }
    }

    // Fall back to sharing a link when the platform can't share the file's
    // actual bytes/type (or doesn't support file sharing at all).
    await navigator.share({ url, title: filename });
  } catch (e) {
    if (e?.name === "AbortError") {
      return;
    }
    notify.showError(e.message || "Failed to share file");
  }
}
