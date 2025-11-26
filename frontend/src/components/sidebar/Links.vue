<template>
  <transition name="expand" @before-enter="beforeEnter" @enter="enter" @leave="leave">
    <div v-if="!isShare || hasLinks" class="sidebar-links card">
      <div class="sidebar-links-header" :class="{ 'no-edit-options': isShare }">
        <i v-if="!isShare" @click="goHome()" class="material-icons action">home</i>
        <span>{{ $t("general.links") }}</span>
        <i v-if="!isShare" @mouseenter="showTooltip($event, $t('sidebar.customizeLinks'))" @mouseleave="hideTooltip"
          @click="openSidebarLinksPrompt" class="material-icons action">edit</i>
      </div>
      <transition-group name="expand" tag="div" class="inner-card">
        <template v-for="(link, index) in sidebarLinksToDisplay" :key="`link-${index}-${link.category}`">
          <!-- Source-type links: styled exactly like original sources -->
          <a v-if="link.category === 'source'" :href="getLinkHref(link)"
            class="action button source-button sidebar-link-button" :class="{
              active: isLinkActive(link),
              disabled: !isLinkAccessible(link)
            }" @click.prevent="handleLinkClick(link)" :aria-label="link.name">
            <div class="source-container">
              <!-- Show custom icon if user has set one -->
              <i v-if="link.icon" :class="getIconClass(link.icon) + ' link-icon'">{{ link.icon }}</i>
              <!-- Otherwise show animated status indicator -->
              <svg v-else-if="isLinkAccessible(link)" class="realtime-pulse" :class="{
                active: realtimeActive,
                danger: (sourceInfo[link.sourceName] || {}).status != 'indexing' && (sourceInfo[link.sourceName] || {}).status != 'ready',
                warning: (sourceInfo[link.sourceName] || {}).status == 'indexing',
                ready: (sourceInfo[link.sourceName] || {}).status == 'ready',
              }">
                <circle class="center" cx="50%" cy="50%" r="7px"></circle>
                <circle class="pulse" cx="50%" cy="50%" r="10px"></circle>
              </svg>
              <i v-else class="material-icons warning-icon"
                @mouseenter="showTooltip($event, $t('sidebar.sourceNotAccessible'))" @mouseleave="hideTooltip">
                warning
              </i>
              <span>{{ link.name }}</span>
              <i v-if="isLinkAccessible(link)" class="no-select material-symbols-outlined tooltip-info-icon"
                @mouseenter="showSourceTooltip($event, sourceInfo[link.sourceName] || {})" @mouseleave="hideTooltip">
                info <!-- eslint-disable-line @intlify/vue-i18n/no-raw-text -->
              </i>
            </div>
            <div v-if="hasSourceInfo && isLinkAccessible(link)" class="usage-info">
              <ProgressBar 
                :key="`progress-${link.sourceName}-${sourceInfo[link.sourceName]?.used || 0}-${sourceInfo[link.sourceName]?.total || 0}`"
                :val="getProgressBarValue(sourceInfo[link.sourceName] || {})" 
                :max="(sourceInfo[link.sourceName] || {}).total || 1" 
                :status="getProgressBarStatus(sourceInfo[link.sourceName] || {})"
                unit="bytes">
              </ProgressBar>
            </div>
          </a>

          <!-- Non-source links: tool and custom links with simple icon style -->
          <a v-else :aria-label="link.name" :href="getLinkHref(link)" class="action button sidebar-link-button"
            :class="{ active: isLinkActive(link) }" @click.prevent="handleLinkClick(link)">
            <div  class="link-container">
              <i :class="getIconClass(link.icon) + ' link-icon'">{{ link.icon }}</i>
              <span>{{ link.name }}</span>
            </div>
          </a>
        </template>
      </transition-group>
    </div>
  </transition>
</template>

<script>
import { state, getters, mutations } from "@/store";
import ProgressBar from "@/components/ProgressBar.vue";
import { goToItem } from "@/utils/url";
import { getIconClass } from "@/utils/material-icons";
import { buildIndexInfoTooltipHTML } from "@/components/files/IndexInfo.vue";
import { globalVars } from "@/utils/constants";
import { publicApi } from "@/api";

