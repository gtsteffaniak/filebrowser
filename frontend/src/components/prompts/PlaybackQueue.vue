<template>
  <div class="card-content">
    <!-- Playback mode display -->
    <div class="playback-mode">
      <div class="mode-info">
        <i class="material-symbols">{{ currentModeIcon }}</i>
        <span>{{ currentModeLabel }}</span>
        <span v-if="loop !== 'off'" class="loop-badge">
          <i class="material-symbols">{{ loopIcon }}</i>
          <span class="loop-label">{{ loopLabel }}</span>
        </span>
      </div>
      <!-- Clear queue button -->
      <button
        v-if="queueCount > 1"
        type="button"
        class="clear-queue-btn"
        @click="clearQueue"
        :title="$t('player.clearQueue')"
        :aria-label="$t('player.clearQueue')"
      >
        <i class="material-symbols">delete</i>
      </button>
    </div>

    <!-- Queue list -->
    <div v-if="queueCount > 0" class="queue-container">
      <div class="queue-list" ref="QueueList" :class="{ 'uniform-rows': virtual.active.value && hasTallRows }">
        <div data-virtual-group="queue" :style="virtual.group('queue').style">
          <div
            v-for="item in visibleQueue"
            :key="`${item.path}-${item.index}`"
            class="queue-item"
            :data-index="item.index"
            :class="{ 'current': item.index === currentQueueIndex }"
            @click="navigateToItem(item.index)"
          >
            <div class="queue-item-icon">
              <Icon
                :mimetype="item.type"
                :filename="item.name"
                :hasPreview="item.hasPreview"
                :thumbnailUrl="item.thumbnailUrl"
                :modified="item.modified"
                :path="item.path"
                :source="item.source"
                :size="item.size"
              />
              <div v-if="item.index === currentQueueIndex" class="wave-indicator" :class="{ paused: !isPlaying }">
                <!-- Now playing wave indicator -->
                <span></span><span></span><span></span><span></span><span></span>
              </div>
            </div>
            <!-- Metadata -->
            <div class="queue-item-info">
              <div class="queue-item-title">{{ item.title }}</div>
              <div v-if="item.artist" class="queue-item-metadata">{{ item.artist }}</div>
              <div v-if="item.album" class="queue-item-metadata">{{ item.album }}</div>
            </div>
            <!-- Duration + file type + play indicator -->
            <div class="queue-item-meta">
              <span v-if="item.duration" class="queue-item-duration">{{ item.duration }}</span>
              <span v-if="item.fileType" class="file-type-badge">{{ item.fileType }}</span>
              <span class="queue-item-indicator">
                <span v-if="item.index === currentQueueIndex" class="current-track">
                  <!-- eslint-disable-next-line @intlify/vue-i18n/no-raw-text -->
                  <i class="material-symbols">{{ isPlaying ? 'pause' : 'play_arrow' }}</i>
                </span>
                <span v-else class="track-number">{{ item.index + 1 }}</span>
              </span>
            </div>
          </div>
        </div>
      </div>
    </div>

    <!-- Empty state -->
    <div v-else class="empty">
      <i class="material-symbols">queue_music</i>
      <p>{{ $t('player.emptyQueue') }}</p>
    </div>
  </div>

  <div class="card-actions">
    <div class="mode-buttons">
      <div class="mode-indicator" :style="modeIndicatorStyle"></div>
      <input
        type="radio"
        id="mode-sequential"
        name="playback-mode"
        value="sequential"
        :checked="playbackMode === 'sequential'"
        @change="setMode('sequential')"
        hidden
      />
      <label for="mode-sequential" class="mode-btn" :class="{ active: playbackMode === 'sequential' }" :title="$t('player.PlayAllOncePlayback')">
        <i class="material-symbols">playlist_play</i>
      </label>
      <input
        type="radio"
        id="mode-shuffle"
        name="playback-mode"
        value="shuffle"
        :checked="playbackMode === 'shuffle'"
        @change="setMode('shuffle')"
        hidden
      />
      <label for="mode-shuffle" class="mode-btn" :class="{ active: playbackMode === 'shuffle' }" :title="$t('player.ShuffleAllPlayback')">
        <i class="material-symbols">shuffle</i>
      </label>
    </div>
    <button
      type="button"
      class="repeat-one-btn"
      :class="{ active: loop !== 'off' }"
      @click="cycleLoop"
      :title="loopLabel"
      :aria-label="loopLabel"
    >
      <i class="material-symbols">{{ loopIcon }}</i>
    </button>
  </div>
</template>

