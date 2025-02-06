<template>
  <div>
    <breadcrumbs v-if="showBreadCrumbs" base="/files" />
    <errors v-if="error" :errorCode="error.status" />
    <component v-else-if="currentViewLoaded" :is="currentView"></component>
    <div v-else>
      <h2 class="message delayed">
        <div class="spinner">
          <div class="bounce1"></div>
          <div class="bounce2"></div>
          <div class="bounce3"></div>
        </div>
        <span>{{ $t("files.loading") }}</span>
      </h2>
    </div>
  </div>
</template>

<script>
import { filesApi } from "@/api";
import Breadcrumbs from "@/components/Breadcrumbs.vue";
import Errors from "@/views/Errors.vue";
import Preview from "@/views/files/Preview.vue";
import ListingView from "@/views/files/ListingView.vue";
import Editor from "@/views/files/Editor.vue";
import OnlyOfficeEditor from "./files/OnlyOfficeEditor.vue";
import MarkdownViewer from "./files/MarkdownViewer.vue";
import { state, mutations, getters } from "@/store";
import { url } from "@/utils";
import { notify } from "@/notify";
//import { removePrefix } from "@/utils/url.js";

export default {
  name: "files",
  components: {
    Breadcrumbs,
    Errors,
    Preview,
    ListingView,
    Editor,
    OnlyOfficeEditor,
    MarkdownViewer,
  },
  data() {
    return {
      error: null,
      width: window.innerWidth,
      lastPath: "",
      lastHash: "",
    };
  },
  computed: {
    showBreadCrumbs() {
      return getters.showBreadCrumbs();
    },
    currentView() {
      return getters.currentView();
    },
    currentViewLoaded() {
      return getters.currentView() !== null;
    },
    reload() {
      return state.reload;
    },
  },
  created() {
    this.fetchData();
  },
  watch: {
    $route: "fetchData",
    reload(value) {
      if (value) {
        this.fetchData();
      }
    },
  },
  mounted() {
    window.addEventListener("hashchange", this.scrollToHash);
    window.addEventListener("keydown", this.keyEvent);
  },
  beforeUnmount() {
    window.removeEventListener("keydown", this.keyEvent);
  },
  unmounted() {
    mutations.replaceRequest({}); // Use mutation
  },
  methods: {
    scrollToHash() {
      if (window.location.hash === this.lastHash) return;
      this.lastHash = window.location.hash;
      if (window.location.hash) {
        const id = url.base64Encode(window.location.hash.slice(1));
        const element = document.getElementById(id);
        if (element) {
          element.scrollIntoView({
            behavior: "instant",
            block: "center",
          });
        }
      }
    },
    async fetchData() {
      if (state.route.path === this.lastPath) return;
      this.lastHash = "";
      // Set loading to true and reset the error.
      mutations.setLoading("files", true);
      this.error = null;
      // Reset view information using mutations
      mutations.setReload(false);
      mutations.setMultiple(false);
      mutations.closeHovers();

      let data = {};
      try {
        // Fetch initial data
        let res = await filesApi.fetchFiles(getters.routePath());
        // If not a directory, fetch content
        if (res.type != "directory") {
          let content = false;
          if (
            !res.onlyOfficeId &&
            (res.type.startsWith("application") || res.type.startsWith("text"))
          ) {
            content = true;
          }
          res = await filesApi.fetchFiles(getters.routePath(), content);
        }
        data = res;
        document.title = `${document.title} - ${res.name}`;
      } catch (e) {
        notify.showError(e);
        this.error = e;
        mutations.replaceRequest({});
      } finally {
        mutations.replaceRequest(data);
        mutations.setLoading("files", false);
      }
      setTimeout(() => {
        this.scrollToHash();
      }, 25);
      this.lastPath = state.route.path;
    },
    keyEvent(event) {
      // F1!
      if (event.keyCode === 112) {
        event.preventDefault();
        mutations.showHover("help"); // Use mutation
      }
    },
  },
};
</script>
