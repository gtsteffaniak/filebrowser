<template>
  <div>
    <div
      v-show="showOverlay"
      @contextmenu.prevent="onOverlayRightClick"
      @click="resetPrompts"
      class="overlay"
    ></div>
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
    <defaultBar v-else :class="{ 'dark-mode-header': isDarkMode }"></defaultBar>
    <sidebar></sidebar>
    <main
      :class="{
        'dark-mode': isDarkMode,
        moveWithSidebar: moveWithSidebar,
        'main-padding': showPadding,
      }"
    >
      <router-view></router-view>
    </main>
    <prompts :class="{ 'dark-mode': isDarkMode }"></prompts>
  </div>
  <Notifications />
  <ContextMenu></ContextMenu>
</template>
<script>
import editorBar from "./bars/EditorBar.vue";
import defaultBar from "./bars/Default.vue";
import listingBar from "./bars/ListingBar.vue";
import Prompts from "@/components/prompts/Prompts.vue";
import Sidebar from "@/components/sidebar/Sidebar.vue";
import ContextMenu from "@/components/ContextMenu.vue";
import Notifications from "@/components/Notifications.vue";

import { state, getters, mutations } from "@/store";
import { events } from "@/notify";
import { generateRandomCode } from "@/utils/auth";

export default {
  name: "layout",
  components: {
    ContextMenu,
    Notifications,
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
    if (state.user.themeColor) {
      document.documentElement.style.setProperty("--primaryColor", state.user.themeColor);
    }
    if (!state.sessionId) {
      mutations.setSession(generateRandomCode(8));
    }
    events.startSSE();
  },
  computed: {
    showPadding() {
      return getters.showBreadCrumbs() || getters.currentView() === "settings";
    },
    isLoggedIn() {
      return getters.isLoggedIn();
    },
    moveWithSidebar() {
      return getters.isSidebarVisible() && getters.isStickySidebar();
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
    currentView() {
      return getters.currentView();
    },
  },
  watch: {
    $route() {
      if (!getters.isLoggedIn()) {
        return;
      }
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
      mutations.setSearch(false);
    },
  },
};
</script>

<style>
#layout-container {
  padding-bottom: 30% !important;
}

main {
  -ms-overflow-style: none;
  /* Internet Explorer 10+ */
  scrollbar-width: none;
  /* Firefox */
  transition: 0.5s ease;
}

main.moveWithSidebar {
  padding-left: 20em;
}

main::-webkit-scrollbar {
  display: none;
  /* Safari and Chrome */
}
</style>
