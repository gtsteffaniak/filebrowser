<template>
  <nav
    id="sidebar"
    :class="{ active: active, 'dark-mode': isDarkMode, 'behind-overlay': behindOverlay, 'scrollable': isSettings }"
    :style="{ width: sidebarWidth + 'em', left: active ? '0' : `-${sidebarWidth}em` }"
  >
    <div v-if="shouldShow" class="button release-banner">
      <a :href="releaseUrl">{{ $t("sidebar.updateIsAvailable") }}</a>
      <i @click="setSeenUpdate" aria-label="close-banner" class="material-icons">close</i>
    </div>
    <SidebarSettings v-if="isSettings"></SidebarSettings>
    <SidebarGeneral v-if="!isSettings"></SidebarGeneral>
    <div class="buffer"></div>
    <div v-if="!isSettings" class="credits">
      <span v-for="item in externalLinks" :key="item.title">
        <a
          v-if="item.url === 'help prompt'"
          href="#"
          @click.prevent="help"
          :title="$t('general.help')"
          >{{ $t("general.help") }}</a
        >
        <a v-else :href="item.url" target="_blank" :title="item.title">{{
          item.text
        }}</a>
      </span>
      <span v-if="name != ''">
        <h4 style="margin: 0">{{ name }}</h4>
      </span>
    </div>
    <!-- Resize Handle -->
    <div v-if="active && !isMobile" class="sidebar-resizer" @mousedown="startResize" @touchstart="startResize" >
      <div class="resizer-handle"></div>
    </div>
  </nav>
</template>

<script>
import { globalVars } from "@/utils/constants";
import { getters, mutations, state } from "@/store"; // Import your custom store
import SidebarGeneral from "./General.vue";
import SidebarSettings from "./Settings.vue";

export default {
  name: "sidebar",
  components: {
    SidebarGeneral,
    SidebarSettings,
  },
  data() {
    return {
      resizeStartX: 0,
      resizeStartWidth: 0,
      previousSidebarSize: null, // Remember the previous width when switching from desktop to mobile.
    };
  },
  mounted() {
    // Ensure the sidebar is initialized correctly
    mutations.setSeenUpdate(localStorage.getItem("seenUpdate"));
    // Add keyboard event listener for Ctrl+B to toggle sidebar
    this.handleKeydown = (event) => {
      if ((event.ctrlKey || event.metaKey) && event.key.toLowerCase() === 'b') {
        event.preventDefault();
        const sidebarVisible = getters.isSidebarVisible();
        const isSticky = getters.isStickySidebar();
        if (!sidebarVisible) {
          mutations.updateCurrentUser({ stickySidebar: true });
          mutations.toggleSidebar();
        } else if (sidebarVisible && isSticky) {
          mutations.updateCurrentUser({ stickySidebar: false });
        } else {
          mutations.toggleSidebar();
        }
      }
    };
    document.addEventListener('keydown', this.handleKeydown);
    document.addEventListener('mousemove', this.handleResize);
    document.addEventListener('touchmove', this.handleResize, { passive: true });
    document.addEventListener('mouseup', this.stopResize);
    document.addEventListener('touchend', this.stopResize);
  },
  beforeUnmount() {
    // Clean up event listener
    if (this.handleKeydown) {
      document.removeEventListener('keydown', this.handleKeydown);
    }
    document.removeEventListener('mousemove', this.handleResize);
    document.removeEventListener('touchmove', this.handleResize);
    document.removeEventListener('mouseup', this.stopResize);
    document.removeEventListener('touchend', this.stopResize);
  },
  watch: {
    isMobile(newIsMobile, oldIsMobile) {
      if (newIsMobile !== oldIsMobile) {
        this.handleMobileStateChange(newIsMobile);
      }
    },
  },
  computed: {
    externalLinks: () => globalVars.externalLinks,
    name: () => globalVars.name,
    isValidShare: () => getters.isValidShare(),
    releaseUrl: () => globalVars.updateAvailable,
    isDarkMode: () => getters.isDarkMode(),
    isLoggedIn: () => getters.isLoggedIn(),
    isSettings: () => getters.isSettings(),
    isMobile: () => getters.isMobile(),
    active: () => getters.isSidebarVisible(),
    behindOverlay: () => state.isSearchActive || (state.prompts && state.prompts.length > 0),
    sidebarWidth: () => getters.sidebarWidth(),
    shouldShow() {
      return (
        globalVars.updateAvailable != "" &&
        state.user.permissions.admin &&
        state.seenUpdate != globalVars.updateAvailable &&
        !state.user.disableUpdateNotifications
      );
    },
  },
  methods: {
    getBaseFontSize() {
      const rootFontSize = getComputedStyle(document.documentElement).fontSize;
      return parseFloat(rootFontSize);
    },
    handleMobileStateChange(isMobile) {
      if (isMobile) {
        // If we switch to mobile, save current width and reset to default
        if (!this.previousSidebarSize) {
          this.previousSidebarSize = state.sidebar.width;
        }
        mutations.setSidebarWidth(20); // 20 em
      } else if (this.previousSidebarSize) {
        // When switching to desktop, restore previous width
        mutations.setSidebarWidth(this.previousSidebarSize);
        this.previousSidebarSize = null;
      }
    },
    startResize(event) {
      event.preventDefault();
      event.stopPropagation();
      // Handle mouse and touch events (maybe someone will want to resize in a big tablet)
      const clientX = event.clientX || (event.touches && event.touches[0].clientX);
      this.resizeStartX = clientX;
      this.resizeStartWidth = this.sidebarWidth;
      mutations.setSidebarResizing(true);
      document.body.classList.add('sidebar-resizing');
    },
    handleResize(event) {
      if (!state.sidebar.isResizing) return;
      event.preventDefault();
      // Same here
      const clientX = event.clientX || (event.touches && event.touches[0].clientX);
      if (!clientX) return;
      const deltaX = clientX - this.resizeStartX;
      // Convert pixels to em
      const baseFontSize = this.getBaseFontSize();
      const deltaEm = deltaX / baseFontSize;
      const newWidth = this.resizeStartWidth + deltaEm;
      mutations.setSidebarWidth(newWidth);
    },
    stopResize() {
      if (!state.sidebar.isResizing) return;
      mutations.setSidebarResizing(false);
      document.body.classList.remove('sidebar-resizing');
    },
    // Show the help overlay
    help() {
      mutations.showHover("help");
    },
    setSeenUpdate() {
      mutations.setSeenUpdate(globalVars.updateAvailable);
    },
  },
};
</script>

