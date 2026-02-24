<template>
  <div class="sidebar-links card">
    <!-- Header - sticks always at the top -->
    <div class="sidebar-links-header" :class="{ 'no-edit-options': isShare, 'with-top-spacing': isShare && !disableShareCard }">
      <i v-if="!isShare" @click="goHome()" class="material-icons action" :title="$t('general.home')">home</i>
      <!-- Mode button (is the title) -->
      <button @click="cycleMode" class="mode-toggle" :title="$t('sidebar.switchMode')">
        {{ mode === 'links' ? $t('general.links') : $t('general.navigation') }}
      </button>
      <!-- Edit button always visible - hidden in shares -->
      <i v-if="!isShare"
         @mouseenter="showTooltip($event, $t('sidebar.customizeLinks'))"
         @mouseleave="hideTooltip"
         @click="openSidebarLinksPrompt"
         class="material-icons action">edit</i>
    </div>
    <!-- Scrollable Content Area -->
    <div class="sidebar-links-content">
      <!-- Links Mode -->
      <template v-if="mode === 'links'">
        <!-- Share Info section - shown when viewing a share -->
        <div v-if="isShare && !disableShareCard" class="share-info-section">
          <ShareInfo :hash="hash" :token="token" :sub-path="subPath" />
        </div>
        <transition-group name="expand" tag="div" class="inner-card">
          <template v-for="(link, index) in sidebarLinksToDisplay" :key="`link-${index}-${link.category}`">
            <!-- Source-type links (source, source-minimal, source-alt, source-hybrid, source-hybrid-2); usage bar hidden for source-minimal -->
            <a v-if="link.category === 'source' || link.category === 'source-minimal' || link.category === 'source-alt' || link.category === 'source-hybrid' || link.category === 'source-hybrid-2'" :href="getLinkHref(link)"
              class="action button source-button sidebar-link-button" :class="{
                active: isLinkActive(link),
                disabled: !isLinkAccessible(link)
              }" @click.prevent="handleLinkClick(link)" :aria-label="link.name">
              <div class="source-container" :class="{ 'has-usage-info': hasUsageInfo(link) }">
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
                <i v-if="hasUsageInfo(link)" class="no-select material-symbols-outlined tooltip-info-icon"
                  @mouseenter="showSourceTooltip($event, sourceInfo[link.sourceName] || {})" @mouseleave="hideTooltip">
                  info
                </i>
              </div>
              <div v-if="hasUsageInfo(link) && link.category !== 'source-minimal'" class="usage-info">
                <!-- For source-hybrid, show single bar with background value for disk usage -->
                <ProgressBar 
                  v-if="link.category === 'source-hybrid' || link.category === 'source-hybrid-2'"
                  :key="`progress-hybrid-${link.sourceName}-${sourceInfo[link.sourceName]?.used || 0}-${sourceInfo[link.sourceName]?.usedAlt || 0}-${sourceInfo[link.sourceName]?.total || 0}`"
                  :val="(sourceInfo[link.sourceName] || {}).used || 0"
                  :val-background="(sourceInfo[link.sourceName] || {}).usedAlt || 0"
                  :val-text="link.category === 'source-hybrid-2' ? ((sourceInfo[link.sourceName] || {}).usedAlt || 0) : null"
                  :max="(sourceInfo[link.sourceName] || {}).total || 1" 
                  :status="getProgressBarStatus(sourceInfo[link.sourceName] || {})"
                  unit="bytes">
                </ProgressBar>
                <!-- For other source types, show single bar -->
                <ProgressBar 
                  v-else
                  :key="`progress-${link.sourceName}-${sourceInfo[link.sourceName]?.used || 0}-${sourceInfo[link.sourceName]?.usedAlt || 0}-${sourceInfo[link.sourceName]?.total || 0}`"
                  :val="getProgressBarValue(link, sourceInfo[link.sourceName] || {})" 
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
          <!-- Edit Share link - only shown when viewing a share and user has share permissions -->
          <a v-if="isShare && canEditShare" :aria-label="$t('general.edit', { suffix: ' ' + $t('general.share') })" href="#" 
            class="action button sidebar-link-button"
            @click.prevent="showEditShareHover">
            <div class="link-container">
              <i class="material-icons link-icon">edit</i>
              <span>{{ $t("general.edit", { suffix: " " + $t("general.share") }) }}</span>
            </div>
          </a>
        </transition-group>
      </template>
      <!-- Navigation Mode -->
      <template v-else>
        <!-- Source card (hidden in shares) -->
        <div v-if="!isShare" class="source-card-container" ref="sourceCardContainer">
          <!-- Current Source Card -->
          <div class="action button source-button navigation-source-card"
               :class="{ 'has-usage': hasUsageInfo(activeSourceLink) }"
               @click="navigateToSource(activeSource)" role="button" tabindex="0">
            <div class="source-container" :class="{ 'has-usage-info': hasUsageInfo(activeSourceLink) }">
              <i v-if="activeSourceLink.icon && isLinkAccessible(activeSourceLink)"
                 :class="getIconClass(activeSourceLink.icon) + ' link-icon'">
                {{ activeSourceLink.icon }}
              </i>
              <svg v-else-if="isLinkAccessible(activeSourceLink)"
                   class="realtime-pulse"
                   :class="{
                     active: realtimeActive,
                     danger: activeSourceInfo.status != 'indexing' && activeSourceInfo.status != 'ready',
                     warning: activeSourceInfo.status == 'indexing',
                     ready: activeSourceInfo.status == 'ready',
                   }">
                <circle class="center" cx="50%" cy="50%" r="7px"></circle>
                <circle class="pulse" cx="50%" cy="50%" r="10px"></circle>
              </svg>
              <i v-else class="material-icons warning-icon"
                @mouseenter="showTooltip($event, $t('sidebar.sourceNotAccessible'))" @mouseleave="hideTooltip">
                warning
              </i>
              <!-- Source name -->
              <span>{{ activeSource }}</span>
              <i v-if="hasUsageInfo(activeSourceLink)"
                 class="no-select material-symbols-outlined tooltip-info-icon"
                 @mouseenter="showSourceTooltip($event, activeSourceInfo)"
                 @mouseleave="hideTooltip"
                 @click.stop>
                info
              </i>
              <!-- Dropdown arrow -->
              <button class="source-dropdown-button" @click.stop="toggleSourceDropdown" ref="dropdownTrigger">
                <i class="material-icons">keyboard_arrow_down</i>
              </button>
            </div>
            <div v-if="hasUsageInfo(activeSourceLink)" class="usage-info">
              <ProgressBar 
                v-if="activeSourceLink.category === 'source-hybrid' || activeSourceLink.category === 'source-hybrid-2'"
                :val="(activeSourceInfo).used || 0"
                :val-background="(activeSourceInfo).usedAlt || 0"
                :val-text="activeSourceLink.category === 'source-hybrid-2' ? ((activeSourceInfo).usedAlt || 0) : null"
                :max="(activeSourceInfo).total || 1" 
                :status="getProgressBarStatus(activeSourceInfo)"
                unit="bytes">
              </ProgressBar>
              <ProgressBar 
                v-else
                :val="getProgressBarValue(activeSourceLink, activeSourceInfo)" 
                :max="(activeSourceInfo).total || 1" 
                :status="getProgressBarStatus(activeSourceInfo)"
                unit="bytes">
              </ProgressBar>
            </div>
          </div>
          <!-- Source Dropdown -->
          <transition name="dropdown">
            <div v-if="showSourceDropdown" class="source-dropdown" ref="dropdown">
              <div v-for="sourceName in sourceNames" :key="sourceName" class="dropdown-item"
                   @click="selectSource(sourceName)">
                {{ sourceName }}
              </div>
            </div>
          </transition>
        </div>
        <FileTree
          :currentPath="currentPath"
          :currentSource="currentSource"
          class="file-tree-container"
        />
      </template>
    </div>
  </div>
