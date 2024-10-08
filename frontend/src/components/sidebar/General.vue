<template>
  <div class="card clickable" style="min-height: 4em">
    <div class="card-wrapper user-card">
      <div @click="navigateTo('/settings#profile-main')" class="inner-card">
        <i class="material-icons">person</i>
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
          :class="{ active: isStickySidebar }"
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
          <div>
            <progress-bar :val="usage.usedPercentage" size="medium"></progress-bar>
            <div class="usage-info">
              <span>{{ usage.usedPercentage }}%</span>
              <span>{{ usage.used }} of {{ usage.total }} used</span>
            </div>
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
    isStickySidebar: () => getters.isStickySidebar(),
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
      if (!this.isStickySidebar) {
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
      mutations.updateCurrentUser({ singleClick: !state.user.singleClick });
    },
    toggleDarkMode() {
      mutations.toggleDarkMode();
    },
    toggleSticky() {
      mutations.updateCurrentUser({ stickySidebar: !state.user.stickySidebar });
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
      let usage = await files.usage(path);
      usageStats = {
        used: getHumanReadableFilesize(usage.used / 1024),
        total: getHumanReadableFilesize(usage.total / 1024),
        usedPercentage: Math.round((usage.used / usage.total) * 100),
      };

      mutations.setUsage(usageStats);
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

.sources {
  display: flex;
  justify-content: center;
  align-items: center;
  flex-direction: column;
  padding: 1em;
  margin-top: 0.5em !important;
}

.sources .inner-card {
  display: flex;
  justify-content: center;
  align-items: center;
  flex-direction: column;
  width: 100%;
}

.usage-info {
  display: flex;
  flex-direction: column;
  text-align: center;
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
</style>
