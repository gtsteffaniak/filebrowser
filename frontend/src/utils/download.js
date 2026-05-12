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
  
  // Chunked single-file (large) vs chunked multi-item archive (folder / multi-select)
  const downloadChunkSizeMb = state.user?.fileLoading?.downloadChunkSizeMb || 0
  const sizeThreshold = downloadChunkSizeMb * 1024 * 1024;
  
  const willUseChunkedDownload =
    downloadChunkSizeMb > 0 &&
    items.length === 1 &&
    !items[0].isDir &&
    items[0].size &&
    items[0].size >= sizeThreshold

  const isMultiItemArchive =
    items.length > 1 || (items.length === 1 && items[0].isDir)

  const willUseChunkedArchive =
    downloadChunkSizeMb > 0 && isMultiItemArchive

  const showChunkedProgressFirst =
    willUseChunkedDownload || willUseChunkedArchive

  if (getters.isShare()) {
    // Perform download without opening a new window
    if (getters.isSingleFileSelected()) {
      if (showChunkedProgressFirst) {
        mutations.showPrompt({ name: "download" });
        startDownload(null, items, state.shareInfo.hash, {
          silentChunkedError: true,
        });
      } else {
        startDownload(null, items, state.shareInfo.hash);
      }
    } else {
      // Multiple files download with user confirmation
      mutations.showPrompt({
        name: "download",
        confirm: (format) => {
          mutations.closeTopPrompt();
          startDownload(format, items, state.shareInfo.hash, {
            silentChunkedError: willUseChunkedArchive,
          });
        },
      });
    }
    return;
  }

  if (getters.isSingleFileSelected()) {
    if (showChunkedProgressFirst) {
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
        startDownload(format, items, "", {
          silentChunkedError: willUseChunkedArchive,
        });
      },
    });
  }
}

async function startDownload(config, files, hash = "", options = {}) {
  try {
    notify.showSuccessToast("Downloading...");
    await resourcesApi.download(config, files, hash);
  } catch (e) {
    if (e?.name === "AbortError" || e?.message?.includes("aborted") || e?.message?.includes("cancelled")) {
      return;
    }
    if (options.silentChunkedError) {
      return;
    }
    notify.showError(`Error downloading: ${e.message || e}`);
  }
}