</template>

<script>
import { state, getters, mutations } from "@/store";
import ProgressBar from "@/components/ProgressBar.vue";
import { goToItem } from "@/utils/url";
import { getIconClass } from "@/utils/material-icons";
import { buildIndexInfoTooltipHTML } from "@/components/files/IndexInfo.vue";
import { globalVars } from "@/utils/constants";
import { publicApi } from "@/api";
import ShareInfo from "@/components/files/ShareInfo.vue";
import FileTree from '@/components/files/FileTree.vue';

export default {
  name: "SidebarLinks",
  components: {
    ProgressBar,
    ShareInfo,
    FileTree,
  },
  data() {
    return {
      showSourceDropdown: false,
    };
  },
  computed: {
    currentSource() {
      return state.req?.source || null;
    },
    currentPath() {
      return state.req?.path || '/';
    },
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
    canEditShare() {
      // Check if user is logged in and has share permissions
      return state.user && state.user.permissions && state.user.permissions.share;
    },
    // Share info card props
    disableShareCard() {
      return state.shareInfo?.disableShareCard;
    },
    hash() {
      return state.shareInfo?.hash || this.$route.params.hash || "";
    },
    token() {
      return state.shareInfo?.token || this.$route.query.token || "";
    },
    subPath() {
      return state.shareInfo?.path || "/";
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
    // List all source names for the dropdown menu
    sourceNames() {
      return Object.keys(state.sources.info || {});
    },
    activeSourceInfo() {
      return this.sourceInfo[this.activeSource] || {};
    },
    mode() {
      return getters.sidebarMode();
    },
    // Build a map from source name to its custom link (if any)
    sourceLinkMap() {
      const map = {};
      if (this.user?.sidebarLinks) {
        this.user.sidebarLinks.forEach(link => {
          if (this.isSourceCategory(link.category) && link.sourceName && !map[link.sourceName]) {
            map[link.sourceName] = link;
          }
        });
      }
      return map;
    },
    activeSourceLink() {
      // If user has a custom link for this source, use it
      const customLink = this.sourceLinkMap[this.activeSource];
      if (customLink) {
        return customLink;
      }
      return {
        name: this.activeSource,
        category: 'source',
        target: '/',
        icon: '',
        sourceName: this.activeSource,
      };
    },
  },
  mounted() {
    document.addEventListener('click', this.closeDropdown);
  },
  beforeUnmount() {
    document.removeEventListener('click', this.closeDropdown);
  },
  methods: {
    isSourceCategory(category) {
      return category === 'source' || category === 'source-minimal' || category === 'source-alt' ||
             category === 'source-hybrid' || category === 'source-hybrid-2';
    },
    getIconClass,
    hasUsageInfo(link) {
      // Check if usage info should be displayed for this link (source only; source-minimal hides usage)
      // Returns true when link is accessible and has usage > 0
      if (!this.isSourceCategory(link.category) || !link.sourceName) return false;
      if (!this.hasSourceInfo || !this.isLinkAccessible(link)) return false;
      if (link.category === 'source-minimal') return false;
      const info = this.sourceInfo[link.sourceName] || {};
      return (info.used || 0) > 0 || (info.usedAlt || 0) > 0;
    },
    getLinkHref(link) {
      // Add baseURL to target for href display
      if (!link.target) return '#';
      const lowerTarget = link.target.toLowerCase();
      if (lowerTarget.startsWith('http://') || lowerTarget.startsWith('https://')) return link.target;

      const baseURL = globalVars.baseURL || '';
      let fullPath = '';

      // Construct full path based on link category
      if (this.isSourceCategory(link.category)) {
        // For source links, use sourceName and relative target
        if (!link.sourceName) return '#';
        const sourceInfo = this.sourceInfo[link.sourceName];
        if (!sourceInfo) return '#'; // Source not found
        const encodedSourceName = encodeURIComponent(link.sourceName);
        const targetPath = link.target.startsWith('/') ? link.target.substring(1) : link.target;
        fullPath = `/files/${encodedSourceName}/${targetPath}`;
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
      if (this.isSourceCategory(link.category)) {
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
      if (this.isSourceCategory(link.category)) {
        // Use sourceName to check if we're currently in this source
        return link.sourceName && state.req.source === link.sourceName;
      }
      // For all other links (tools, custom, share), compare target with route path
      return this.$route.path === link.target;
    },
    getSourceInfoForLink(link) {
      // Method that directly accesses reactive sourceInfo
      // Vue will track this dependency when called in template
      if (!this.isSourceCategory(link.category) || !link.sourceName) return {};
      // Direct access to reactive computed property ensures Vue tracks changes
      return this.sourceInfo && link.sourceName ? this.sourceInfo[link.sourceName] || {} : {};
    },
    getProgressBarStatus(sourceInfo) {
      if (sourceInfo.status === 'indexing' && sourceInfo.complexity == 0) {
        return 'indexing';
      }
      return 'default';
    },
    getProgressBarValue(link, sourceInfo) {
      // Called with (link, sourceInfo) from both modes
      if (link.category === 'source-alt') {
        return sourceInfo.usedAlt || 0;
      }
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

      if (this.isSourceCategory(link.category)) {
        // For source links, use sourceName and target (relative path)
        if (!link.sourceName) return;
        const path = link.target || "/";
        goToItem(link.sourceName, path, {});
        return;
      }

      const lowerTarget = link.target.toLowerCase();
      if (lowerTarget.startsWith('http://') || lowerTarget.startsWith('https://')) {
        window.open(link.target, "_blank");
        return;
      }
      // For all other links (tools, custom, share), navigate using target directly
      if (link.target) {
        this.$router.push(link.target);
        mutations.closeTopHover();
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
            mutations.closeTopHover();
            const downloadLink = publicApi.getDownloadURL({
              path: "/",
              hash: state.shareInfo.hash,
              token: state.shareInfo.token,
              inline: false,
            }, [state.req.path]);
            window.open(downloadLink + "&format=" + format, "_blank");
          },
        });
      } else {
        // Direct download for single files or directories
        const downloadLink = publicApi.getDownloadURL({
          path: "/",
          hash: state.shareInfo.hash,
          token: state.shareInfo.token,
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
      mutations.closeTopHover();
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
    async showEditShareHover() {
      // Get the current share hash and fetch full share details
      const shareHash = state.shareInfo?.hash;
      if (!shareHash) {
        console.error("No share hash found");
        return;
      }

      try {
        // Fetch the full share details to pass to the edit dialog
        // The shareInfo object should already have most details we need
        const shareData = {
          ...state.shareInfo,
          hash: shareHash,
        };

        mutations.showHover({
          name: "share",
          props: {
            editing: true,
            link: shareData,
          },
        });
      } catch (err) {
        console.error("Failed to open edit share dialog:", err);
      }
    },
    cycleMode() {
      const newMode = state.sidebar.mode === 'links' ? 'navigation' : 'links';
      mutations.setSidebarMode(newMode);
    },
    toggleSourceDropdown() {
      this.showSourceDropdown = !this.showSourceDropdown;
    },
    selectSource(sourceName) {
      this.showSourceDropdown = false;
      this.navigateToSource(sourceName);
    },
    navigateToSource(sourceName) {
      goToItem(sourceName, '/', {});
    },
    closeDropdown(event) {
      if (!this.showSourceDropdown) return;
      const dropdown = this.$refs.dropdown;
      const container = this.$refs.sourceCardContainer;
      if (!dropdown || !container) return;
      if (!container.contains(event.target)) {
        this.showSourceDropdown = false;
      }
    },
  },
};
</script>

<style scoped>
.sidebar-links {
  display: flex;
  flex-direction: column;
  height: 100%;
  padding: 1em;
  overflow: auto;
}

.sidebar-links-header {
  display: flex;
  justify-content: space-between;
  align-items: center;
  padding: 0;
  flex-shrink: 0;
}

.sidebar-links-header .material-icons.action {
  padding: 0.25em 0.25em;
  border-radius: 0.5em;
  transition: background 0.2s;
}

.sidebar-links-header .material-icons.action:hover {
  background: var(--surfaceSecondary);
}

.sidebar-links-header .mode-toggle {
  background: none;
  border: none;
  font-weight: 500;
  color: var(--textPrimary);
  font-size: 1em;
  padding: 0.25em 0.5em;
  border-radius: 0.5em;
  transition: background 0.2s;
}

.sidebar-links-header .mode-toggle:hover {
  background: var(--surfaceSecondary);
  cursor: pointer;
}

.sidebar-links-content {
  flex: 1;
  overflow: auto;
  min-height: 0;
  margin-top: 0.5em;
}

.file-tree-container {
  margin-top: 0;
}

.no-edit-options {
  justify-content: center !important;
}

.share-info-section {
  margin-bottom: 0.5em;
  padding-bottom: 0.25em;
  border-bottom: 1px solid var(--borderColor);
}

.with-top-spacing {
  margin-top: 0.5em;
}

.usage-info .vue-simple-progress {
  border-style: solid;
  border-color: var(--surfaceSecondary);
  border-radius: 1em !important;
}

.sidebar-links .inner-card {
  display: flex;
  justify-content: center;
  align-items: center;
  flex-direction: column;
  width: 100%;
}

/* Non-source link styles (tools, custom) */
.sidebar-link-button {
  margin: 0;
  margin-top: 0.25em;
  padding: 0;
  border-radius: 0.5em;
  justify-content: flex-start;
  max-width: 98%;
}

.sidebar-link-button:first-child {
  margin-top: 0 !important;
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
  min-height: 2.75em;
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
  will-change: opacity, max-height;
}

.expand-enter,
.expand-leave-to {
  height: 0 !important;
  opacity: 0;
}

.source-button {
  margin-top: 0.5em !important;
  display: block !important;
}

.source-button.active {
  background: var(--alt-background);
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
  min-height: 3em;
}

.source-container.has-usage-info {
  min-height: 2.5em;
}

.realtime-pulse {
  width: 2em;
  height: 2em;
  margin: 0.25em;
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
.vue-simple-progress {
  margin-top: 0 !important;
}

.edit-share-button {
  margin-top: 0.5em !important;
  border-top: 1px solid var(--surfaceSecondary);
  padding-top: 0.5em !important;
}

.edit-share-button .link-icon {
  color: var(--primaryColor);
}

.source-card-container {
  position: relative;
  margin-bottom: 0;
  padding-bottom: 0;
  border-bottom: 1px solid var(--borderColor);
}

.navigation-source-card {
  margin-top: 0 !important;
  max-width: 98%;
}

.source-dropdown-button {
  background: none;
  border: none;
  color: var(--textSecondary);
  padding: 0;
  display: flex;
  align-items: center;
  margin-left: auto;
  border-radius: 0.5em;
  size: 1em;
}
.source-dropdown-button:hover {
  color: var(--primaryColor);
}

.source-dropdown {
  position: absolute;
  top: 100%;
  left: 0;
  right: 0;
  background: var(--background);
  border: 2px solid var(--surfaceSecondary);
  border-radius: 0.5em;
  z-index: 10;
  overflow-y: auto;
  min-width: 100%;
  margin-top: 0.2em;
  transform-origin: top center;
}

.dropdown-enter-active,
.dropdown-leave-active {
  transition: opacity 0.2s ease, transform 0.2s ease;
}

.dropdown-enter-from,
.dropdown-leave-to {
  opacity: 0;
  transform: scaleY(0.8);
}

.dropdown-item {
  padding: 0.5em 1em;
}

.dropdown-item:hover {
  background: var(--surfaceSecondary);
}

</style>