export default {
  name: "SidebarLinks",
  components: {
    ProgressBar,
  },
  computed: {
    isShare: () => getters.isShare(),
    hasLinks() {
      return this.sidebarLinksToDisplay?.length > 0;
    },
    user: () => (state.user || {username: 'anonymous'}),
    sourceInfo() {
      // Access state.sources.info to create reactive dependency
      return state.sources.info;
    },
    activeSource: () => state.sources.current,
    realtimeActive: () => state.realtimeActive,
    hasSourceInfo: () => state.sources.hasSourceInfo,
    showSidebarLinks() {
      // Always show sidebar links section (replaces sources)
      return true;
    },
    hasCustomLinks() {
      // Check if user has customized their links
      return this.user?.sidebarLinks && this.user.sidebarLinks.length > 0;
    },
    sidebarLinksToDisplay() {
      // If viewing a share, use share's links
      if (getters.isShare() && state.shareInfo?.sidebarLinks && state.shareInfo.sidebarLinks.length > 0) {
        return state.shareInfo.sidebarLinks;
      }

      // If user has custom links, use those
      if (this.hasCustomLinks) {
        return this.user.sidebarLinks;
      }

      // Otherwise, return default links (sources)
      return this.getDefaultLinks();
    },
  },
  methods: {
    getIconClass,
    getLinkHref(link) {
      // Add baseURL to target for href display
      if (!link.target) return '#';
      if (link.target.startsWith('http://') || link.target.startsWith('https://')) return link.target;

      const baseURL = globalVars.baseURL || '';
      let fullPath = '';

      // Construct full path based on link category
      if (link.category === 'source') {
        // For source links, use sourceName and relative target
        if (!link.sourceName) return '#';
        const sourceInfo = this.sourceInfo[link.sourceName];
        if (!sourceInfo) return '#'; // Source not found
        const basePath = `/files/${link.sourceName}${sourceInfo.pathPrefix}`
        fullPath = basePath + link.target;
      } else {
        // For other links (tools, custom, share), use target as-is
        fullPath = link.target;
      }

      const target = fullPath.startsWith('/') ? fullPath.substring(1) : fullPath;
      return baseURL + target;
    },
    goHome() {
      this.$router.push('/');
    },
    getDefaultLinks() {
      // Generate default links from sources
      const defaultLinks = [];

      if (this.sourceInfo) {
        Object.keys(this.sourceInfo).forEach(sourceName => {
          defaultLinks.push({
            name: sourceName,
            category: 'source',
            target: '/', // Relative path to source root
            icon: '', // No icon by default - will show animated status indicator
            sourceName: sourceName,
          });
        });
      }

      return defaultLinks;
    },
    isLinkAccessible(link) {
      // Check if link is accessible
      if (link.category === 'source') {
        // Use sourceName to check if the source is accessible
        if (!link.sourceName) return false;
        for (const [name] of Object.entries(this.sourceInfo || {})) {
          if (name === link.sourceName) {
            return true;
          }
        }
        return false;
      }
      // Tools and custom links are always accessible
      return true;
    },
    isLinkActive(link) {
      // Check if the current route matches this link
      if (link.category === 'source') {
        // Use sourceName to check if we're currently in this source
        return link.sourceName && state.req.source === link.sourceName;
      }
      // For all other links (tools, custom, share), compare target with route path
      return this.$route.path === link.target;
    },
    getSourceInfoForLink(link) {
      // Method that directly accesses reactive sourceInfo
      // Vue will track this dependency when called in template
      if (link.category !== 'source' || !link.sourceName) return {};
      // Direct access to reactive computed property ensures Vue tracks changes
      return this.sourceInfo && link.sourceName ? this.sourceInfo[link.sourceName] || {} : {};
    },
    getProgressBarStatus(sourceInfo) {
      if (sourceInfo.status === 'indexing' && sourceInfo.complexity == 0) {
        return 'indexing';
      }
      return 'default';
    },
    getProgressBarValue(sourceInfo) {
      // When indexing and files/complexity are unknown, show 100% to indicate value is not yet known
      if (sourceInfo.status === 'indexing') {
        const filesUnknown = !sourceInfo.files || sourceInfo.files === 0;
        const complexityUnknown = !sourceInfo.complexity || sourceInfo.complexity === 0;
        if (filesUnknown || complexityUnknown) {
          // Return the max value to show 100% (will be overridden by max prop)
          return sourceInfo.total || 1;
        }
      }
      // Otherwise return the actual used value
      return sourceInfo.used || 0;
    },
    handleLinkClick(link) {
      // Handle special share actions
      if (link.category === 'shareInfo') {
        mutations.showHover({ name: "ShareInfo" });
        return;
      }
      if (link.category === 'download') {
        this.goToDownload();
        return;
      }

      // Don't navigate if link is not accessible
      if (!this.isLinkAccessible(link)) {
        return;
      }

      if (link.category === 'source') {
        // For source links, use sourceName and target (relative path)
        if (!link.sourceName) return;
        const path = link.target || "/";
        goToItem(link.sourceName, path);
        return;
      }

      // For all other links (tools, custom, share), navigate using target directly
      if (link.target) {
        this.$router.push(link.target);
        mutations.closeHovers();
      }
    },
    goToDownload() {
      // Check if we're in a directory with multiple items
      const hasMultipleItems = state.req.items && state.req.items.length > 1;
      if (hasMultipleItems) {
        // Show format selector for directories with multiple items
        mutations.showHover({
          name: "download",
          confirm: (format) => {
            mutations.closeHovers();
            const downloadLink = publicApi.getDownloadURL({
              path: "/",
              hash: state.share.hash,
              token: state.share.token,
              inline: false,
            }, [state.req.path]);
            window.open(downloadLink + "&format=" + format, "_blank");
          },
        });
      } else {
        // Direct download for single files or directories
        const downloadLink = publicApi.getDownloadURL({
          path: "/",
          hash: state.share.hash,
          token: state.share.token,
          inline: false,
        }, [state.req.path]);
        window.open(downloadLink, "_blank");
      }
    },
    navigateTo(path, hash) {
      mutations.setPreviousHistoryItem({
        name: state.req?.name,
        source: state.req?.source,
        path: state.req?.path,
      });
      this.$router.push({ path: path, hash: hash });
      mutations.closeHovers();
    },
    openSidebarLinksPrompt() {
      mutations.showHover({
        name: "SidebarLinks",
      });
    },
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
      return buildIndexInfoTooltipHTML(info, this.$t, state.user.locale);
    },
  },
};
</script>

