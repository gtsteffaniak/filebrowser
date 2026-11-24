import { state } from "@/store/index.ts";

export interface PreviewBlob {
  blobUrl: string;
  bytes: number;
}

interface QueuedPreview {
  url: string;
  signal: AbortSignal;
  resolve: (value: PreviewBlob) => void;
  reject: (reason?: unknown) => void;
  onAbort: () => void;
}

const MAX_CONCURRENT_LOADS = 8;
const queue: QueuedPreview[] = [];
let activeLoads = 0;

/**
 * Fetch a preview image and return an object URL for display.
 * Pass an AbortSignal so the caller can cancel
 * @param url - Preview API URL from getPreviewURL / getPreviewURLPublic
 */
export async function fetchPreviewBlob(url: string, signal?: AbortSignal): Promise<PreviewBlob> {
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
  return { blobUrl: URL.createObjectURL(blob), bytes: blob.size };
}

/**
 * same as fetchPreviewBlob but only returns the object URL
 * @returns Blob object URL, the caller must revoke it when done
 */
export async function fetchPreviewImage(url: string, signal?: AbortSignal): Promise<string> {
  return (await fetchPreviewBlob(url, signal)).blobUrl;
}

/**
 * like fetchPreviewBlob, but shared across every Icon.
 * @returns the blob URL and its size in bytes, the caller must revoke the URL when done
 */
export function queuedFetchPreview(url: string, signal: AbortSignal): Promise<PreviewBlob> {
  return new Promise((resolve, reject) => {
    if (signal.aborted) {
      reject(abortError());
      return;
    }
    const job: QueuedPreview = {
      url,
      signal,
      resolve,
      reject,
      onAbort: () => {
        const i = queue.indexOf(job);
        if (i !== -1) {
          queue.splice(i, 1);
          reject(abortError());
        }
      },
    };
    signal.addEventListener("abort", job.onAbort, { once: true });
    queue.push(job);
    processQueue();
  });
}

function abortError(): DOMException {
  return new DOMException("Aborted", "AbortError");
}

function processQueue(): void {
  while (activeLoads < MAX_CONCURRENT_LOADS && queue.length > 0) {
    const job = queue.shift() as QueuedPreview;
    job.signal.removeEventListener("abort", job.onAbort);
    activeLoads++;
    fetchPreviewBlob(job.url, job.signal)
      .then(job.resolve, job.reject)
      .finally(() => {
        activeLoads--;
        processQueue();
      });
  }
}
