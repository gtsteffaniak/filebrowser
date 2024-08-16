<template v-if="isLoggedIn">
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
    <search v-if="showSearch"></search>
    <main :class="{ 'dark-mode': isDarkMode, moveWithSidebar: moveWithSidebar }">
      <router-view></router-view>
    </main>
    <prompts :class="{ 'dark-mode': isDarkMode }"></prompts>
  </div>
  <div class="card" id="popup-notification">
    <i v-on:click="closePopUp" class="material-icons">close</i>
    <div id="popup-notification-content">no info</div>
  </div>
  <fileSelection> </fileSelection>
</template>
<script>
import editorBar from "./bars/EditorBar.vue";
import defaultBar from "./bars/Default.vue";
import listingBar from "./bars/ListingBar.vue";
import Prompts from "@/components/prompts/Prompts.vue";
import Sidebar from "@/components/sidebar/Sidebar.vue";
import Search from "@/components/Search.vue";
import fileSelection from "@/components/FileSelection.vue";

import { closePopUp } from "@/notify";
import { enableExec } from "@/utils/constants";
import { state, getters, mutations } from "@/store";

export default {
  name: "layout",
  components: {
    fileSelection,
    Search,
    defaultBar,
    editorBar,
    listingBar,
    Sidebar,
    Prompts,
  },
  data() {
    return {
      showContexts: true,
      dragCounter: 0,
      width: window.innerWidth,
      itemWeight: 0,
    };
  },
  mounted() {
    window.addEventListener("resize", this.updateIsMobile);
  },
  computed: {
    showSearch() {
      return getters.isLoggedIn() && this.currentView == "listingView";
    },
    isLoggedIn() {
      return getters.isLoggedIn();
    },
    moveWithSidebar() {
      return (
        getters.isStickySidebar()
      );
    },
    closePopUp() {
      return closePopUp;
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
    showOverlay() {
      return getters.showOverlay();
    },
    isDarkMode() {
      return getters.isDarkMode();
    },
    isExecEnabled() {
      return enableExec;
    },
    currentView() {
      return getters.currentView();
    },
  },
  watch: {
    $route() {
      if (!getters.isLoggedIn()) {
        return;
      }
      mutations.resetSelected();
      mutations.setMultiple(false);
      if (getters.currentPromptName() !== "success") {
        mutations.closeHovers();
      }
    },
  },
  methods: {
    updateIsMobile() {
      mutations.setMobile();
    },
    resetPrompts() {
      mutations.closeSidebar();
      mutations.closeHovers();
    }
  },
};
</script>

<style>
main {
  -ms-overflow-style: none; /* Internet Explorer 10+ */
  scrollbar-width: none; /* Firefox */
  transition: 0.5s ease;
}

main.moveWithSidebar {
  padding-left: 21em;
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