<script setup lang="ts">
import { computed, nextTick, onMounted, ref, watch } from "vue";
import { useI18n } from "vue-i18n";
import { state, mutations, getters } from "@/store/index.ts";
import type { MediaMetadata } from "@/store/types.ts";
import { url } from "@/utils/index.ts";
import {
  cyclePlaybackModes,
  cycleLoopState,
  toggleSingleLoop,
  clearPlaybackQueue,
  getModeLabel,
  getModeIcon,
  getLoopLabel,
  getLoopIcon,
  formatArtist
} from '@/utils/playbackQueue.js';
import Icon from "@/components/files/Icon.vue";
import { resourcesApi } from "@/api/index.js";
import { globalVars } from "@/utils/constants.js";
import { formatDuration, getTypeFromMime, removeExtension } from "@/utils/files.js";
import { findScrollParents, useListingVirtualization } from "@/composables/useListingVirtualization.ts";
import type { VirtualItem } from "@/composables/useListingVirtualization.ts";

interface QueueItem {
  name: string;
  path: string;
  type: string;
  source?: string;
  modified?: string;
  created?: string;
  size?: number;
  hasPreview?: boolean;
  metadata?: MediaMetadata;
}

const props = withDefaults(
  defineProps<{
    embedded?: boolean;
    promptId?: number | null;
  }>(),
  {
    embedded: false,
    promptId: null,
  }
);

const { t } = useI18n();

const QueueList = ref<HTMLElement | null>(null);
// Only renders the rows around the screen when the queue is long
const virtual = useListingVirtualization<VirtualItem>({
  container: QueueList,
  groups: () => [{ key: "queue", count: state.playbackQueue.queue.length, at: (index) => ({ index }) }],
  scroller: (container) => findScrollParents(container, true),
  itemClass: "queue-item",
  minItems: 10,
});

const playbackQueue = computed(() => state.playbackQueue.queue as QueueItem[]);
const currentQueueIndex = computed(() => state.playbackQueue.currentIndex);
const playbackMode = computed(() => state.playbackQueue.mode || 'single');
const loop = computed(() => state.playbackQueue.loop || 'off');
const queueCount = computed(() => playbackQueue.value.length);
const currentModeLabel = computed(() => getModeLabel(playbackMode.value, t, queueCount.value));
const currentModeIcon = computed(() => getModeIcon(playbackMode.value));
const loopIcon = computed(() => getLoopIcon(loop.value));
const loopLabel = computed(() => getLoopLabel(loop.value, t));
const isPlaying = computed(() => state.playbackQueue.isPlaying || false);
const isPromptVisible = computed(() => state.prompts.some(prompt => prompt.name === 'PlaybackQueue'));
const modeIndicatorStyle = computed(() => {
  const modes = ['sequential', 'shuffle'];
  const index = modes.indexOf(playbackMode.value);
  if (index === -1) return {};
  const width = `${100 / modes.length}%`;
  const left = `${index * (100 / modes.length)}%`;
  return { left, width };
});

// Only the rendered rows are formatted, with their position in the queue
const visibleQueue = computed(() =>
  virtual.group("queue").items.flatMap(({ index }) => {
    const item = playbackQueue.value[index];
    return item ? [{ ...formatQueueItem(item), index }] : [];
  })
);
// Rows with artist and album are taller than the others, but the virtual list needs rows of one height
const hasTallRows = computed(() => playbackQueue.value.some((item) => queueArtist(item) && queueAlbum(item)));
const itemLayout = computed(() => {
  const item = playbackQueue.value[currentQueueIndex.value];
  if (!item) return '';
  return `${queueArtist(item)}|${queueAlbum(item)}`;
});

watch(currentQueueIndex, async (newIndex, oldIndex) => {
  // Auto-scroll when current item changes
  if ((isPromptVisible.value || props.embedded) && newIndex !== oldIndex) {
    await nextTick();
    await scrollToCurrentItem();
  }
});
watch(
  isPromptVisible,
  async (newVal) => {
    if (!props.embedded && newVal) {
      await nextTick();
      setTimeout(() => {
        void scrollToCurrentItem();
      }, 50);
    }
  },
  { immediate: true }
);
watch(playbackMode, async (newMode, oldMode) => {
  if (newMode !== oldMode && (isPromptVisible.value || props.embedded)) {
    await nextTick();
    await scrollToCurrentItem();
  }
});
watch(queueCount, () => {
  updatePromptTitle();
});
watch(itemLayout, async () => {
  if (isPromptVisible.value || props.embedded) {
    await nextTick();
    await scrollToCurrentItem();
  }
});

