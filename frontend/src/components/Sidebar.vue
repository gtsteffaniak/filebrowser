<template>
  <nav
    id="sidebar"
    :class="{ active: active, 'dark-mode': isDarkMode, sticky: user?.stickySidebar }"
  >
    <div class="card">
      <div class="card-wrapper">
        <button
          v-if="user.username"
          @click="navigateTo('/settings/profile')"
          class="action"
        >
          <i class="material-icons">person</i>
          <span>{{ user.username }}</span>
        </button>
      </div>
    </div>
    <div class="card">
      <div class="card-wrapper" @mouseleave="resetHoverTextToDefault">
        <span>{{ hoverText }}</span>
        <div class="quick-toggles">
          <div
            :class="{ active: user?.singleClick }"
            @click="toggleClick"
            @mouseover="updateHoverText('Toggle single click')"
          >
            <i class="material-icons">ads_click</i>
          </div>
          <div
            :class="{ active: user?.darkMode }"
            @click="toggleDarkMode"
            @mouseover="updateHoverText('Toggle dark mode')"
          >
            <i class="material-icons">dark_mode</i>
          </div>
          <div
            :class="{ active: user?.stickySidebar }"
            @click="toggleSticky"
            @mouseover="updateHoverText('Toggle sticky sidebar')"
            v-if="!isMobile"
          >
            <i class="material-icons">push_pin</i>
          </div>
        </div>
      </div>
    </div>

    <!-- Section for logged-in users -->
    <div v-if="isLoggedIn" class="sidebar-scroll-list">
      <!-- Buttons visible if user has create permission -->
      <div v-if="user.perm?.create">
        <!-- New Folder button -->
        <button
          @click="showHover('newDir')"
          class="action"
          :aria-label="$t('sidebar.newFolder')"
          :title="$t('sidebar.newFolder')"
        >
          <i class="material-icons">create_new_folder</i>
          <span>{{ $t("sidebar.newFolder") }}</span>
        </button>
        <!-- New File button -->
        <button
          @click="showHover('newFile')"
          class="action"
          :aria-label="$t('sidebar.newFile')"
          :title="$t('sidebar.newFile')"
        >
          <i class="material-icons">note_add</i>
          <span>{{ $t("sidebar.newFile") }}</span>
        </button>
        <!-- Upload button -->
        <button id="upload-button" @click="uploadFunc" class="action">
          <i class="material-icons">file_upload</i>
          <span>Upload file</span>
        </button>
      </div>

      <!-- Settings and Logout buttons -->
      <div>
        <!-- Settings button -->
        <button
          class="action"
          @click="navigateTo('/settings/global')"
          :aria-label="$t('sidebar.settings')"
          :title="$t('sidebar.settings')"
        >
          <i class="material-icons">settings_applications</i>
          <span>{{ $t("sidebar.settings") }}</span>
        </button>
        <!-- Logout button -->
        <button
          v-if="canLogout"
          @click="logout"
          class="action"
          id="logout"
          :aria-label="$t('sidebar.logout')"
          :title="$t('sidebar.logout')"
        >
          <i class="material-icons">exit_to_app</i>
          <span>{{ $t("sidebar.logout") }}</span>
        </button>
      </div>
      <div v-if="isLoggedIn" class="sources card">
        <span>Sources</span>
        <div class="inner-card">
          <!-- My Files button -->
          <button
            class="action"
            @click="navigateTo('/files/')"
            :aria-label="$t('sidebar.myFiles')"
            :title="$t('sidebar.myFiles')"
          >
            <i class="material-icons">folder</i>
            <span>{{ $t("sidebar.myFiles") }}</span>
            <div class="usage-info">
              <progress-bar :val="usage.usedPercentage" size="medium"></progress-bar>
              <span style="text-align: center">{{ usage.usedPercentage }}%</span>
              <span>{{ usage.used }} of {{ usage.total }} used</span>
            </div>
          </button>
        </div>
      </div>
    </div>

    <!-- Section for non-logged-in users -->
    <div v-else class="sidebar-scroll-list">
      <!-- Login button -->
      <router-link
        class="action"
        to="/login"
        :aria-label="$t('sidebar.login')"
        :title="$t('sidebar.login')"
      >
        <i class="material-icons">exit_to_app</i>
        <span>{{ $t("sidebar.login") }}</span>
      </router-link>
      <!-- Signup button, if signup is enabled -->
      <router-link
        v-if="signup"
        class="action"
        to="/login"
        :aria-label="$t('sidebar.signup')"
        :title="$t('sidebar.signup')"
      >
        <i class="material-icons">person_add</i>
        <span>{{ $t("sidebar.signup") }}</span>
      </router-link>
    </div>

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
      <span>{{ version }}</span>
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
  signup,
  disableExternal,
  disableUsedPercentage,
  noAuth,
  loginPage,
} from "@/utils/constants";
import { files, users } from "@/api";
import ProgressBar from "@/components/ProgressBar.vue";
import { getHumanReadableFilesize } from "@/utils/filesizes";
import { state, getters, mutations } from "@/store"; // Import your custom store
import { showError } from "@/notify";

export default {
  name: "sidebar",
  components: {
    ProgressBar,
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
    isMobile() {
      return getters.isMobile();
    },
    isFiles() {
      return getters.isFiles();
    },
    user() {
      if (!getters.isLoggedIn()) {
        return {};
      }
      return state.user;
    },
    isDarkMode() {
      return getters.isDarkMode();
    },
    isLoggedIn() {
      return getters.isLoggedIn();
    },
    currentPrompt() {
      return getters.currentPrompt();
    },
    active() {
      return getters.isSidebarVisible();
    },
    signup: () => signup,
    version: () => version,
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
      let newSettings = state.user;
      newSettings.stickySidebar = !state.user.stickySidebar;
      users.update(newSettings, ["stickySidebar"]);
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
      this.$router.push({ path: path }, () => {});
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
  overflow: scroll;
  margin-bottom: 0px !important;
}
#sidebar {
  top: 0;
  display: flex;
  flex-direction: column;
  padding: 1em;
  padding-top: 5em;
  width: 20em;
  position: fixed;
  z-index: 4;
  left: -20em;
  height: 100%;
  box-shadow: 0 0 5px rgba(0, 0, 0, 0.1);
  transition: 0.5s ease;
  background-color: #ededed;
}

#sidebar.sticky {
  z-index: 3;
}

@supports (backdrop-filter: none) {
  nav {
    background-color: transparent;
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

#sidebar > div {
  border-top: 1px solid rgba(0, 0, 0, 0.05);
  margin-bottom: 0.5em;
}

#sidebar .card {
  overflow: unset !important;
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

.inner-card {
  border-radius: 0.5em;
  padding: 0px !important;
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
</style>
