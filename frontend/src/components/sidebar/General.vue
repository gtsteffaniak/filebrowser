<template>
  <div class="card headline-card">
    <div class="card-wrapper user-card">
      <div
        v-if="settingsAllowed"
        class="inner-card"
      >
        <a
          href="/settings#profile-main"
          class="person-button action button"
          @click.prevent="navigateTo('/settings','#profile-main')"
          @mouseenter="showTooltip($event, $t('index.settingsHover'))"
          @mouseleave="hideTooltip"
        >
          <i class="material-icons">person</i>
          {{ user.username }}
          <i aria-label="settings" class="material-icons">settings</i>
        </a>
      </div>
      <div v-else-if="user.username === 'anonymous'" @click="navigateToLogin" class="inner-card">
        <button class="person-button action button">
          <i class="material-symbols-outlined">login</i> {{ $t("general.login") }}
        </button>
      </div>
      <div v-else class="inner-card">
        <button class="person-button action button">
          <i class="material-icons">person</i>
          {{ user.username }}
        </button>
      </div>

      <div class="inner-card" v-if="canLogout" @click="logout">
        <button
          aria-label="logout-button"
          class="logout-button action button"
          @mouseenter="showTooltip($event, $t('general.logout'))"
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

  <!-- Sidebar Links Component (replaces sources) -->
  <SidebarLinks />
</template>

<script>
import * as auth from "@/utils/auth";
import { globalVars, shareInfo } from "@/utils/constants";
import { state, getters, mutations } from "@/store";
import { fromNow } from "@/utils/moment";
import SidebarLinks from "./Links.vue";

export default {
  name: "SidebarGeneral",
  components: {
    SidebarLinks,
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
    checkLogin() {
      return getters.isLoggedIn() && !getters.routePath().startsWith("/share");
    },
    getAssessmentLabel(assessment) {
      // Translate assessment string to i18n label
      switch (assessment) {
        case "simple":
          return this.$t("index.simple");
        case "normal":
          return this.$t("index.normal");
        case "complex":
          return this.$t("index.complex");
        case "highlyComplex":
          return this.$t("index.highlyComplex");
        case "unknown":
        default:
          return this.$t("index.unknown");
      }
    },
    getStatusLabel(status) {
      // Translate status string to i18n label
      switch (status) {
        case "ready":
          return this.$t("index.ready");
        case "indexing":
          return this.$t("index.indexing");
        case "unavailable":
          return this.$t("index.unavailable");
        case "error":
          return this.$t("index.error");
        case "unknown":
        default:
          return this.$t("index.unknown");
      }
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
                <td style="padding: 0.2em 0.5em; border-bottom: 1px solid #ccc;">${this.$t("general.status")}</td>
                <td style="padding: 0.2em 0.5em; border-bottom: 1px solid #ccc;">${this.getStatusLabel(info.status)}</td>
              </tr>
              <tr>
                <td style="padding: 0.2em 0.5em; border-bottom: 1px solid #ccc;">${this.$t("index.assessment")}</td>
                <td style="padding: 0.2em 0.5em; border-bottom: 1px solid #ccc;">${this.getAssessmentLabel(info.assessment)}</td>
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

a.person-button {
  text-decoration: none;
  cursor: pointer;
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
