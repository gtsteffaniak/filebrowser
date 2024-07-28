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
  <div class="card" id="popup-notification"></div>
</template>
<script>
import editorBar from "./bars/EditorBar.vue";
import defaultBar from "./bars/Default.vue";
import listingBar from "./bars/ListingBar.vue";
import Prompts from "@/components/prompts/Prompts.vue";
import Sidebar from "@/components/Sidebar.vue";
import UploadFiles from "../components/prompts/UploadFiles.vue";
import { enableExec } from "@/utils/constants";
import { state, getters, mutations } from "@/store";

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
    showOverlay() {
      return getters.currentPrompt() !== null && getters.currentPromptName() !== "more";
    },
    isDarkMode() {
      console.log("state", state);

      return getters.isDarkMode();
    },
    isExecEnabled() {
      return enableExec;
    },
    currentView() {
      console.log("Current view changed", getters.currentView());
      return getters.currentView();
    },
  },
  watch: {
    $route() {
      mutations.resetSelected();
      mutations.setMultiple(false);
      if (getters.currentPromptName() !== "success") {
        mutations.closeHovers();
      }
    },
  },
  methods: {
    resetPrompts() {
      mutations.closeHovers();
    },
    getTitle() {
      let title = "Title";
      if (state.route.path.startsWith("/settings/")) {
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
  background: var(--background) !important;
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
