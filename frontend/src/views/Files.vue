<template>
  <div>
    <breadcrumbs v-if="showBreadCrumbs" />
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
  <PopupPreview v-if="popupEnabled" />
</template>

<script>
import { filesApi } from "@/api";
import Breadcrumbs from "@/components/Breadcrumbs.vue";
import Errors from "@/views/Errors.vue";
import Preview from "@/views/files/Preview.vue";
import ListingView from "@/views/files/ListingView.vue";
import Editor from "@/views/files/Editor.vue";
import OnlyOfficeEditor from "./files/OnlyOfficeEditor.vue";
import EpubViewer from "./files/EpubViewer.vue";
import DocViewer from "./files/DocViewer.vue";
import MarkdownViewer from "./files/MarkdownViewer.vue";
import { state, mutations, getters } from "@/store";
import { url } from "@/utils";
import router from "@/router";
import { baseURL } from "@/utils/constants";
import PopupPreview from "@/components/files/PopupPreview.vue";

export default {
  name: "files",
  components: {
    Breadcrumbs,
    Errors,
    Preview,
    ListingView,
    Editor,
    EpubViewer,
    DocViewer,
    OnlyOfficeEditor,
    MarkdownViewer,
    PopupPreview,
  },
  data() {
    return {
      error: null,
      width: window.innerWidth,
      lastPath: "",
      lastHash: "",
      popupSource: "",
    };
  },
  computed: {
    popupEnabled() {
      return state.user.preview.popup;
    },
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
      if (!getters.isLoggedIn()) {
        return;
      }
      const routePath = url.removeTrailingSlash(getters.routePath(`${baseURL}files`));
      const rootRoute =
        routePath == "/files" ||
        routePath == "/files/" ||
        routePath == "" ||
        routePath == "/";
      // lets redirect if multiple sources and user went to /files/
      if (state.serverHasMultipleSources && rootRoute) {
        const urlEncodedSource = encodeURIComponent(state.sources.current)
        router.push(`${routePath}/${urlEncodedSource}`);
        return;
      }
      this.lastHash = "";
      // Set loading to true and reset the error.
      mutations.setLoading("files", true);
      this.error = null;
      // Reset view information using mutations
      mutations.setReload(false);

      let data = {};
      try {
        // Fetch initial data
        let res = await filesApi.fetchFiles(getters.routePath());
        // If not a directory, fetch content
        if (res.type != "directory") {
          const content = !getters.onlyOfficeEnabled();
          res = await filesApi.fetchFiles(getters.routePath(), content);
        }
        data = res;
        if (state.sources.count > 1) {
          mutations.setCurrentSource(data.source);
        }
        document.title = `${document.title} - ${res.name}`;
        mutations.replaceRequest(data);
      } catch (e) {
        this.error = e;
        mutations.replaceRequest({});
        if (e.status === 404) {
          router.push({ name: "notFound" });
        } else if (e.status === 403) {
          router.push({ name: "forbidden" });
        } else {
          router.push({ name: "error" });
        }
      } finally {
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
