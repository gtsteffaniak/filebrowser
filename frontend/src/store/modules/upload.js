import Vue from "vue";
import { resourcesApi } from "@/api";
import throttle from "@/utils/throttle";
import buttons from "@/utils/buttons";

const UPLOADS_LIMIT = 5;

const state = {
  id: 0,
  sizes: [],
  progress: [],
  queue: [],
  uploads: {},
};

const mutations = {
  setProgress(state, { id, loaded }) {
    Vue.set(state.progress, id, loaded);
  },
  reset: (state) => {
    state.id = 0;
    state.sizes = [];
    state.progress = [];
  },
  addJob: (state, item) => {
    state.queue.push(item);
    state.sizes[state.id] = item.file.size;
    state.id++;
  },
  moveJob(state) {
    const item = state.queue[0];
    state.queue.shift();
    Vue.set(state.uploads, item.id, item);
  },
  removeJob(state, id) {
    Vue.delete(state.uploads, id);
    delete state.uploads[id];
  },
};

const beforeUnload = (event) => {
  event.preventDefault();
  event.returnValue = "";
};

const actions = {
  upload: (context, item) => {
    let uploadsCount = Object.keys(context.state.uploads).length;

    let isQueueEmpty = context.state.queue.length == 0;
    let isUploadsEmpty = uploadsCount == 0;

    if (isQueueEmpty && isUploadsEmpty) {
      window.addEventListener("beforeunload", beforeUnload);
      buttons.loading("upload");
    }

    context.commit("addJob", item);
    context.dispatch("processUploads");
  },
  finishUpload: (context, item) => {
    context.commit("setProgress", { id: item.id, loaded: item.file.size });
    context.commit("removeJob", item.id);
    context.dispatch("processUploads");
  },
  processUploads: async (context) => {
    let uploadsCount = Object.keys(context.state.uploads).length;

    let isBellowLimit = uploadsCount < UPLOADS_LIMIT;
    let isQueueEmpty = context.state.queue.length == 0;
    let isUploadsEmpty = uploadsCount == 0;

    let isFinished = isQueueEmpty && isUploadsEmpty;
    let canProcess = isBellowLimit && !isQueueEmpty;

    if (isFinished) {
      window.removeEventListener("beforeunload", beforeUnload);
      buttons.success("upload");
      context.commit("reset");
      context.commit("setReload", true, { root: true });
    }

    if (canProcess) {
      const item = context.state.queue[0];
      context.commit("moveJob");

      if (item.file.type == "directory") {
        await resourcesApi.post(item.source, item.path, "", false, undefined, {}, true);
      } else {
        let onUpload = throttle(
          (event) =>
            context.commit("setProgress", {
              id: item.id,
              loaded: event.loaded,
            }),
          100,
          { leading: true, trailing: false }
        );

        await resourcesApi.post(item.source, item.path, item.file, item.overwrite, onUpload);
      }

      context.dispatch("finishUpload", item);
    }
  },
};

export default { state, mutations, actions, namespaced: true };
