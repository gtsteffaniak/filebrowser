<template>
  <nav
    id="sidebar"
    :class="{ active: active, 'dark-mode': isDarkMode, sticky: user.stickySidebar }"
  >
    <div class="card">
      <button v-if="user.username" @click="toAccountSettings" class="action">
        <i class="material-icons">person</i>
        <span>{{ user.username }}</span>
      </button>
    </div>

    <div class="card card-wrapper">
      <span>Quick Toggles</span>
      <div class="quick-toggles">
        <button @click=""><i class="material-icons">folder</i></button>
        <button @click="toggleDarkMode"><i class="material-icons">folder</i></button>
        <button @click="toggleSticky"><i class="material-icons">folder</i></button>
      </div>
    </div>

    <!-- Section for logged-in users -->
    <div v-if="isLoggedIn">
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
          @click="toSettings"
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
    </div>

    <!-- Section for non-logged-in users -->
    <div v-else>
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
    <div class="sources card card-wrapper">
      <span>Sources</span>
      <div class="inner-card">
        <!-- My Files button -->
        <button
          class="action"
          @click="toRoot"
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
    <div class="jobs card card-wrapper">
        <span>jobs</span>
        <div class="inner-card"><span>sample</span></div>
    </div>

    <div class="buffer"></div>
    <!-- Credits and usage information section -->
    <div class="credits" v-if="isFiles && !disableUsedPercentage && usage">
      <span v-if="disableExternal">File Browser</span>
      <span v-else>
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
  mounted() {
    this.updateUsage();
  },
  computed: {
    isFiles() {
      return getters.isFiles();
    },
    user() {
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
      if (!state.user.stickySidebar) {
        mutations.closeSidebar();
      }
    },
  },
  methods: {
    toggleDarkMode() {
      mutations.toggleDarkMode();
    },
    toggleSticky() {
      let newSettings = state.user;
      newSettings.stickySidebar = !state.user.stickySidebar;
      console.log("sticky sidebar ", newSettings.stickySidebar);
      users.update(newSettings, ["stickySidebar"]);
      console.log("toggle sticky");
    },
    async updateUsage() {
      console.log("updating usage");

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
      console.log(usageStats);
      mutations.setUsage(usageStats);
    },
    showHover(value) {
      return mutations.showHover(value);
    },
    // Navigate to the root files directory
    toRoot() {
      this.$router.push({ path: "/files/" }, () => {});
      mutations.closeHovers();
    },
    // Navigate to the settings page
    toSettings() {
      this.$router.push({ path: "/settings" }, () => {});
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
  background-color: white;
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

#sidebar .action {
  width: 100%;
  display: block;
  border-radius: 0;
  padding: 0.5em;
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
  padding: 1em;
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
}

.quick-toggles button {
  border-radius: 10em;
  cursor: pointer;
  flex: none;
}

.card-wrapper {
  display: flex;
  flex-direction: column;
  justify-content: center;
  align-items: center;
  padding-top: 0.5em;
}

.inner-card {
  background-color: var(--surfaceSecondary);
  padding: 0px !important;
}

</style>
