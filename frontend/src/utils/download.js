import { state, mutations, getters } from "@/store";
import { resourcesApi } from "@/api";
import { notify } from "@/notify";

export default function downloadFiles(items) {
  if (items.length == 0) {
    notify.showError("No files selected");
    return;
  }
  if (typeof items[0] === "number") {
    // map the index to state.req.items
    items = items.map(i => state.req.items[i]);
  }
  
  // Check if chunked download will be used (single file only)
  const downloadChunkSizeMb = state.user?.fileLoading?.downloadChunkSizeMb || 0
  const sizeThreshold = downloadChunkSizeMb * 1024 * 1024;
  
  const willUseChunkedDownload = 
    downloadChunkSizeMb > 0 && 
    items.length === 1 && 
    !items[0].isDir && 
    items[0].size && 
    items[0].size >= sizeThreshold

  if (getters.isShare()) {
    // Perform download without opening a new window
    if (getters.isSingleFileSelected()) {
      // Show download prompt for chunked downloads, otherwise start directly
      if (willUseChunkedDownload) {
        mutations.showPrompt({ name: "download" });
        startDownload(null, items, state.shareInfo.hash, { silentChunkedError: true });
      } else {
        startDownload(null, items, state.shareInfo.hash);
      }
    } else {
      // Multiple files download with user confirmation
      mutations.showPrompt({
        name: "download",
        confirm: (format) => {
          mutations.closeTopPrompt();
          startDownload(format, items, state.shareInfo.hash);
        },
      });
    }
    return;
  }

  if (getters.isSingleFileSelected()) {
    // Show download prompt for chunked downloads, otherwise start directly
    if (willUseChunkedDownload) {
      mutations.showPrompt({ name: "download" });
      startDownload(null, items, "", { silentChunkedError: true });
    } else {
      startDownload(null, items);
    }
  } else {
    // Multiple files download with user confirmation
    mutations.showPrompt({
      name: "download",
      confirm: (format) => {
        mutations.closeTopPrompt();
        startDownload(format, items);
      },
    });
  }
}

async function startDownload(config, files, hash = "", opts = {}) {
  const silentChunkedError = opts.silentChunkedError === true;
  try {
    notify.showSuccessToast("Downloading...");
    await resourcesApi.download(config, files, hash);
  } catch (e) {
    if (e?.name === "AbortError" || e?.message?.includes("aborted") || e?.message?.includes("cancelled")) {
      return;
    }
    notify.showError(`Error downloading: ${e.message || e}`);
  }
}
