<template>
  <div>
    <breadcrumbs base="/files" />
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
import { files as api } from "@/api";

import Breadcrumbs from "@/components/Breadcrumbs.vue";
import Errors from "@/views/Errors.vue";
import Preview from "@/views/files/Preview.vue";
import ListingView from "@/views/files/ListingView.vue";
import Editor from "@/views/files/Editor.vue";
import { state, mutations, getters } from "@/store";
import { pathsMatch } from "@/utils/url";

export default {
  name: "files",
  components: {
    Breadcrumbs,
    Errors,
    Preview,
    ListingView,
    Editor,
  },
  data() {
    return {
      error: null,
      width: window.innerWidth,
    };
  },
  computed: {
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
        console.log("reloading");
        this.fetchData();
      }
    },
  },
  mounted() {
    window.addEventListener("keydown", this.keyEvent);
  },
  beforeUnmount() {
    window.removeEventListener("keydown", this.keyEvent);
  },
  unmounted() {
    if (state.showShell) {
      mutations.toggleShell(); // Use mutation
    }
    mutations.replaceRequest({}); // Use mutation
  },
  methods: {
    async fetchData() {
      // Set loading to true and reset the error.
      mutations.setLoading(true);
      this.error = null;

      // Reset view information using mutations
      mutations.setReload(false);
      mutations.resetSelected();
      mutations.setMultiple(false);
      mutations.closeHovers();

      let url = state.route.path;
      if (url === "") url = "/";
      if (url[0] !== "/") url = "/" + url;
      let data = {};
      try {
        // Fetch initial data
        let res = await api.fetch(url);
        // If not a directory, fetch content
        if (!res.isDir) {
          res = await api.fetch(url, true);
        }
        data = res;
        // Verify if the fetched path matches the current route
        if (pathsMatch(res.path, `/${state.route.params.path}`)) {
          document.title = `${res.name} - ${document.title}`;
        }
      } catch (e) {
        this.error = e;
      } finally {
        mutations.setLoading(false);
        mutations.replaceRequest(data);
      }
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