onMounted(async () => {
  updatePromptTitle();
  await nextTick();
  await scrollToCurrentItem();
});

function queueArtist(item: QueueItem) {
  return formatArtist(item.metadata?.artist ?? "");
}

function queueAlbum(item: QueueItem) {
  const metadata = item.metadata || {};
  // Album name with year
  if (metadata.album) {
    return metadata.year ? `${metadata.album} (${metadata.year})` : metadata.album;
  }
  return metadata.year || '';
}

function formatQueueItem(item: QueueItem) {
  const metadata = item.metadata || {};
  return {
    name: item.name,
    path: item.path,
    type: item.type,
    source: item.source,
    modified: item.modified,
    created: item.created,
    size: item.size,
    hasPreview: item.hasPreview,
    thumbnailUrl: getThumbnailUrl(item),
    // Title: fallback to filename without extension
    title: metadata.title || removeExtension(item.name),
    artist: queueArtist(item),
    album: queueAlbum(item),
    fileType: getTypeFromMime(item.type),
    duration: formatDuration(metadata.duration),
  };
}

function getThumbnailUrl(item: QueueItem) {
  if (!globalVars.enableThumbs) return "";
  const source = item.source;
  const path = item.path;
  if (!source || !path) return '';
  if (getters.isShare()) {
    return resourcesApi.getPreviewURLPublic(path, "small");
  }
  return resourcesApi.getPreviewURL(source, path, item.modified);
}

function setMode(mode: 'sequential' | 'shuffle') {
  if (mode === playbackMode.value) return;
  const listing = state.navigation.listing || state.req?.parentDirItems || [];
  cyclePlaybackModes(playbackMode.value, {
    listing,
    currentItem: state.req,
    isShare: getters.isShare(),
    targetMode: mode
  });
  updatePromptTitle();
}

function cycleLoop() {
  const nextLoop = queueCount.value <= 1 ? toggleSingleLoop(loop.value) : cycleLoopState(loop.value);
  mutations.setPlaybackQueue({
    queue: playbackQueue.value,
    currentIndex: currentQueueIndex.value,
    mode: playbackMode.value,
    loop: nextLoop as typeof loop.value
  });
}

function clearQueue() {
  clearPlaybackQueue();
  updatePromptTitle();
}

function navigateToItem(index: number) {
  if (index === currentQueueIndex.value) {
    // Toggle play/pause for current item
    togglePlayPause();
    return;
  } else {
    // Navigate to different item
    mutations.setNavigationTransitioning(true);
    navigateToIndex(index);
  }
}

function togglePlayPause() {
  mutations.togglePlayPause();
}

function navigateToIndex(index: number) {
  if (index >= 0 && index < playbackQueue.value.length) {
    const item = playbackQueue.value.at(index);
    // Update store with new current index
    mutations.setPlaybackQueue({
      queue: playbackQueue.value,
      currentIndex: index,
      mode: playbackMode.value,
      loop: loop.value
    });
    // Trigger actual navigation
    if (item) triggerNavigation(item);
  }
}

function triggerNavigation(item: QueueItem) {
  url.goToItem(item.source || state.req.source, item.path, undefined, false, getters.isShare());
}

async function scrollToCurrentItem() {
  if (queueCount.value === 0) return;
  await nextTick();
  const list = QueueList.value;
  if (!list) return;
  let currentItem = list.querySelector<HTMLElement>('.queue-item.current');
  const index = currentQueueIndex.value;
  if (!currentItem && virtual.active.value && index >= 0 && index < queueCount.value) {
    // Not rendered: bring the row in, then it is centered like any other
    virtual.scrollToIndex(currentQueueIndex.value, { block: 'center', behavior: 'instant' });
    await virtual.refresh();
    currentItem = list.querySelector<HTMLElement>('.queue-item.current');
  }
  if (!currentItem) return;

  centerCurrentItem(list, currentItem);
}

function centerCurrentItem(list: HTMLElement, item: HTMLElement) {
  const listRect = list.getBoundingClientRect();
  const itemRect = item.getBoundingClientRect();
  const itemTopRelative = itemRect.top - listRect.top + list.scrollTop;
  const itemHeight = item.offsetHeight;
  const listHeight = list.clientHeight;
  const maxScroll = list.scrollHeight - listHeight;

  // Target the item at 35% from the top
  const viewportOffset = listHeight * 0.35;
  const scrollTo = itemTopRelative - viewportOffset + (itemHeight / 2);

  list.scrollTo({
    top: Math.max(0, Math.min(scrollTo, maxScroll)),
    behavior: 'instant'
  });
}

