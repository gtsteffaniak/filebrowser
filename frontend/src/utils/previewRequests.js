import { state } from "@/store";

/**
 * Fetch a preview image and return an object URL for display.
 * Pass an AbortSignal so the caller can cancel (e.g. on navigation or unmount).
 * @param {string} url - Preview API URL from getPreviewURL / getPreviewURLPublic
 * @param {AbortSignal} signal
 * @returns {Promise<string>} Blob object URL — caller must revoke when done
 */
export async function fetchPreviewImage(url, signal) {
  const res = await fetch(url, {
    credentials: "same-origin",
    signal,
    headers: {
      sessionId: state.sessionId,
    },
  });

  if (res.status < 200 || res.status > 299) {
    throw new Error(`Preview request failed: ${res.status}`);
  }

  const blob = await res.blob();
  return URL.createObjectURL(blob);
}
