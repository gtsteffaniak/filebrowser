<template>
  <div class="card clickable" style="min-height: 4em">
    <div class="card-wrapper user-card">
      <div @click="navigateTo('/settings#profile-main')" class="inner-card">
        <i class="material-icons">person</i>
        {{ user.username }}
        <i aria-label="settings" class="material-icons">settings</i>
      </div>

      <div class="inner-card logout-button" @click="logout">
        <i v-if="canLogout" class="material-icons">exit_to_app</i>
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
  <div v-if="loginCheck && !disableUsedPercentage" class="sidebar-scroll-list">
    <div class="sources card">
      <span>Sources</span>
      <div class="inner-card">
        <!-- My Files button -->
        <button
          v-for="(info, name) in sourceInfo"
          :key="name"
          class="action source-button"
          :class="{ active: activeSource == name }"
          @click="navigateTo('/files/' + info.pathPrefix)"
          :aria-label="$t('sidebar.myFiles')"
          :title="name"
        >
          <i class="material-icons source-icon">folder</i>
          <span>{{ name }}</span>
          <div>
            <progress-bar
              :val="info.usedPercentage"
              text-position="inside"
              :text="info.usedPercentage + '%'"
              size="large"
              text-fg-color="white"
            ></progress-bar>
            <div class="usage-info">
              <span>{{ info.used }} of {{ info.total }} used</span>
            </div>
          </div>
        </button>
      </div>
    </div>
  </div>
</template>

<script>
import * as auth from "@/utils/auth";
import {
  signup,
  disableExternal,
  disableUsedPercentage,
  noAuth,
  loginPage,
} from "@/utils/constants";
import { filesApi } from "@/api";
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
  computed: {
    isSettings: () => getters.isSettings(),
    isStickySidebar: () => getters.isStickySidebar(),
    isMobile: () => getters.isMobile(),
    isFiles: () => getters.isFiles(),
    user: () => (getters.isLoggedIn() ? state.user : {}),
    isDarkMode: () => getters.isDarkMode(),
    loginCheck: () => getters.isLoggedIn() && !getters.routePath().startsWith("/share"),
    currentPrompt: () => getters.currentPrompt(),
    active: () => getters.isSidebarVisible(),
    signup: () => signup,
    version: () => version,
    commitSHA: () => commitSHA,
    disableExternal: () => disableExternal,
    disableUsedPercentage: () => disableUsedPercentage,
    canLogout: () => !noAuth && loginPage,
    route: () => state.route,
    sourceInfo: () => state.sources.info,
    activeSource: () => state.sources.current,
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
  mounted() {
    if (this.loginCheck) {
      this.updateUsage();
    }
  },
  methods: {
    async updateUsage() {
      if (!disableUsedPercentage) {
        for (const source of state.user.sources) {
          let usage = await filesApi.usage(source);
          let sourceInfo = state.sources.info[source];
          sourceInfo.used = getHumanReadableFilesize(usage.used);
          sourceInfo.total = getHumanReadableFilesize(usage.total);
          sourceInfo.usedPercentage = Math.round((usage.used / usage.total) * 100);
          mutations.updateSource(source, sourceInfo);
        }
      }
    },
    checkLogin() {
      return getters.isLoggedIn() && !getters.routePath().startsWith("/share");
    },
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
  color: var(--textPrimary);
}

.quick-toggles {
  display: flex;
  justify-content: space-evenly;
  width: 100%;
  margin-top: 0.5em !important;
  color: var(--textPrimary);
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
  background-color: var(--primaryColor) !important;
  border-radius: 10em;
}

.inner-card {
  display: flex;
  align-items: center;
  padding: 0px !important;
}

.source-button {
  margin-top: 0.5em !important;
}

.source-button.active {
  background: var(--alt-background);
}
.source-icon {
  padding: 0.1em !important;
}
</style>
