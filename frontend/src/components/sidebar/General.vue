<template>
  <div class="card" style="min-height: 4em">
    <div class="card-wrapper user-card">
      <div @click="navigateTo('/settings#profile-main')" class="inner-card">
        <button class="person-button action">
          <i class="material-icons">person</i>
          {{ user.username }}
          <i aria-label="settings" class="material-icons">settings</i>
        </button>
      </div>

      <div class="inner-card" @click="logout">
        <button aria-label="logout-button" class="logout-button action">
          <i v-if="canLogout" class="material-icons">exit_to_app</i>
        </button>
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
  <div v-if="loginCheck" class="sidebar-scroll-list">
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
          <div class="source-container">
            <svg
              v-if="!realtime"
              class="realtime-pulse"
              :class="{
                danger: info.status != 'indexing' && info.status != 'ready',
                warning: info.status == 'indexing',
                ready: info.status == 'ready',
              }"
            >
              <circle cx="50%" cy="50%" r="7px"></circle>
              <circle class="pulse" cx="50%" cy="50%" r="10px"></circle>
            </svg>
            <i v-else class="material-icons source-icon">folder</i>
            <span>{{ name }}</span>
          </div>
          <div v-if="info.used != 0" class="usage-info">
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
  noAuth,
  loginPage,
  disableUsedPercentage,
} from "@/utils/constants";
import ProgressBar from "@/components/ProgressBar.vue";
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
    realtime: () => state.user.realtime,
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

.logout-button,
.person-button {
  padding: 0 !important;
}

.pulse {
  fill: #21d721;
  stroke: #21d721;
  fill-opacity: 0;
  transform-origin: 50% 50%;
  animation: pulse 10s infinite backwards;
}

/* Trigger it once every 30 seconds using a visibility trick */
@keyframes pulse {
  from {
    stroke-width: 3px;
    stroke-opacity: 1;
    transform: scale(0.3);
  }
  to {
    stroke-width: 0;
    stroke-opacity: 0;
    transform: scale(1.5);
  }
}

.source-container {
  display: flex;
  flex-direction: row;
  color: var(--textPrimary);
  align-content: center;
  align-items: center;
}

.realtime-pulse {
  width: 2em;
  height: 2em;
}

.realtime-pulse.ready > circle {
  fill: #21d721;
}

.realtime-pulse.danger > circle {
  fill: rgb(235, 55, 55);
}

.realtime-pulse.warning > circle {
  fill: rgb(255, 157, 0);
}

.realtime-pulse.danger .pulse,
.realtime-pulse.warning .pulse {
  display: none;
}
</style>
