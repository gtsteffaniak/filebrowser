import { state, mutations, getters } from "@/store";
import { filesApi } from "@/api";
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
  if (getters.isShare()) {
    // Perform download without opening a new window
    startDownload(null, items, state.share.hash);
    return;
  }

  if (getters.isSingleFileSelected()) {
    startDownload(null, items);
    return;
  } else {
      // Multiple files download with user confirmation
    mutations.showHover({
      name: "download",
      confirm: (format) => {
        mutations.closeHovers();
        startDownload(format, items);
      },
    });
  }
}

async function startDownload(config, files, hash = "") {
  try {
    filesApi.download(config, files, hash);
    notify.showSuccess("Downloading...");
  } catch (e) {
    notify.showError(`Error downloading: ${e}`);
  }
}
