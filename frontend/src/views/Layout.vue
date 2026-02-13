<template>
  <div >
    <div v-show="showOverlay" @contextmenu.prevent="onOverlayRightClick" @click="resetItems" class="overlay"></div>
    <div v-if="progress" class="progress">
      <div v-bind:style="{ width: this.progress + '%' }"></div>
    </div>
    <defaultBar :class="{ 'dark-mode-header': isDarkMode }"></defaultBar>
    <sidebar v-if="!invalidShare"></sidebar>
    <Scrollbar id="main" :class="{
      'dark-mode': isDarkMode,
      moveWithSidebar: moveWithSidebar.shouldMove,
      'remove-padding-top': isOnlyOffice,
      'main-padding': showPadding,
      scrollable: scrollable,
    }" :style="moveWithSidebar.style">
      <shelf />
      <router-view />
    </Scrollbar>
    <prompts :class="{ 'dark-mode': isDarkMode }"></prompts>
  </div>
  <Notifications />
  <Toast :toasts="toasts" />
  <StatusBar :class="{ moveWithSidebar: moveWithSidebar.shouldMove }" />
  <ContextMenu v-if="showContextMenu" v-bind="contextMenuProps"></ContextMenu>
  <Tooltip />
  <NextPrevious />
  <PopupPreview v-if="popupEnabled" />
</template>

<script>
import defaultBar from "./bars/Default.vue";
import Prompts from "@/components/prompts/Prompts.vue";
import Sidebar from "@/components/sidebar/Sidebar.vue";
import ContextMenu from "@/components/ContextMenu.vue";
import Notifications from "@/components/Notifications.vue";
import Toast from "@/components/Toast.vue";
import StatusBar from "@/components/StatusBar.vue";
import Scrollbar from "@/components/files/Scrollbar.vue";
import Tooltip from "@/components/Tooltip.vue";
import NextPrevious from "@/components/files/nextPrevious.vue";
import PopupPreview from "@/components/files/PopupPreview.vue";
import Shelf from "@/components/Shelf.vue";
import { filesApi } from "@/api";
import { state, getters, mutations } from "@/store";
import { events, notify } from "@/notify";
import { generateRandomCode } from "@/utils/auth";

export default {
  name: "layout",
  components: {
    ContextMenu,
    Notifications,
    Toast,
    StatusBar,
    defaultBar,
    Sidebar,
    Prompts,
    Scrollbar,
    Tooltip,
    NextPrevious,
    PopupPreview,
    Shelf,
  },
  data() {
    return {
      showContexts: true,
      dragCounter: 0,
      width: window.innerWidth,
      itemWeight: 0,
      toasts: [],
    };
  },
  mounted() {
    window.addEventListener("resize", this.updateIsMobile);
    if (getters.eventTheme() == "halloween") {
      document.documentElement.style.setProperty("--primaryColor", "var(--icon-orange)");
    } else if (state.user.themeColor) {
      document.documentElement.style.setProperty("--primaryColor", state.user.themeColor);
    }
    if (!state.sessionId) {
      mutations.setSession(generateRandomCode(8));
    }
    // Set up toast callback
    notify.setToastUpdateCallback((toasts) => {
      this.toasts = toasts;
    });
    this.reEval()
    this.initialize();
  },
  computed: {
    isOnlyOffice() {
      return getters.currentView() === "onlyOfficeEditor";
    },
    invalidShare() {
      return getters.isShare() && getters.isInvalidShare();
    },
    scrollable() {
      return getters.isScrollable();
    },
    showPadding() {
      return getters.showBreadCrumbs() || getters.currentView() === "settings";
    },
    isLoggedIn() {
      return getters.isLoggedIn();
    },
    moveWithSidebar() {
      const shouldMove = getters.isSidebarVisible() && getters.isStickySidebar();
      return {
        shouldMove,
        style: shouldMove ? { paddingLeft: state.sidebar.width + 'em' } : {}
      };
    },
    progress() {
      return getters.progress(); // Access getter directly from the store
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
    showContextMenu() {
      // for now lets disable for tools view
      return getters.currentView() != "tools"
    },
    contextMenuProps() {
      const prompt = getters.currentPrompt();
      if (prompt && prompt.name === "ContextMenu") {
        return prompt.props || {};
      }
      return {};
    },
    popupEnabled() {
      if (!state.user || state.user?.username == "") {
        return false;
      }
      return getters.previewPerms().popup;
    },
  },
  watch: {
    $route() {
      this.reEval()
    },
  },
  methods: {
    reEval() {
      mutations.setPreviewSource("");
      if (!getters.isLoggedIn()) {
        return;
      }
      const currentView = getters.currentView()
      mutations.setMultiple(false);
      const currentPrompt = getters.currentPromptName();
      if (currentPrompt !== "success" && currentPrompt !== "generic") {
        mutations.closeHovers();
      }
      if (window.location.hash == "" && currentView == "listingView" || currentView == "share") {
        const element = document.getElementById("main");
        if (element) {
          element.scrollTop = 0;
        }
      }
    },
    async initialize() {
      if (getters.isLoggedIn()) {
        const sourceinfo = await filesApi.sources();
        mutations.updateSourceInfo(sourceinfo);
        if (state.user.permissions.realtime) {
          events.startSSE();
        }
        const maxUploads = state.user.fileLoading?.maxConcurrentUpload || 0;
        if (maxUploads > 10 || maxUploads < 1) {
          mutations.setMaxConcurrentUpload(1);
        }
        if (state.user.showFirstLogin) {
          mutations.showHover({
            name: "generic",
            props: {
              title: this.$t("prompts.firstLoadTitle"),
              body: this.$t("prompts.firstLoadBody"),
              buttons: [
                {
                  label: this.$t("general.acknowledge"),
                  action: () => {
                    mutations.updateCurrentUser({
                      showFirstLogin: false,
                    });
                    mutations.closeTopHover();
                  },
                },
              ],
            },
          });
        }
      }
    },
    updateIsMobile() {
      mutations.setMobile();
    },
    resetItems() {
      mutations.setSearch(false);
    },
  },
};
</script>

<style>
.scrollable {
  overflow: scroll !important;
  -webkit-overflow-scrolling: touch;
  /* Enable momentum scrolling in iOS */
}

.remove-padding-top {
  padding-top: 0 !important;
}

#main {
  overflow: unset;
  -ms-overflow-style: none;
  /* Internet Explorer 10+ */
  scrollbar-width: none;
  /* Firefox */
  transition: 0.2s ease;
}

#main.moveWithSidebar {
  transition: padding-left 0.2s ease;
}

#main.moveWithSidebar.resizing {
  transition: none;
}

#main::-webkit-scrollbar {
  display: none;
  /* Safari and Chrome */
}
#main>div {
  height: 100%;
}
</style>
