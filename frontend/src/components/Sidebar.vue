<template>
  <nav :class="{ active, 'dark-mode': isDarkMode }">
    <!-- Section for logged-in users -->
    <template v-if="isLoggedIn">
      <!-- My Files button -->
      <button
        class="action"
        @click="toRoot"
        :aria-label="$t('sidebar.myFiles')"
        :title="$t('sidebar.myFiles')"
      >
        <i class="material-icons">folder</i>
        <span>{{ $t("sidebar.myFiles") }}</span>
      </button>

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
    </template>

    <!-- Section for non-logged-in users -->
    <template v-else>
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
    </template>

    <!-- Credits and usage information section -->
    <div class="credits" v-if="isFiles && !disableUsedPercentage && usage">
      <progress-bar :val="usage.usedPercentage" size="medium"></progress-bar>
      <span style="text-align: center">{{ usage.usedPercentage }}%</span>
      <span>{{ usage.used }} of {{ usage.total }} used</span>
      <br />
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
import { files as api } from "@/api";
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
      return getters.currentPromptName() === "sidebar";
    },
    signup: () => signup,
    version: () => version,
    disableExternal: () => disableExternal,
    disableUsedPercentage: () => disableUsedPercentage,
    canLogout: () => !noAuth && loginPage,
    usage: () => state.usage,
  },
  methods: {
    async updateUsage() {
      console.log("updating usage");

      let path = getters.getRoutePath();
      let usageStats = { used: "0 B", total: "0 B", usedPercentage: 0 };
      if (this.disableUsedPercentage) {
        return usageStats;
      }
      try {
        let usage = await api.usage(path);
        usageStats = {
          used: getHumanReadableFilesize(usage.used / 1024),
          total: getHumanReadableFilesize(usage.total / 1024),
          usedPercentage: Math.round((usage.used / usage.total) * 100),
        };
      } catch (error) {
        showError("Error fetching usage:", error);
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
