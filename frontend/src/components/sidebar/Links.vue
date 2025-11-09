<template>
    <transition name="expand" @before-enter="beforeEnter" @enter="enter" @leave="leave">
        <div v-if="showSidebarLinks" class="sidebar-scroll-list">
            <div class="sidebar-links card">
                <div class="sidebar-links-header">
                    <span>{{ $t("general.links") }}</span>
                    <button class="action edit-icon" @click="openSidebarLinksPrompt"
                        @mouseenter="showTooltip($event, $t('sidebar.customizeLinks'))" @mouseleave="hideTooltip">
                        <i class="material-icons no-padding">edit</i>
                    </button>
                </div>
                <transition-group name="expand" tag="div" class="inner-card">
                    <template v-for="(link, index) in sidebarLinksToDisplay" :key="`link-${index}-${link.category}`">
                        <!-- Source-type links: styled exactly like original sources -->
                        <a v-if="link.category === 'source'" :href="link.target" class="action button source-button sidebar-link-button" :class="{
                            active: isLinkActive(link),
                            disabled: !isLinkAccessible(link)
                        }" @click.prevent="handleLinkClick(link)" :aria-label="link.name">
                            <div class="source-container">
                                <svg v-if="isLinkAccessible(link)" class="realtime-pulse" :class="{
                                    active: realtimeActive,
                                    danger: getSourceInfo(link).status != 'indexing' && getSourceInfo(link).status != 'ready',
                                    warning: getSourceInfo(link).status == 'indexing',
                                    ready: getSourceInfo(link).status == 'ready',
                                }">
                                    <circle class="center" cx="50%" cy="50%" r="7px"></circle>
                                    <circle class="pulse" cx="50%" cy="50%" r="10px"></circle>
                                </svg>
                                <i v-else class="material-icons warning-icon"
                                    @mouseenter="showTooltip($event, $t('sidebar.sourceNotAccessible'))"
                                    @mouseleave="hideTooltip">
                                    warning
                                </i>
                                <span>{{ link.name }}</span>
                                <i v-if="isLinkAccessible(link)"
                                    class="no-select material-symbols-outlined tooltip-info-icon"
                                    @mouseenter="showSourceTooltip($event, getSourceInfo(link))"
                                    @mouseleave="hideTooltip">
                                    info <!-- eslint-disable-line @intlify/vue-i18n/no-raw-text -->
                                </i>
                            </div>
                            <div v-if="hasSourceInfo && isLinkAccessible(link)" class="usage-info">
                                <ProgressBar :val="getSourceInfo(link).used" :max="getSourceInfo(link).total"
                                    unit="bytes"></ProgressBar>
                            </div>
                        </a>

                        <!-- Non-source links: tool and custom links with simple icon style -->
                        <a v-else :href="link.target" class="action button sidebar-link-button" :class="{ active: isLinkActive(link) }"
                            @click.prevent="handleLinkClick(link)">
                            <div class="link-container">
                                <i class="material-icons link-icon">{{ link.icon }}</i>
                                <span>{{ link.name }}</span>
                            </div>
                        </a>
          </template>
                </transition-group>
            </div>
        </div>
    </transition>
</template>

<script>
import { state, getters, mutations } from "@/store";
import { fromNow } from "@/utils/moment";
import ProgressBar from "@/components/ProgressBar.vue";
import { goToItem } from "@/utils/url";

export default {
  name: "SidebarLinks",
  components: {
    ProgressBar,
  },
  computed: {
    user: () => (state.user || {username: 'anonymous'}),
    sourceInfo: () => state.sources.info,
    activeSource: () => state.sources.current,
    realtimeActive: () => state.realtimeActive,
    hasSourceInfo: () => state.sources.hasSourceInfo,
    showSidebarLinks() {
      // Always show sidebar links section (replaces sources)
      return !getters.isShare();
    },
    hasCustomLinks() {
      // Check if user has customized their links
      return this.user?.sidebarLinks && this.user.sidebarLinks.length > 0;
    },
    sidebarLinksToDisplay() {
      // If user has custom links, use those
      if (this.hasCustomLinks) {
        return this.user.sidebarLinks;
      }

      // Otherwise, return default links (sources)
      return this.getDefaultLinks();
    },
  },
  methods: {
    getDefaultLinks() {
      // Generate default links from sources
      const defaultLinks = [];

      if (this.sourceInfo) {
        Object.keys(this.sourceInfo).forEach(sourceName => {
          const info = this.sourceInfo[sourceName];
          defaultLinks.push({
            name: sourceName,
            category: 'source',
            target: `/files/${info.pathPrefix}`,
            icon: 'folder',
          });
        });
      }

      return defaultLinks;
    },
    isLinkAccessible(link) {
      // Check if link is accessible
      if (link.category === 'source') {
        for (const [name] of Object.entries(this.sourceInfo || {})) {
          if (name === link.name) {
            return true;
          }
        }
        return false;
      }
      // Tools and custom links are always accessible
      return true;
    },
    getSourceInfo(link) {
      // Get source info for a source link
      if (link.category !== 'source') return {};
      return this.sourceInfo && link.name ? this.sourceInfo[link.name] || {} : {};
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
    isLinkActive(link) {
      // Check if the current route matches this link
      if (link.category === 'source') {
        return state.req.source === link.name
      }
      return this.$route.path === link.target;
    },
    handleLinkClick(link) {
      // Don't navigate if link is not accessible
      if (!this.isLinkAccessible(link)) {
        return;
      }

      if (link.category === 'source') {
        goToItem(link.name, "/");
        return;
      }

      // Navigate to the link target
      this.navigateTo(link.target);
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

<style scoped>
.sidebar-links {
    display: flex;
    justify-content: center;
    align-items: center;
    flex-direction: column;
    padding: 1em;
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
    justify-content: center;
    padding: .25em;
    padding-top: 0 !important;
    align-items: center;
    gap: 1em;
}

.sidebar-links-header span {
    font-weight: 500;
    color: var(--textPrimary);
}

/* Non-source link styles (tools, custom) */
.sidebar-link-button {
  margin-top: 0.5em;
  text-decoration: none;
  padding: 0.5em;
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