function updatePromptTitle() {
  if (props.embedded || props.promptId === null) return;
  const base = t('player.QueuePlayback');
  const title = queueCount.value > 0
    ? `${base} (${queueCount.value})`
    : base;
  mutations.updatePromptTitle(props.promptId, title);
}
</script>

<style scoped>
.queue-count-badge {
  background: var(--primaryColor);
  color: white;
  border-radius: var(--borderRadius);
  padding: 2px 8px;
  font-size: 0.8rem;
  font-weight: 600;
  vertical-align: middle;
}

.card-content {
  margin-top: 0;
  display: flex;
  flex-direction: column;
  overflow: hidden;
  padding-left: 15px;
  padding-right: 15px;
}

.card-actions {
  display: flex;
  align-items: center;
  gap: 0.75rem;
  justify-content: center !important;
  padding-top: 0 !important;
}

.playback-mode {
  display: flex;
  justify-content: space-between;
  align-items: center;
  padding-bottom: 0.4rem;
  border-bottom: 1px solid var(--divider);
  margin-bottom: 0.12rem;
  flex-shrink: 0;
}

.mode-info {
  display: flex;
  align-items: center;
  gap: 0.5rem;
  font-weight: 500;
}

.mode-info i.material-symbols {
  color: var(--primaryColor);
  user-select: none;
}

.loop-badge {
  display: inline-flex;
  align-items: center;
  gap: 0.2rem;
  margin-left: 0.5rem;
  background: var(--surfaceSecondary);
  padding: 0.1rem 0.6rem;
  border-radius: var(--borderRadius);
  font-size: 0.85rem;
  color: var(--textSecondary);
}

.loop-badge i.material-symbols {
  font-size: 1.1rem;
  color: var(--primaryColor);
}

.clear-queue-btn {
  background: transparent;
  border: none;
  color: var(--textSecondary);
  cursor: pointer;
  padding: 0.25rem 0.5rem;
  border-radius: var(--borderRadius);
  transition: background 0.2s, color 0.2s;
  display: flex;
  align-items: center;
  justify-content: center;
}

