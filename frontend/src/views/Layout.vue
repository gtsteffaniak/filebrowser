<template>
  <div>
    <div v-show="showOverlay" @click="resetPrompts" class="overlay"></div>
    <div v-if="progress" class="progress">
      <div v-bind:style="{ width: this.progress + '%' }"></div>
    </div>
    <listingBar
      :class="{ 'dark-mode-header': isDarkMode }"
      v-if="currentView == 'listingView'"
    ></listingBar>
    <editorBar
      :class="{ 'dark-mode-header': isDarkMode }"
      v-else-if="currentView == 'editor'"
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
import Sidebar from "@/components/Sidebar.vue";
import UploadFiles from "../components/prompts/UploadFiles";
import { enableExec, darkMode } from "@/utils/constants";
import { state, getters, commit } from "@/store"; // Import your custom store

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
  data() {
    return {
      showContexts: true,
      dragCounter: 0,
      width: window.innerWidth,
      itemWeight: 0,
    };
  },
  computed: {
    isLogged() {
      return getters.isLogged(); // Access getter directly from the store
    },
    progress() {
      return getters.progress(); // Access getter directly from the store
    },
    isListing() {
      return getters.isListing(); // Access getter directly from the store
    },
    currentPrompt() {
      return getters.currentPrompt(); // Access getter directly from the store
    },
    currentPromptName() {
      return getters.currentPromptName(); // Access getter directly from the store
    },
    req() {
      return state.req; // Access state directly from the store
    },
    user() {
      return state.user; // Access state directly from the store
    },
    state() {
      return state.state; // Access state directly from the store
    },
    showOverlay() {
      return this.currentPrompt !== null && this.currentPrompt.prompt !== "more";
    },
    isDarkMode() {
      return this.user && Object.prototype.hasOwnProperty.call(this.user, "darkMode")
        ? this.user.darkMode
        : darkMode;
    },
    isExecEnabled() {
      return enableExec;
    },
    currentView() {
      if (this.req.type === undefined) {
        return null;
      }
      if (this.req.isDir) {
        return "listingView";
      } else if (Object.prototype.hasOwnProperty.call(this.req, "content")) {
        return "editor";
      } else {
        return "preview";
      }
    },
  },
  watch: {
    $route() {
      commit("resetSelected"); // Commit mutations directly to the store
      commit("multiple", false);
      if (this.currentPrompt?.prompt !== "success") commit("closeHovers");
    },
  },
  methods: {
    resetPrompts() {
      commit("closeHovers");
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
