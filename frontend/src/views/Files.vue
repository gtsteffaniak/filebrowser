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
import { mapState, mapMutations } from "vuex";

import HeaderBar from "@/components/header/HeaderBar";
import Breadcrumbs from "@/components/Breadcrumbs";
import Errors from "@/views/Errors";
import Preview from "@/views/files/Preview.vue";
import Listing from "@/views/files/Listing.vue";
import Editor from "@/views/files/Editor.vue";

function clean(path) {
  return path.endsWith("/") ? path.slice(0, -1) : path;
}

export default {
  name: "files",
  components: {
    HeaderBar,
    Breadcrumbs,
    Errors,
    Preview,
    Listing,
    Editor,
  },
  data: function () {
    return {
      error: null,
      width: window.innerWidth,
    };
  },
  computed: {
    ...mapState(["req", "reload", "loading", "show"]),
    currentView() {
      if (this.req.type == undefined) {
        return null;
      }

      if (this.req.isDir) {
        return "listing";
      } else if (
        this.req.type === "text" ||
        this.req.type === "textImmutable"
      ) {
        return "editor";
      } else {
        return "preview";
      }
    },
  },
  created() {
    this.fetchData();
  },
  watch: {
    $route: "fetchData",
    reload: function (value) {
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
    if (this.$store.state.showShell) {
      this.$store.commit("toggleShell");
    }
    this.$store.commit("updateRequest", {});
  },
  currentView(newView) {
    // Commit the new value to the store
    this.setCurrentValue(newView);
  },
  methods: {
    ...mapMutations(["setLoading","setCurrentView"]),
    async fetchData() {
      // Reset view information.
      this.$store.commit("setReload", false);
      this.$store.commit("resetSelected");
      this.$store.commit("multiple", false);
      this.$store.commit("closeHovers");

      // Set loading to true and reset the error.
      this.setLoading(true);
      this.error = null;

      let url = this.$route.path;
      if (url === "") url = "/";
      if (url[0] !== "/") url = "/" + url;

      try {
        const res = await api.fetch(url);

        if (clean(res.path) !== clean(`/${this.$route.params.pathMatch}`)) {
          return;
        }

        this.$store.commit("updateRequest", res);
        document.title = `${res.name} - ${document.title}`;
      } catch (e) {
        this.error = e;
      } finally {
        this.setLoading(false);
      }
    },
    keyEvent(event) {
      // F1!
      if (event.keyCode === 112) {
        event.preventDefault();
        this.$store.commit("showHover", "help");
      }
    },
  },
};
</script>
