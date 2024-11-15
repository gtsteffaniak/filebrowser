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
import { filesApi } from "@/api";
import Breadcrumbs from "@/components/Breadcrumbs.vue";
import Errors from "@/views/Errors.vue";
import Preview from "@/views/files/Preview.vue";
import ListingView from "@/views/files/ListingView.vue";
import Editor from "@/views/files/Editor.vue";
import { state, mutations, getters } from "@/store";
import { pathsMatch } from "@/utils/url";
import { notify } from "@/notify";
import { removePrefix } from "@/api/utils";

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
    mutations.replaceRequest({}); // Use mutation
  },
  methods: {
    async fetchData() {
      // Set loading to true and reset the error.
      mutations.setLoading("files", true);
      this.error = null;

      // Reset view information using mutations
      mutations.setReload(false);
      mutations.resetSelected();
      mutations.setMultiple(false);
      mutations.closeHovers();

      let data = {};
      try {
        let url = removePrefix(state.route.path, "files");
        console.log("Fetching data for", url);
        // Fetch initial data
        let res = await filesApi.fetch(url);
        // If not a directory, fetch content
        if (res.type != "directory") {
          res = await filesApi.fetch(url, true);
        }
        data = res;
        // Verify if the fetched path matches the current route
        if (pathsMatch(res.path, `/${state.route.params.path}`)) {
          document.title = `${res.name} - ${document.title}`;
        }
      } catch (e) {
        notify.showError(e);
        this.error = e;
        mutations.replaceRequest(null);
      } finally {
        mutations.replaceRequest(data);
        mutations.setLoading("files", false);
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