.clear-queue-btn:hover {
  background: var(--hoverOverlay);
  color: var(--dangerColor, #e74c3c);
}

.clear-queue-btn i.material-symbols {
  font-size: 1.25rem;
}

.queue-container {
  flex: 1;
  min-height: 0;
  display: flex;
  flex-direction: column;
}

.queue-list {
  overflow-y: auto;
  min-height: 0;
  border-radius: var(--borderRadius);
  padding: 0;
}

.queue-item {
  display: flex;
  align-items: center;
  text-align: left;
  padding: 0.22rem 0.85rem;
  cursor: pointer;
  transition: background-color 0.2s ease;
  gap: 0.5rem;
  border-radius: var(--borderRadius);
}

.queue-item-info {
  flex: 1;
  min-width: 0;
  display: flex;
  flex-direction: column;
  justify-content: center;
  padding: 0.1em 0;
  overflow: hidden;
  line-height: 1.3;
}

.queue-list.uniform-rows .queue-item-info {
  box-sizing: border-box;
  min-height: 3.50em;
}

.queue-item:hover {
  background: var(--hoverOverlay);
}

.queue-item.current {
  background: var(--primaryColor);
  color: white;
}

.mode-btn i,
.repeat-one-btn i {
  font-size: 1.5rem;
}

.queue-item.current .queue-item-icon i,
.queue-item.current .current-indicator,
.queue-item-indicator {
  color: white;
  user-select: none;
}

.queue-item-icon {
  flex-shrink: 0;
  width: 2.65em;
  height: 2.65em;
  border-radius: 6px;
  overflow: hidden;
  position: relative;
}

.queue-item-icon :deep(.image-preview) {
  width: 100%;
  height: 100%;

  --icon-font-size: 1.8em;
}

.queue-item-title {
  font-weight: 500;
  white-space: nowrap;
  overflow: hidden;
  text-overflow: ellipsis;
  font-size: 0.95em;
}

.queue-item-metadata {
  font-size: 0.8em;
  opacity: 0.7;
  white-space: nowrap;
  overflow: hidden;
  text-overflow: ellipsis;
}

.queue-item.current .queue-item-metadata {
  opacity: 0.85;
}

.queue-item-meta {
  display: flex;
  align-items: center;
  gap: 0.5rem;
  flex-shrink: 0;
  font-size: 0.8em;
}

.queue-item-duration {
  opacity: 0.6;
  font-variant-numeric: tabular-nums;
}

.file-type-badge {
  font-size: 0.7rem;
  text-transform: uppercase;
  background: var(--surfaceSecondary);
  padding: 0.1em 0.6em;
  border-radius: var(--borderRadius);
  opacity: 0.7;
  font-weight: 600;
  letter-spacing: 0.02em;
}

.queue-item.current .file-type-badge {
  background: rgb(255 255 255 / 20%);
  color: white;
  opacity: 1;
}

.queue-item-indicator {
  display: flex;
  align-items: center;
  justify-content: center;
  width: 1.8em;
  font-size: 0.9rem;
}

.track-number {
  color: var(--textSecondary);
  font-weight: 600;
  user-select: none;
}

.queue-item.current .track-number {
  color: white;
}

.current-track i.material-symbols {
  font-size: 1.1rem;
}

.empty {
  flex: 1;
  display: flex;
  flex-direction: column;
  align-items: center;
  justify-content: center;
  text-align: center;
  color: var(--textSecondary);
}

.empty i.material-symbols {
  font-size: 3rem;
  opacity: 0.5;
  user-select: none;
}

.mode-buttons {
  display: flex;
  position: relative;
  background: var(--surfaceSecondary);
  border-radius: 2em;
  overflow: hidden;
  padding: 0;
  flex-shrink: 0;
}

.mode-indicator {
  position: absolute;
  top: 0;
  bottom: 0;
  left: 0;
  background: var(--primaryColor);
  border-radius: inherit;
  transition: left 0.35s cubic-bezier(0.25, 0.8, 0.25, 1),
              width 0.35s cubic-bezier(0.25, 0.8, 0.25, 1);
  pointer-events: none;
  z-index: 0;
}

.mode-btn,
.repeat-one-btn {
  background: transparent;
  border: none;
  border-radius: 0;
  padding: 0.35rem 0.75rem;
  cursor: pointer;
  font-size: inherit;
  transition: background 0.2s, color 0.2s, transform 0.2s;
  user-select: none;
  display: flex;
  align-items: center;
  justify-content: center;
}

.mode-btn {
  flex: 1;
  border-radius: 0;
  color: var(--textSecondary);
  z-index: 1;
  position: relative;
}

.mode-btn.active {
  color: white;
}

.mode-btn:hover:not(.active) {
  color: var(--primaryColor);
  transform: scale(1.02);
}

.repeat-one-btn {
  border-radius: 2em;
  color: var(--textSecondary);
}

.repeat-one-btn.active {
  background: var(--primaryColor);
  color: white;
}

.repeat-one-btn:hover:not(.active) {
  color: var(--primaryColor);
  transform: scale(1.02);
}

.repeat-one-btn.active:hover {
  transform: scale(1.02);
}

.wave-indicator {
  position: absolute;
  top: 0;
  left: 0;
  width: 100%;
  height: 100%;
  border-radius: 8px;
  background: rgb(0 0 0 / 40%);
  display: flex;
  align-items: center;
  justify-content: center;
  gap: 3px;
  pointer-events: none;
  z-index: 3;
}

.wave-indicator span {
  display: block;
  width: 3px;
  border-radius: 2px;
  background: #fff;
  box-shadow: 0 0 4px rgb(0 0 0 / 30%);
}

.wave-indicator.paused span {
  animation: none;
  height: 3px;
}

/* 5 bars with different animations */
.wave-indicator:not(.paused) span:nth-child(1) { animation: wave1 2.0s ease-in-out infinite alternate; }
.wave-indicator:not(.paused) span:nth-child(2) { animation: wave2 2.5s ease-in-out infinite alternate; }
.wave-indicator:not(.paused) span:nth-child(3) { animation: wave3 1.8s ease-in-out infinite alternate; }
.wave-indicator:not(.paused) span:nth-child(4) { animation: wave1 2.3s ease-in-out infinite alternate; }
.wave-indicator:not(.paused) span:nth-child(5) { animation: wave2 1.6s ease-in-out infinite alternate; }

@keyframes wave1 {
  0%   { height: 3px; }
  25%  { height: 12px; }
  50%  { height: 5px; }
  75%  { height: 14px; }
  100% { height: 7px; }
}

@keyframes wave2 {
  0%   { height: 8px; }
  30%  { height: 3px; }
  60%  { height: 15px; }
  90%  { height: 5px; }
  100% { height: 10px; }
}

@keyframes wave3 {
  0%   { height: 6px; }
  20%  { height: 14px; }
  45%  { height: 4px; }
  70%  { height: 11px; }
  100% { height: 6px; }
}
</style>
