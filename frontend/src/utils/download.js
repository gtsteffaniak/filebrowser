import { state, mutations, getters } from "@/store"
import { filesApi } from "@/api";
import { notify } from "@/notify"

export default function download() {
  if (getters.isSingleFileSelected()) {
    filesApi.download(null, getters.selectedDownloadUrl());
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
        filesApi.download(format, ...files);
        notify.showSuccess("download started");
      } catch (e) {
        notify.showError("error downloading", e);
      }
    },
  });
}
