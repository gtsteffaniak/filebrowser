import { state, mutations, getters } from "@/store";
import { filesApi } from "@/api";
import { notify } from "@/notify";
import { removePrefix } from "@/utils/url.js";
import { publicApi } from "@/api";

export default function download() {
  if (getters.currentView() === "share") {
    let urlPath = getters.routePath("share");
    let parts = urlPath.split("/");
    const hash = parts[1];
    const subPath = "/" + parts.slice(2).join("/");
    let files = [];
    for (let i of state.selected) {
      const dlfile = removePrefix(state.req.items[i].url, "share/" + hash);
      files.push(dlfile);
    }
    const share = {
      path: subPath,
      hash: hash,
      token: "",
      format: files.length ? "zip" : null,
    };
    // Perform download without opening a new window
    startDownload(share, files, true);
    return;
  }

  if (state.isSearchActive) {
    startDownload(null, [state.selected[0].url]);
    return;
  }

  if (getters.isSingleFileSelected()) {
    startDownload(null, [getters.selectedDownloadUrl()]);
    return;
  }

  // Multiple files download with user confirmation
  mutations.showHover({
    name: "download",
    confirm: (format) => {
      mutations.closeHovers();
      let files = [];
      if (state.selected.length > 0) {
        for (let i of state.selected) {
          files.push(state.req.items[i].url);
        }
      } else {
        files.push(state.route.path);
      }
      startDownload(format, files);
    },
  });
}

async function startDownload(config, files, isPublic) {
  try {
    if (isPublic) {
      publicApi.download(config, files);
    } else {
      filesApi.download(config, files);
    }
    notify.showSuccess("Downloading...");
  } catch (e) {
    notify.showError(`Error downloading: ${e}`);
  }
}
