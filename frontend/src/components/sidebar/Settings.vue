<template>
    settings!
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
  name: "SidebarSettings",
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
