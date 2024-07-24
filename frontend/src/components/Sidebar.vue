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
        <button id="upload-button" @click="upload($event)" class="action">
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
    <div
      class="credits"
      v-if="isFiles && !disableUsedPercentage && usage"
    >
      <!-- Progress bar for used storage -->
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
import * as upload from "@/utils/upload";
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
import ProgressBar from "vue-simple-progress";
import { getHumanReadableFilesize } from "@/utils/filesizes";
import { darkMode } from "@/utils/constants";
import { state, getters, mutations } from "@/store"; // Import your custom store

export default {
  name: "sidebar",
  components: {
    ProgressBar,
  },
  computed: {
    isFiles() {
      return this.$route.path.includes("/files/");
    },
    user() {
      return state.user;
    },
    isDarkMode() {
      return this.user && Object.prototype.hasOwnProperty.call(this.user, "darkMode")
        ? this.user.darkMode
        : darkMode;
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
  },
  asyncComputed: {
    usage: {
      async get() {
        let path = this.$route.path.endsWith("/")
          ? this.$route.path
          : this.$route.path + "/";
        let usageStats = { used: 0, total: 0, usedPercentage: 0 };
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
          this.$showError(error);
        }
        return usageStats;
      },
      default: { used: "0 B", total: "0 B", usedPercentage: 0 },
      shouldUpdate() {
        return this.$router.currentRoute.path.includes("/files/");
      },
    },
  },
  methods: {
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
    // Handle file upload
    upload(event) {
      return this.$upload(event);
    },
    // Handle files selected for upload
    uploadInput(event) {
      mutations.closeHovers();

      let files = event.currentTarget.files;
      let folder_upload =
        files[0].webkitRelativePath !== undefined && files[0].webkitRelativePath !== "";

      if (folder_upload) {
        for (let i = 0; i < files.length; i++) {
          let file = files[i];
          files[i].fullPath = file.webkitRelativePath;
        }
      }

      let path = this.$route.path.endsWith("/")
        ? this.$route.path
        : this.$route.path + "/";
      let conflict = upload.checkConflict(files, state.req.items);

      if (conflict) {
        mutations.showHover({
          name: "replace",
          confirm: (event) => {
            event.preventDefault();
            mutations.closeHovers();
            upload.handleFiles(files, path, true);
          },
        });

        return;
      }

      upload.handleFiles(files, path);
    },
    // Logout the user
    logout: auth.logout,
  },
};
</script>
