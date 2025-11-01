<template>
  <div class="card headline-card">
    <div class="card-wrapper user-card">
      <div
        v-if="settingsAllowed"
        @click="navigateTo('/settings','#profile-main')"
        class="inner-card"
      >
        <button
          class="person-button action"
          @mouseenter="showTooltip($event, $t('index.settingsHover'))"
          @mouseleave="hideTooltip"
        >
          <i class="material-icons">person</i>
          {{ user.username }}
          <i aria-label="settings" class="material-icons">settings</i>
        </button>
      </div>
      <div v-else-if="user.username === 'anonymous'" @click="navigateToLogin" class="inner-card">
        <button class="person-button action">
          <i class="material-symbols-outlined">login</i> {{ $t("sidebar.login") }}
        </button>
      </div>
      <div v-else class="inner-card">
        <button class="person-button action">
          <i class="material-icons">person</i>
          {{ user.username }}
        </button>
      </div>

      <div class="inner-card" v-if="canLogout" @click="logout">
        <button
          aria-label="logout-button"
          class="logout-button action"
          @mouseenter="showTooltip($event, $t('index.logout'))"
          @mouseleave="hideTooltip"
        >
          <i class="material-icons">exit_to_app</i>
        </button>
      </div>
    </div>

    <div v-if="!disableQuickToggles" class="card-wrapper" @mouseleave="hideTooltip">
      <div class="quick-toggles" :class="{ 'extra-padding': !hasCreateOptions }">
        <div
          class="clickable"
          :class="{ active: user?.singleClick }"
          @click="toggleClick"
          @mouseenter="showTooltip($event, $t('index.toggleClick'))"
          @mouseleave="hideTooltip"
          v-if="!isInvalidShare"
        >
          <i class="material-icons">ads_click</i>
        </div>
        <div
          aria-label="Toggle Theme"
          v-if="darkModeTogglePossible"
          class="clickable"
          :class="{ active: user?.darkMode }"
          @click="toggleDarkMode"
          @mouseenter="showTooltip($event, $t('index.toggleDark'))"
          @mouseleave="hideTooltip"
        >
          <i class="material-icons">dark_mode</i>
        </div>
        <div
          class="clickable"
          :class="{ active: isStickySidebar }"
          @click="toggleSticky"
          @mouseenter="showTooltip($event, $t('index.toggleSticky'))"
          @mouseleave="hideTooltip"
          v-if="!isMobile"
        >
          <i class="material-icons">push_pin</i>
        </div>
      </div>
    </div>

    <!-- Sidebar file actions -->
    <transition
      v-if="shareInfo.shareType !== 'upload'"
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
    <div v-if="showSources" class="sidebar-scroll-list">
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
              <i class="no-select material-symbols-outlined tooltip-info-icon"
                @mouseenter="showSourceTooltip($event, info)"
                @mouseleave="hideTooltip">
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
import { globalVars, shareInfo } from "@/utils/constants";
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
    return {};
  },
  computed: {
    hasCreateOptions() {
      if (getters.isShare()) {
        return shareInfo.allowCreate
      }
      return state.user?.permissions?.create || state.user?.permissions?.share || state.user?.permissions?.admin;
    },
    shareInfo: () => shareInfo,
    disableQuickToggles: () => state.user?.disableQuickToggles,
    hasSourceInfo: () => state.sources.hasSourceInfo,
    hideSidebarFileActions() {
      return state.user?.hideSidebarFileActions || getters.isInvalidShare() || !this.hasCreateOptions;
    },
    settingsAllowed: () => !state.user?.disableSettings,
    isSettings: () => getters.isSettings(),
    isStickySidebar: () => getters.isStickySidebar(),
    isMobile: () => getters.isMobile(),
    isInvalidShare: () => getters.isInvalidShare(),
    isListingView: () => getters.currentView() == "listingView",
    user: () => (state.user || {username: 'anonymous'}),
    isDarkMode: () => getters.isDarkMode(),
    showSources: () => !getters.isShare(),
    currentPrompt: () => getters.currentPrompt(),
    active: () => getters.isSidebarVisible(),
    signup: () => globalVars.signup,
    disableExternal: () => globalVars.disableExternal,
    canLogout: () => !globalVars.noAuth && state.user?.username !== 'anonymous',
    route: () => state.route,
    sourceInfo: () => state.sources.info,
    activeSource: () => state.sources.current,
    realtimeActive: () => state.realtimeActive,
    darkModeTogglePossible: () => shareInfo.enforceDarkLightMode != "dark" && shareInfo.enforceDarkLightMode != "light",
  },
  watch: {
    route() {
      if (!getters.isLoggedIn()) {
        return;
      }
      if (!this.isStickySidebar && !shareInfo.singleFileShare) {
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
    checkLogin() {
      return getters.isLoggedIn() && !getters.routePath().startsWith("/share");
    },
    toggleClick() {
      mutations.updateCurrentUser({ singleClick: !state.user.singleClick });
    },
    toggleDarkMode() {
      mutations.toggleDarkMode();
    },
    toggleSticky() {
      // keep sidebar open if disabling sticky sidebar
      if (!state.showSidebar && state.user.stickySidebar) {
        mutations.toggleSidebar();
      }
      mutations.updateCurrentUser({ stickySidebar: !state.user.stickySidebar });
    },
    navigateTo(path,hash) {
      mutations.setPreviousHistoryItem({
        name: state.req.name,
        source: state.req.source,
        path: state.req.path,
      });
      this.$router.push({ path: path, hash: hash });
      mutations.closeHovers();
    },
    navigateToLogin() {
      this.$router.push({ path: "/login", query: { redirect: this.$route.path } });
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
          showTooltip(event, text) {
        if (text) {
          mutations.showTooltip({
            content: text,
            x: event.clientX,
            y: event.clientY,
          });
        }
      },
      hideTooltip() {
        mutations.hideTooltip();
      },
      showSourceTooltip(event, info) {
        if (info) {
          const tooltipContent = this.buildSourceTooltipContent(info);
          mutations.showTooltip({
            content: tooltipContent,
            x: event.clientX,
            y: event.clientY,
          });
        }
      },
      buildSourceTooltipContent(info) {
        const getHumanReadable = (lastIndex) => {
          if (isNaN(Number(lastIndex))) return "";
          let val = Number(lastIndex);
          if (val === 0) {
            return "now";
          }
          return fromNow(val, state.user.locale);
        };

        return `
          <table style="border-collapse: collapse; text-align: left;">
            <thead>
              <tr>
                <th colspan="2" style="text-align: center; font-weight: bold; font-size: 1.1em; padding-bottom: 0.3em; border-bottom: 1px solid #888;">${info.name || 'Source'}</th>
              </tr>
            </thead>
            <tbody>
              <tr>
                <td style="padding: 0.2em 0.5em; border-bottom: 1px solid #ccc;">${this.$t("index.status")}</td>
                <td style="padding: 0.2em 0.5em; border-bottom: 1px solid #ccc;">${info.status || 'unknown'}</td>
              </tr>
              <tr>
                <td style="padding: 0.2em 0.5em; border-bottom: 1px solid #ccc;">${this.$t("index.assessment")}</td>
                <td style="padding: 0.2em 0.5em; border-bottom: 1px solid #ccc;">${info.assessment || 'unknown'}</td>
              </tr>
              <tr>
                <td style="padding: 0.2em 0.5em; border-bottom: 1px solid #ccc;">${this.$t("general.files")}</td>
                <td style="padding: 0.2em 0.5em; border-bottom: 1px solid #ccc;">${info.files || 0}</td>
              </tr>
              <tr>
                <td style="padding: 0.2em 0.5em; border-bottom: 1px solid #ccc;">${this.$t("general.folders")}</td>
                <td style="padding: 0.2em 0.5em; border-bottom: 1px solid #ccc;">${info.folders || 0}</td>
              </tr>
              <tr>
                <td style="padding: 0.2em 0.5em; border-bottom: 1px solid #ccc;">${this.$t("index.lastScanned")}</td>
                <td style="padding: 0.2em 0.5em; border-bottom: 1px solid #ccc;">${getHumanReadable(info.lastIndex)}</td>
              </tr>
              <tr>
                <td style="padding: 0.2em 0.5em; border-bottom: 1px solid #ccc;">${this.$t("index.quickScan")}</td>
                <td style="padding: 0.2em 0.5em; border-bottom: 1px solid #ccc;">${isNaN(Number(info.quickScanDurationSeconds)) ? '' : Number(info.quickScanDurationSeconds)}</td>
              </tr>
              <tr>
                <td style="padding: 0.2em 0.5em;">${this.$t("index.fullScan")}</td>
                <td style="padding: 0.2em 0.5em;">${isNaN(Number(info.fullScanDurationSeconds)) ? '' : Number(info.fullScanDurationSeconds)}</td>
              </tr>
            </tbody>
          </table>
        `;
      },
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
  color: white;
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

.extra-padding {
  padding-bottom: 0.5em !important;
}
</style>
