<template>
  <div class="tooltip">
    <span class="tooltiptext-first" :class="{ visible: hoverText != '' }">
      {{ hoverText }}
    </span>
  </div>

  <div v-if="hasSourceInfo" class="tooltip-sources">
    <div
      class="tooltiptext-sources"
      :style="{ top: mouseY - 500 + 'px' }"
      :class="{ visible: sourceInfoTooltip != '' }"
    >
      <table class="tooltip-table">
        <thead>
          <tr>
            <th colspan="2">{{ sourceInfoTooltip.name }}</th>
          </tr>
        </thead>
        <tbody>
          <tr>
            <td>{{ $t("index.status") }}</td>
            <td>{{ sourceInfoTooltip.status }}</td>
          </tr>
          <tr>
            <td>{{ $t("index.assessment") }}</td>
            <td>{{ sourceInfoTooltip.assessment }}</td>
          </tr>
          <tr>
            <td>{{ $t("index.files") }}</td>
            <td>{{ sourceInfoTooltip.files }}</td>
          </tr>
          <tr>
            <td>{{ $t("index.folders") }}</td>
            <td>{{ sourceInfoTooltip.folders }}</td>
          </tr>
          <tr>
            <td>{{ $t("index.lastScanned") }}</td>
            <td>{{ gethumanReadable }}</td>
          </tr>
          <tr>
            <td>{{ $t("index.quickScan") }}</td>
            <td>{{ humanReadableQuickScan }}</td>
          </tr>
          <tr>
            <td>{{ $t("index.fullScan") }}</td>
            <td>{{ humanReadableFullScan }}</td>
          </tr>
        </tbody>
      </table>
    </div>
  </div>

  <div class="card headline-card">
    <div class="card-wrapper user-card">
      <div
        v-if="settingsAllowed"
        @click="navigateTo('/settings#profile-main')"
        class="inner-card"
      >
        <button
          class="person-button action"
          @mouseover="updateHoverText($t('index.settingsHover'))"
          @mouseleave="resetHoverTextToDefault"
        >
          <i class="material-icons">person</i>
          {{ user.username }}
          <i aria-label="settings" class="material-icons">settings</i>
        </button>
      </div>
      <div v-else class="inner-card">
        <button class="person-button action">
          <i class="material-icons">person</i>
          {{ user.username }}
        </button>
      </div>

      <div class="inner-card" @click="logout">
        <button
          aria-label="logout-button"
          class="logout-button action"
          @mouseover="updateHoverText($t('index.logout'))"
          @mouseleave="resetHoverTextToDefault"
        >
          <i v-if="canLogout" class="material-icons">exit_to_app</i>
        </button>
      </div>
    </div>

    <div v-if="!disableQuickToggles" class="card-wrapper" @mouseleave="resetHoverTextToDefault">
      <div class="quick-toggles">
        <div
          :class="{ active: user?.singleClick }"
          @click="toggleClick"
          @mouseover="updateHoverText($t('index.toggleClick'))"
          @mouseleave="resetHoverTextToDefault"
        >
          <i class="material-icons">ads_click</i>
        </div>
        <div
          :class="{ active: user?.darkMode }"
          @click="toggleDarkMode"
          @mouseover="updateHoverText($t('index.toggleDark'))"
          @mouseleave="resetHoverTextToDefault"
        >
          <i class="material-icons">dark_mode</i>
        </div>
        <div
          :class="{ active: isStickySidebar }"
          @click="toggleSticky"
          @mouseover="updateHoverText($t('index.toggleSticky'))"
          @mouseleave="resetHoverTextToDefault"
          v-if="!isMobile"
        >
          <i class="material-icons">push_pin</i>
        </div>
      </div>
    </div>

    <!-- Sidebar file actions -->
    <transition
      name="expand"
      @before-enter="beforeEnter"
      @enter="enter"
      @leave="leave"
    >
      <div v-if="!hideSidebarFileActions && isListingView" class="card-wrapper">
        <button @click="openContextMenu" aria-label="File-Actions" class="action file-actions">
          <i class="material-icons">add</i>
          {{ $t("sidebar.fileActions") }}
        </button>
      </div>
    </transition>
  </div>
  <!-- Section for logged-in users -->
  <transition
    name="expand"
    @before-enter="beforeEnter"
    @enter="enter"
    @leave="leave"
  >
    <div v-if="loginCheck" class="sidebar-scroll-list">
      <div class="sources card">
        <span> {{ $t("sidebar.sources") }}</span>
        <transition-group name="expand" tag="div" class="inner-card">
          <button
            v-for="(info, name) in sourceInfo"
            :key="name"
            class="action source-button"
            :class="{ active: activeSource == name }"
            @click="navigateTo('/files/' + info.pathPrefix)"
            :aria-label="$t('sidebar.myFiles')"
          >
            <div class="source-container">
              <svg
                class="realtime-pulse"
                :class="{
                  active: realtimeActive,
                  danger: info.status != 'indexing' && info.status != 'ready',
                  warning: info.status == 'indexing',
                  ready: info.status == 'ready',
                }"
              >
                <circle class="center" cx="50%" cy="50%" r="7px"></circle>
                <circle class="pulse" cx="50%" cy="50%" r="10px"></circle>
              </svg>
              <span>{{ name }}</span>
              <i class="no-select material-symbols-outlined source-info-icon"
                @mouseenter="updateSourceTooltip($event, info)"
                @mouseleave="resetSourceTooltip">
                info <!-- eslint-disable-line @intlify/vue-i18n/no-raw-text -->
              </i>
            </div>
            <div v-if="hasSourceInfo" class="usage-info">
              <ProgressBar :val="info.used" :max="info.total" unit="bytes"></ProgressBar>
            </div>
          </button>
        </transition-group>
      </div>
    </div>
  </transition>
