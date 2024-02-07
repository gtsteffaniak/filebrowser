<template>
  <div>
    <div v-show="showOverlay" @click="resetPrompts" class="overlay"></div>
    <div v-if="progress" class="progress">
      <div v-bind:style="{ width: this.progress + '%' }"></div>
    </div>
    <listingBar
      :class="{ 'dark-mode-header': isDarkMode }"
      v-if="currentView === 'listingView'"
    ></listingBar>
    <editorBar
      :class="{ 'dark-mode-header': isDarkMode }"
      v-else-if="currentView === 'editor'"
    ></editorBar>
    <defaultBar :class="{ 'dark-mode-header': isDarkMode }" v-else></defaultBar>
    <sidebar></sidebar>
    <main :class="{ 'dark-mode': isDarkMode }">
      <router-view></router-view>
    </main>
    <prompts :class="{ 'dark-mode': isDarkMode }"></prompts>
    <upload-files></upload-files>
  </div>
</template>

<script>
import editorBar from "./bars/EditorBar.vue";
import defaultBar from "./bars/Default.vue";
import listingBar from "./bars/ListingBar.vue";
import Prompts from "@/components/prompts/Prompts";
import { mapState, mapGetters } from "vuex";
import Sidebar from "@/components/Sidebar.vue";
import UploadFiles from "../components/prompts/UploadFiles";
import { enableExec } from "@/utils/constants";
import { darkMode } from "@/utils/constants";

export default {
  name: "layout",
  components: {
    defaultBar,
    editorBar,
    listingBar,
    Sidebar,
    Prompts,
    UploadFiles,
  },
  data: function () {
    return {
      showContexts: true,
      dragCounter: 0,
      width: window.innerWidth,
      itemWeight: 0,
    };
  },
  computed: {
    ...mapGetters([
      "isLogged",
      "progress",
      "isListing",
      "currentPrompt",
      "currentPromptName",
    ]),
    ...mapState(["req", "user", "state"]),
    showOverlay: function () {
      return this.currentPrompt !== null && this.currentPrompt.prompt !== "more";
    },
    isDarkMode() {
      return this.user && Object.prototype.hasOwnProperty.call(this.user, "darkMode")
        ? this.user.darkMode
        : darkMode;
    },
    isExecEnabled: () => enableExec,
    currentView() {
      if (this.req.type == undefined) {
        return null;
      }

      if (this.req.isDir) {
        return "listingView";
      } else if (this.req.type === "text" || this.req.type === "textImmutable") {
        return "editor";
      } else {
        return "preview";
      }
    },
  },
  watch: {
    $route: function () {
      this.$store.commit("resetSelected");
      this.$store.commit("multiple", false);
      if (this.currentPrompt?.prompt !== "success") this.$store.commit("closeHovers");
    },
  },
  methods: {
    resetPrompts() {
      this.$store.commit("closeHovers");
    },
    getTitle() {
      let title = "Title";
      if (this.$route.path.startsWith("/settings/")) {
        title = "Settings";
      }
      return title;
    },
  },
};
</script>

<style>
main {
  -ms-overflow-style: none; /* Internet Explorer 10+ */
  scrollbar-width: none; /* Firefox */
}
main::-webkit-scrollbar {
  display: none; /* Safari and Chrome */
}
/* Use the class .dark-mode to apply styles conditionally */
.dark-mode {
  background: var(--background);
  color: var(--textPrimary);
}

/* Header */
.dark-mode-header {
  color: white;
  background: var(--surfacePrimary);
}

/* Header with backdrop-filter support */
@supports (backdrop-filter: none) {
  .dark-mode-header {
    background: transparent;
    backdrop-filter: blur(16px) invert(0.1);
  }
}
</style>