<style>
.sidebar-scroll-list {
  overflow: auto;
  margin-bottom: 0px !important;
}

#sidebar {
  display: flex;
  flex-direction: column;
  padding: 1em;
  width: 20em;
  position: fixed;
  z-index: 4;
  transform: translateZ(0);
  height: 100%;
  transition: 0.4s ease;
  top: 4em;
  padding-bottom: 4em;
  background-color: rgb(37 49 55 / 5%) !important;
  will-change: left;
  backface-visibility: hidden;
}

/* sidebar with backdrop-filter support */
@supports (backdrop-filter: none) {
  #sidebar {
    backdrop-filter: blur(8px) invert(0.1);
    isolation: isolate;
  }
  #sidebar.dark-mode {
    background-color: rgb(37 49 55 / 33%) !important;
  }
  #sidebar:not(.active) {
    backdrop-filter: blur(0) invert(0);
  }
}

#sidebar.behind-overlay {
  z-index: 3;
}

#sidebar.sticky {
  z-index: 3;
}

body.rtl nav {
  transform: translateX(100%);
}

#sidebar.active {
  transform: translateZ(0);
}

#sidebar.rtl nav.active {
  left: unset;
  right: 0;
}

#sidebar .button {
  width: 100%;
  text-overflow: ellipsis;
}

body.rtl .action {
  direction: rtl;
  text-align: right;
}

#sidebar .action > * {
  vertical-align: middle;
}

/* * * * * * * * * * * * * * * *
 *            FOOTER           *
 * * * * * * * * * * * * * * * */

.credits {
  font-size: 1em;
  color: var(--textPrimary);
  padding-left: 1em;
  padding-bottom: 1em;
}

.credits > span {
  display: block;
  margin-top: 0.5em;
  margin-left: 0;
}

.credits a,
.credits a:hover {
  cursor: pointer;
}

.buffer {
  flex-grow: 1;
}

.release-banner {
  background-color: var(--primarColor);
  display: flex !important;
  height: fit-content !important;
  justify-content: space-between;
  align-items: center;
  margin-bottom: 1em;
}

#sidebar.scrollable {
  overflow: auto;
  -ms-overflow-style: none; /* IE and Edge */
  scrollbar-width: none; /* Firefox */
}

#sidebar.scrollable::-webkit-scrollbar {
  display: none; /* Chrome, Safari, and Opera */
}

.sidebar-resizer {
  position: absolute;
  top: 0;
  right: -0.3em;
  width: 0.5em;
  height: 100%;
  cursor: col-resize;
  z-index: 1000;
}

.resizer-handle {
  position: absolute;
  top: 44%;
  right: 0;
  width: 0.125em;
  height: 2.5em;
}

.sidebar-resizer:hover .resizer-handle,
body.sidebar-resizing .resizer-handle {
  background-color: var(--primaryColor);
  border-radius: 1em;
  width: 0.3em;
}

body.sidebar-resizing,
body.sidebar-resizing * {
  cursor: col-resize !important;
  pointer-events: none;
  transition: none !important;
}

</style>
