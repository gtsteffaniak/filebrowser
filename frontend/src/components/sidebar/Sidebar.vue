<template>
  <nav
    id="sidebar"
    :class="{ active: active, 'dark-mode': isDarkMode, sticky: isSticky }"
  >
    <div class="card clickable" style="min-height: 4em">
      <div @click="navigateTo('/settings#profile-main')" class="card-wrapper">
        <div class="inner-card">
          {{ user.username }}
          <i class="material-icons">settings</i>
          <i v-if="canLogout"
          @click="logout" class="material-icons">exit_to_app</i>

        </div>
      </div>
    </div>
    <SidebarSettings v-if="isSettings"></SidebarSettings>
    <SidebarGeneral v-else></SidebarGeneral>

    <div class="buffer"></div>
    <div class="credits">
      <span>
        <a
          rel="noopener noreferrer"
          target="_blank"
          href="https://github.com/gtsteffaniak/filebrowser"
        >
          File Browser
        </a>
      </span>
      <span>
        <a
          :href="'https://github.com/gtsteffaniak/filebrowser/releases/'"
          :title="commitSHA"
        >
          ({{ version }})
        </a>
      </span>
      <span>
        <a @click="help">{{ $t("sidebar.help") }}</a>
      </span>
    </div>
  </nav>
</template>

<script>
import * as auth from "@/utils/auth";
import {
  version,
  commitSHA,
  signup,
  disableExternal,
  disableUsedPercentage,
  noAuth,
  loginPage,
} from "@/utils/constants";
import { files } from "@/api";
import { getHumanReadableFilesize } from "@/utils/filesizes";
import { state, getters, mutations } from "@/store"; // Import your custom store
import { showError } from "@/notify";
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
      hoverText: "Quick Toggles", // Initially empty
    };
  },
  mounted() {
    if (getters.isLoggedIn()) {
      this.updateUsage();
    }
  },
  computed: {
    isSettings: () => getters.isSettings(),
    isSticky: () => getters.isStickySidebar(),
    isMobile: () => getters.isMobile(),
    isFiles: () => getters.isFiles(),
    user: () => (getters.isLoggedIn() ? state.user : {}),
    isDarkMode: () => getters.isDarkMode(),
    isLoggedIn: () => getters.isLoggedIn(),
    currentPrompt: () => getters.currentPrompt(),
    active: () => getters.isSidebarVisible(),
    signup: () => signup,
    version: () => version,
    commitSHA: () => commitSHA,
    disableExternal: () => disableExternal,
    disableUsedPercentage: () => disableUsedPercentage,
    canLogout: () => !noAuth && loginPage,
    usage: () => state.usage,
    route: () => state.route,
  },
  watch: {
    route() {
      if (!getters.isLoggedIn()) {
        return;
      }
      if (!state.user.stickySidebar) {
        mutations.closeSidebar();
      }
    },
  },
  methods: {
    updateHoverText(text) {
      this.hoverText = text;
    },
    resetHoverTextToDefault() {
      this.hoverText = "Quick Toggles"; // Reset to default hover text
    },
    toggleClick() {
      mutations.updateUser({ singleClick: !state.user.singleClick });
    },
    toggleDarkMode() {
      mutations.toggleDarkMode();
    },
    toggleSticky() {
      mutations.updateUser({ stickySidebar: !state.user.stickySidebar });
    },
    async updateUsage() {
      if (!getters.isLoggedIn()) {
        return;
      }
      let path = getters.getRoutePath();
      let usageStats = { used: "0 B", total: "0 B", usedPercentage: 0 };
      if (this.disableUsedPercentage) {
        return usageStats;
      }
      try {
        let usage = await files.usage(path);
        usageStats = {
          used: getHumanReadableFilesize(usage.used / 1024),
          total: getHumanReadableFilesize(usage.total / 1024),
          usedPercentage: Math.round((usage.used / usage.total) * 100),
        };
      } catch (error) {
        showError("Error fetching usage", error);
      }
      mutations.setUsage(usageStats);
    },
    showHover(value) {
      return mutations.showHover(value);
    },
    navigateTo(path) {
      const hashIndex = path.indexOf("#");
      if (hashIndex !== -1) {
        // Extract the hash
        const hash = path.substring(hashIndex);
        // Remove the hash from the path
        const cleanPath = path.substring(0, hashIndex);
        this.$router.push({ path: cleanPath, hash: hash }, () => {});
      } else {
        this.$router.push({ path: path }, () => {});
      }
      mutations.closeHovers();
    },
    // Show the help overlay
    help() {
      mutations.showHover("help");
    },
    uploadFunc() {
      mutations.showHover("upload");
    },
    // Logout the user
    logout: auth.logout,
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
  box-shadow: 0 0 5px rgba(0, 0, 0, 0.1);
  transition: 0.5s ease;
  top: 4em;
  padding-bottom: 4em;
  background-color: rgb(255 255 255 / 50%) !important;
}
#sidebar.dark-mode {
  background-color: rgb(37 49 55 / 33%) !important;
}

#sidebar.sticky {
  z-index: 3;
}

@supports (backdrop-filter: none) {
  nav {
    backdrop-filter: blur(16px) invert(0.1);
  }
}

.usage-info {
  padding: 0.5em;
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

#sidebar .action {
  width: 100%;
  display: block;
  white-space: nowrap;
  height: 100%;
  overflow: hidden;
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
  color: var(--textSecondary);
  padding-left: 1em;
}

.credits > span {
  display: block;
  margin-top: 0.5em;
  margin-left: 0;
}

.credits a,
.credits a:hover {
  color: inherit;
  cursor: pointer;
}

.buffer {
  flex-grow: 1;
}

.quick-toggles {
  display: flex;
  justify-content: space-evenly;
  width: 100%;
  margin-top: 0.5em !important;
}

.quick-toggles button {
  border-radius: 10em;
  cursor: pointer;
  flex: none;
}

.card-wrapper {
  display: flex !important;
  flex-direction: column;
  justify-content: center;
  align-items: center;
  padding: 1em !important;
  min-height: 4em;
  box-shadow: 0 2px 2px #00000024, 0 1px 5px #0000001f, 0 3px 1px -2px #0003;
  /* overflow: auto; */
  border-radius: 1em;
  height: 100%;
}

.sources {
  padding: 1em;
  margin-top: 0.5em !important;
}

.quick-toggles div {
  border-radius: 10em;
  background-color: var(--surfaceSecondary);
}

.quick-toggles div i {
  font-size: 2em;
  padding: 0.25em;
  border-radius: 10em;
  cursor: pointer;
}

button.action {
  border-radius: 0.5em;
}

.quick-toggles .active {
  background-color: var(--blue) !important;
  border-radius: 10em;
}
.inner-card {
  display: flex;
  align-items: center;
  padding: 0px !important;
}
.clickable {
  cursor: pointer;
}
.clickable:hover {
  font-weight: bold;
  box-shadow: 0 2px 2px #00000024, 0 1px 5px #0000001f, 0 3px 1px -2px #0003;
}
</style>