</template>

<script>
import * as auth from "@/utils/auth";
import { signup, disableExternal, noAuth, loginPage } from "@/utils/constants";
import ProgressBar from "@/components/ProgressBar.vue";
import { state, getters, mutations } from "@/store"; // Import your custom store
import { getHumanReadableFilesize } from "@/utils/filesizes.js";
import { fromNow } from "@/utils/moment";

export default {
  name: "SidebarGeneral",
  components: {
    ProgressBar,
  },
  data() {
    return {
      mouseY: 0,
      hoverText: "", // Initially empty
      sourceInfoTooltip: "",
    };
  },
  computed: {
    disableQuickToggles: () => state.user.disableQuickToggles,
    hasSourceInfo: () => state.sources.hasSourceInfo,
    hideSidebarFileActions: () => state.user.hideSidebarFileActions,
    settingsAllowed: () => !state.user.disableSettings,
    isSettings: () => getters.isSettings(),
    isStickySidebar: () => getters.isStickySidebar(),
    isMobile: () => getters.isMobile(),
    isListingView: () => getters.currentView() == "listingView",
    user: () => (getters.isLoggedIn() ? state.user : {}),
    isDarkMode: () => getters.isDarkMode(),
    loginCheck: () => getters.isLoggedIn() && !getters.routePath().startsWith("/share"),
    currentPrompt: () => getters.currentPrompt(),
    active: () => getters.isSidebarVisible(),
    signup: () => signup,
    version: () => version,
    commitSHA: () => commitSHA,
    disableExternal: () => disableExternal,
    canLogout: () => !noAuth && loginPage,
    route: () => state.route,
    sourceInfo: () => state.sources.info,
    activeSource: () => state.sources.current,
    realtimeActive: () => state.realtimeActive,
    humanReadableQuickScan() {
      const tooltip = this.getTooltipInfo();
      if (!tooltip || isNaN(Number(tooltip.quickScanDurationSeconds))) return "";
      return Number(tooltip.quickScanDurationSeconds);
    },
    humanReadableFullScan() {
      const tooltip = this.getTooltipInfo();
      if (!tooltip || isNaN(Number(tooltip.fullScanDurationSeconds))) return "";
      return Number(tooltip.fullScanDurationSeconds);
    },
    gethumanReadable() {
      const tooltip = this.getTooltipInfo();
      if (!tooltip || isNaN(Number(tooltip.lastIndex))) return "";
      let val = Number(tooltip.lastIndex);
      if (val === 0) {
        return "now";
      }
      return fromNow(val, state.user.locale);
    },
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
    openContextMenu() {
      mutations.resetSelected();
      mutations.showHover({
        name: "ContextMenu",
        props: {
          showCentered: true,
        },
      });
    },
    getHumanReadableFilesize(size) {
      return getHumanReadableFilesize(size);
    },
    getTooltipInfo() {
      return this.sourceInfoTooltip || null;
    },
    checkLogin() {
      return getters.isLoggedIn() && !getters.routePath().startsWith("/share");
    },
    updateSourceTooltip(event, text) {
      this.mouseY = event.clientY;
      this.sourceInfoTooltip = text;
    },
    resetSourceTooltip() {
      this.sourceInfoTooltip = ""; // Reset to default hover text
    },
    updateHoverText(text) {
      this.hoverText = text;
    },
    resetHoverTextToDefault() {
      this.hoverText = ""; // Reset to default hover text
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
      this.sourceInfoTooltip = ""; // Reset tooltip when navigating
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
    beforeEnter(el) {
      el.style.height = '0';
      el.style.opacity = '0';
    },
    enter(el, done) {
      el.style.transition = '';
      el.style.height = '0';
      el.style.opacity = '0';
      // Force reflow
      void el.offsetHeight;
      el.style.transition = 'height 0.3s, opacity 0.3s';
      el.style.height = el.scrollHeight + 'px';
      el.style.opacity = '1';
      setTimeout(() => {
        el.style.height = 'auto';
        done();
      }, 300);
    },
    leave(el, done) {
      el.style.transition = 'height 0.3s, opacity 0.3s';
      el.style.height = el.scrollHeight + 'px';
      void el.offsetHeight;
      el.style.height = '0';
      el.style.opacity = '0';
      setTimeout(done, 300);
    },
  },
};
</script>

<style>
.tooltip {
  position: absolute;
  display: inline-block;
  left: 50%;
  top: 2em;
}

.tooltiptext-first {
  visibility: hidden;
  opacity: 0;
  transition: opacity 0.2s ease;
  position: absolute;
  width: max-content;
  max-width: 20em;
  background-color: var(--alt-background);
  color: var(--textPrimary);
  text-align: center;
  border-radius: 1em;
  padding: 0.5em;
  z-index: 1000;
  bottom: 125%;
  left: 50%;
  transform: translateX(-50%);
  box-shadow: 0 0.25em 1em rgba(0, 0, 0, 0.2);
  white-space: normal;
  overflow-wrap: break-word;
}

.tooltiptext-first.visible {
  visibility: visible;
  opacity: 1;
}

.tooltiptext-sources.visible {
  visibility: visible;
  opacity: 1;
}

.tooltip-sources {
  position: absolute;
  display: inline-block;
  left: 50%;
  top: 20em;
}

.tooltiptext-sources {
  visibility: hidden;
  opacity: 0;
  transition: opacity 0.2s ease;
  position: absolute;
  width: max-content;
  height: fit-content;
  max-width: 20em;
  background-color: var(--alt-background);
  color: var(--textPrimary);
  text-align: center;
  border-radius: 1em;
  padding: 0.5em;
  z-index: 1000;
  bottom: 125%;
  left: 50%;
  transform: translateX(-50%);
  box-shadow: 0 0.25em 1em rgba(0, 0, 0, 0.2);
  white-space: normal;
  overflow-wrap: break-word;
}

.tooltiptext-sources .tooltip-table {
  width: 100%;
  border-collapse: collapse;
  text-align: left;
}

.tooltiptext-sources .tooltip-table th {
  text-align: center;
  font-weight: bold;
  font-size: 1.1em;
  padding-bottom: 0.3em;
  border-bottom: 1px solid #888;
}

.tooltiptext-sources .tooltip-table td {
  padding: 0.2em 0.5em;
  vertical-align: top;
  border-bottom: 1px solid #ccc !important; /* force apply thin gray lines */
  border-style: hidden;
}

.tooltiptext-sources .tooltip-table tr {
  border-style: hidden;
}

.tooltiptext-sources .tooltip-table tr:last-child td {
  border-bottom: none !important;
}

.tooltiptext-first,
.tooltiptext-sources {
  pointer-events: none;
}

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

.realtime-pulse > .pulse {
  display: none;
  fill-opacity: 0;
  transform-origin: 50% 50%;
  animation: pulse 10s infinite backwards;
}

.realtime-pulse.active > .pulse {
  display: block;
}

.realtime-pulse.ready > .pulse {
  fill: #21d721;
  stroke: #21d721;
}

.realtime-pulse.danger > .pulse {
  fill: rgb(190, 147, 147);
  stroke: rgb(235, 55, 55);
}

.realtime-pulse.warning > .pulse {
  fill: rgb(255, 157, 0);
  stroke: rgb(255, 157, 0);
}

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

.realtime-pulse.ready > .center {
  fill: #21d721;
}

.realtime-pulse.danger > .center {
  fill: rgb(235, 55, 55);
}

.realtime-pulse.warning > .center {
  fill: rgb(255, 157, 0);
}

.card-wrapper {
  display: flex !important;
  flex-direction: column;
  justify-content: center;
  align-items: center;
  padding: 0em !important;
  border-radius: 1em;
}

.headline-card {
  padding: 1em;
  overflow: hidden !important;
  min-height: fit-content;
}

.person-button {
  max-width: 13em;
  padding-right: 1em !important;
}

.file-actions {
  padding: 0.25em !important;
  margin-top: 0.5em !important;
  display: flex !important;
  align-items: center;
  justify-content: center;
}

.file-actions i {
  padding: 0em !important;
}

.expand-enter-active,
.expand-leave-active {
  transition: height 0.3s cubic-bezier(0.4, 0, 0.2, 1), opacity 0.3s cubic-bezier(0.4, 0, 0.2, 1);
  overflow: hidden;
}
.expand-enter,
.expand-leave-to {
  height: 0 !important;
  opacity: 0;
}

.source-info-icon {
  font-size: 1em !important;
  padding: 0.1em !important;
  padding-left: 0.5em !important;
}
</style>
