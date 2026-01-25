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
    };
  },
  mounted() {
    // Ensure the sidebar is initialized correctly
    mutations.setSeenUpdate(localStorage.getItem("seenUpdate"));
    // Add keyboard event listener for Ctrl+B to toggle sidebar
    this.handleKeydown = (event) => {
      if ((event.ctrlKey || event.metaKey) && event.key.toLowerCase() === 'b') {
        event.preventDefault();
        if (state.user.stickySidebar) {
          mutations.updateCurrentUser({ stickySidebar: false });
        }
        mutations.toggleSidebar();
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
    sidebarWidth: () => state.sidebar.width,
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
      const deltaEm = deltaX / 16;
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
  left: -20em;
  height: 100%;
  transition: 0.2s ease;
  top: 4em;
  padding-bottom: 4em;
  background-color: rgb(37 49 55 / 5%) !important;
}

/* sidebar with backdrop-filter support */
@supports (backdrop-filter: none) {
  #sidebar {
    backdrop-filter: blur(16px) invert(0.1);
  }
  #sidebar.dark-mode {
    background-color: rgb(37 49 55 / 33%) !important;
  }
}

#sidebar.behind-overlay {
  z-index: 3;
}

#sidebar.sticky {
  z-index: 3;
}

body.rtl nav {
  left: unset;
  right: -17em;
}

#sidebar.active {
  left: 0;
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
  right: -0.25em;
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
}

</style>
