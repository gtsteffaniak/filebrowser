<template>
  <div class="card clickable" style="min-height: 4em">
    <div  class="card-wrapper user-card">
    <div @click="navigateTo('/settings#profile-main')" class="inner-card">
      {{ user.username }}
      <i class="material-icons">settings</i>
    </div>


      <div class="inner-card">
        <i v-if="canLogout" @click="logout" class="material-icons">exit_to_app</i>
      </div>
    </div>
  </div>
  <div class="card" style="min-height: 6em">
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
import ProgressBar from "@/components/ProgressBar.vue";
import { getHumanReadableFilesize } from "@/utils/filesizes";
import { state, getters, mutations } from "@/store"; // Import your custom store
import { showError } from "@/notify";

export default {
  name: "SidebarGeneral",
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
      console.log("Fetching usage for", path,state.user);
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
.user-card {
  flex-direction: row !important;
  justify-content: space-between !important;
}
</style>