<style scoped>

.no-edit-options {
  justify-content: center !important;
}

.usage-info .vue-simple-progress {
  border-style: solid;
  border-color: var(--surfaceSecondary);
  border-radius: 1em !important;
}

.sidebar-links {
  padding: 1em;
  overflow: auto;
  min-height: 5em;
}

.material-icons.action {
  width: unset !important;
  padding: 0.25em;
  border-radius: 0.5em;
}

.sidebar-links .inner-card {
  display: flex;
  justify-content: center;
  align-items: center;
  flex-direction: column;
  width: 100%;
}

.sidebar-links-header {
  display: flex;
  justify-content: space-between;
  padding: .25em;
  padding-top: 0 !important;
  align-items: center;
  gap: 1em;
}

.sidebar-links-header span {
  font-weight: 500;
  color: var(--textPrimary);
}

.sidebar-link-action {
  cursor: pointer;
  padding: 0.5em;
}

/* Non-source link styles (tools, custom) */
.sidebar-link-button {
  margin: 0;
  margin-top: 0.25em;
  padding: 0;
  border-radius: 0.5em;
}

.sidebar-link-button:first-child {
  margin-top: 0 !important;
}

.edit-icon {
  padding: 0.5em;
  height: auto;
}

.sidebar-link-button.active {
  background: var(--alt-background);
}

/* Make anchor tags behave like buttons */
a.source-button,
a.sidebar-link-button {
  text-decoration: none;
  cursor: pointer;
}

.link-container {
  display: flex;
  flex-direction: row;
  color: var(--textPrimary);
  align-content: center;
  align-items: center;
  gap: 0.5em;
}

.link-icon {
  color: var(--primaryColor);
}

/* Warning icon for inaccessible sources */
.source-button.disabled {
  opacity: 0.5;
  cursor: not-allowed;
  pointer-events: none;
}

.source-button.disabled:hover {
  background: var(--surfaceSecondary);
  box-shadow: none !important;
  transform: none !important;
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

.realtime-pulse>.pulse {
  display: none;
  fill-opacity: 0;
  transform-origin: 50% 50%;
  animation: pulse 10s infinite backwards;
}

.realtime-pulse.active>.pulse {
  display: block;
}

.realtime-pulse.ready>.pulse {
  fill: #21d721;
  stroke: #21d721;
}

.realtime-pulse.danger>.pulse {
  fill: rgb(190, 147, 147);
  stroke: rgb(235, 55, 55);
}

.realtime-pulse.warning>.pulse {
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

.disabled .source-container {
  display: block;
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

.realtime-pulse.ready>.center {
  fill: #21d721;
}

.realtime-pulse.danger>.center {
  fill: rgb(235, 55, 55);
}

.realtime-pulse.warning>.center {
  fill: rgb(255, 157, 0);
}
</style>
