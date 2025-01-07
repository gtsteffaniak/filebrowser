import { state, mutations, getters } from "@/store"
import { filesApi } from "@/api";
import { notify } from "@/notify"
import { removePrefix } from "@/utils/url.js";
import { publicApi } from "@/api";

export default function download() {
  if (getters.currentView() === "share") {
    let urlPath = getters.routePath("share");
    // Step 1: Split the path by '/'
    let parts = urlPath.split("/");
    // Step 2: Assign hash to the second part (index 2) and join the rest for subPath
    const hash = parts[1];
    const subPath = "/" + parts.slice(2).join("/");
    let files = [];
    for (let i of state.selected) {
      const dlfile = removePrefix(state.req.items[i].url, "share/"+hash);
      files.push(dlfile);
    }
    const share = {
      path: subPath,
      hash: hash,
      token: "",
      format: files.length ? "zip" : null,
    };
    publicApi.download(share, ...files);
    return
  }

  if (getters.isSingleFileSelected()) {
    filesApi.download(null, [getters.selectedDownloadUrl()]);
    return;
  }
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
      try {
        filesApi.download(format, files);
        notify.showSuccess("download started");
      } catch (e) {
        notify.showError("error downloading", e);
      }
    },
  });
}
