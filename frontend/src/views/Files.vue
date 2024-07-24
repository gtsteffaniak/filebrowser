<template>
  <div>
    <breadcrumbs base="/files" />
    <errors v-if="error" :errorCode="error.status" />
    <component v-else-if="currentView" :is="currentView"></component>
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

import Breadcrumbs from "@/components/Breadcrumbs";
import Errors from "@/views/Errors";
import Preview from "@/views/files/Preview.vue";
import ListingView from "@/views/files/ListingView.vue";
import Editor from "@/views/files/Editor.vue";

function clean(path) {
  return path.endsWith("/") ? path.slice(0, -1) : path;
}

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
      if (state.req.type === undefined) {
        return null;
      }
      if (state.req.isDir) {
        return "listingView";
      } else if (Object.prototype.hasOwnProperty.call(state.req, "content")) {
        return "editor";
      } else {
        return "preview";
      }
    },
    reload() {
      return state.reload; // Access reload from state
    },
  },
  created() {
    this.fetchData();
  },
  watch: {
    $route: "fetchData",
    reload(value) {
      if (value === true) {
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
    mutations.updateRequest({}); // Use mutation
  },
  methods: {
    async fetchData() {
      // Reset view information using mutations
      mutations.setReload(false);
      mutations.resetSelected();
      mutations.multiple(false);
      mutations.closeHovers();

      // Set loading to true and reset the error.
      mutations.setLoading(true);
      this.error = null;

      let url = this.$route.path;
      if (url === "") url = "/";
      if (url[0] !== "/") url = "/" + url;

      try {
        let res = await api.fetch(url);
        if (!res.isDir) {
          // Get content of file if possible
          res = await api.fetch(url, true);
        }

        if (clean(res.path) !== clean(`/${this.$route.params.pathMatch}`)) {
          return;
        }

        mutations.updateRequest(res); // Use mutation
        document.title = `${res.name} - ${document.title}`;
      } catch (e) {
        this.error = e;
      } finally {
        mutations.setLoading(false);
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
