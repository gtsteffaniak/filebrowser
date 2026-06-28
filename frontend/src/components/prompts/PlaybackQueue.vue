<template>
  <div class="card-content">
    <!-- Playback mode display -->
    <div class="playback-mode">
      <div class="mode-info">
        <i class="material-symbols">{{ currentModeIcon }}</i>
        <span>{{ currentModeLabel }}</span>
      </div>
    </div>

    <!-- Queue list -->
    <div v-if="formattedQueue.length > 0" class="queue-container">
      <div class="queue-list" ref="QueueList">
        <div
          v-for="(item, index) in formattedQueue"
          :key="`${item.path}-${index}`"
          class="queue-item"
          :class="{ 'current': index === currentQueueIndex }"
          @click="navigateToItem(index)"
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
              <span v-if="index === currentQueueIndex" class="current-track">
                <!-- eslint-disable-next-line @intlify/vue-i18n/no-raw-text -->
                <i class="material-symbols">{{ isPlaying ? 'pause' : 'play_arrow' }}</i>
              </span>
              <span v-else class="track-number">{{ index + 1 }}</span>
            </span>
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
    <button
      type="button"
      class="button button--flat"
      @click.stop="cyclePlaybackModes"
      :title="$t('player.changePlaybackMode')"
    >
      <i class="material-symbols">swap_vert</i> {{ $t('player.changePlaybackMode') }}
    </button>
  </div>
</template>

<script>
import { state, mutations, getters } from "@/store";
import { url } from "@/utils";
import { cyclePlaybackModes as cycleModes, formatArtist } from '@/utils/playbackQueue.js';
import Icon from "@/components/files/Icon.vue";
import { resourcesApi } from "@/api";
import { globalVars } from "@/utils/constants";
import { formatDuration, getTypeFromMime, removeExtension } from "@/utils/files.js";

export default {
  name: "PlaybackQueue",
  components: { Icon },
  props: {
    embedded: {
      type: Boolean,
      default: false
    },
    promptId: {
      type: Number,
      default: null,
    },
  },
  computed: {
    playbackQueue() {
      return state.playbackQueue.queue;
    },
    currentQueueIndex() {
      return state.playbackQueue.currentIndex;
    },
    playbackMode() {
      return state.playbackQueue.mode || 'single';
    },
    queueCount() {
      return this.playbackQueue.length;
    },
    currentModeLabel() {
      const modeLabels = {
        'single': this.$t('player.LoopDisabled'),
        'sequential': this.$t('player.PlayAllOncePlayback'),
        'shuffle': this.$t('player.ShuffleAllPlayback'),
        'loop-single': this.$t('player.LoopEnabled'),
        'loop-all': this.$t('player.PlayAllLoopedPlayback')
      };
      return modeLabels[this.playbackMode] || this.$t('player.LoopDisabled');
    },
    currentModeIcon() {
      const modeIcons = {
        'single': 'music_note',
        'sequential': 'playlist_play',
        'shuffle': 'shuffle',
        'loop-single': 'repeat_one',
        'loop-all': 'repeat'
      };
      return modeIcons[this.playbackMode] || 'music_note';
    },
    formattedQueue() {
      return this.playbackQueue.map((item) => {
        const metadata = item.metadata || {};
        // Title: fallback to filename without extension
        const title = metadata.title || removeExtension(item.name);
        // Artist formatted
        const artist = formatArtist(metadata.artist);
        // Album name with year
        let album = '';
        if (metadata.album) {
          album = metadata.album;
          if (metadata.year) {
            album += ` (${metadata.year})`;
          }
        } else if (metadata.year) {
          album = metadata.year;
        }
        // File type
        const fileType = getTypeFromMime(item.type);
        // Duration
        const duration = formatDuration(metadata.duration);
        return {
          name: item.name,
          path: item.path,
          type: item.type,
          source: item.source,
          modified: item.modified,
          size: item.size,
          hasPreview: item.hasPreview,
          thumbnailUrl: this.getThumbnailUrl(item),
          title: title,
          artist: artist,
          album: album,
          fileType: fileType,
          duration: duration,
        };
      });
    },
    isPlaying() {
      return state.playbackQueue.isPlaying || false;
    },
    isPromptVisible() {
      // Check if this PlaybackQueue prompt is the current active prompt
      return state.prompts.some(prompt => prompt.name === 'PlaybackQueue');
    }
  },
  watch: {
    currentQueueIndex(newIndex, oldIndex) {
      // Auto-scroll when current item changes
      if (this.isPromptVisible && newIndex !== oldIndex) {
        this.$nextTick(() => {
          this.scrollToCurrentItem();
        });
      }
      if (this.embedded && newIndex !== oldIndex) {
        this.$nextTick(() => this.scrollToCurrentItem());
      }
    },
    isPromptVisible: {
      handler(newVal) {
        if (!this.embedded && newVal) {
          this.$nextTick(() => {
            setTimeout(() => {
              this.scrollToCurrentItem();
            }, 50);
          });
        }
      },
      immediate: true
    },
    playbackMode(newMode, oldMode) {
      if (newMode !== oldMode && (this.isPromptVisible || this.embedded)) {
        this.$nextTick(() => this.scrollToCurrentItem());
      }
    },
    queueCount() {
      this.updatePromptTitle();
    },
  },
  mounted() {
    this.$nextTick(() => this.scrollToCurrentItem());
    this.updatePromptTitle();
  },
  methods: {
    getThumbnailUrl(item) {
      if (!globalVars.enableThumbs) return "";
      const source = item.source;
      const path = item.path;
      if (!source || !path) return '';
      if (getters.isShare()) {
        return resourcesApi.getPreviewURLPublic(path);
      }
      return resourcesApi.getPreviewURL(source, path, item.modified);
    },
    cyclePlaybackModes() {
      const nextMode = cycleModes(this.playbackMode);
      mutations.setPlaybackQueue({
        queue: this.playbackQueue,
        currentIndex: this.currentQueueIndex,
        mode: nextMode
      });
      // Auto-scroll after mode change
      this.$nextTick(() => {
        this.scrollToCurrentItem();
      });
    },

    navigateToItem(index) {
      if (index === this.currentQueueIndex) {
        // Toggle play/pause for current item
        this.togglePlayPause();
        return;
      } else {
        // Navigate to different item
        mutations.setNavigationTransitioning(true);
        this.navigateToIndex(index);
      }
    },
    togglePlayPause() {
      mutations.togglePlayPause();
    },
    navigateToIndex(index) {
      if (index >= 0 && index < this.playbackQueue.length) {
        const item = this.playbackQueue.at(index);
        // Update store with new current index
        mutations.setPlaybackQueue({
          queue: this.playbackQueue,
          currentIndex: index,
          mode: this.playbackMode
        });
        // Trigger actual navigation
        this.triggerNavigation(item);
      }
    },
    triggerNavigation(item) {
      url.goToItem(item.source || state.req.source, item.path, undefined, false, getters.isShare());
    },
    scrollToCurrentItem() {
      if (this.queueCount === 0) return;
      this.$nextTick(() => {
        const list = this.$refs.QueueList;
        if (!list) return;
        const currentItem = list.querySelector('.queue-item.current');
        if (!currentItem) return;

        const listRect = list.getBoundingClientRect();
        const itemRect = currentItem.getBoundingClientRect();
        const itemTopRelative = itemRect.top - listRect.top + list.scrollTop;
        const itemHeight = currentItem.offsetHeight;
        const listHeight = list.clientHeight;
        const maxScroll = list.scrollHeight - listHeight;

        // Target the item at 35% from the top
        const viewportOffset = listHeight * 0.35;
        const scrollTo = itemTopRelative - viewportOffset + (itemHeight / 2);

        list.scrollTo({
          top: Math.max(0, Math.min(scrollTo, maxScroll)),
          behavior: 'smooth'
        });
      });
    },
    updatePromptTitle() {
      if (this.embedded || this.promptId === null) return;
      const base = this.$t('player.QueuePlayback');
      const title = this.queueCount > 0
        ? `${base} (${this.queueCount})`
        : base;
      mutations.updatePromptTitle(this.promptId, title);
    },
  }
};
</script>

<style scoped>
.queue-count-badge {
  background: var(--primaryColor);
  color: white;
  border-radius: 12px;
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

.card-action {
  display: flex;
  justify-content: space-between;
  align-items: center;
  padding-top: 0 !important;
}

.card-title {
  padding-bottom: 0.2em !important;
}

.card-action .button {
  display: flex;
  align-items: center;
  justify-content: center;
  gap: 4px;
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

.playback-mode {
  padding-bottom: 0.75rem;
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
  border-radius: 12px;
  padding: 0;
}

.queue-item {
  display: flex;
  align-items: center;
  text-align: left;
  padding: 0.2rem 0.85rem;
  cursor: pointer;
  transition: background-color 0.2s ease;
  gap: 0.5rem;
  border-radius: 12px;
}

.queue-item:hover {
  background: var(--surfaceSecondary);
}

.queue-item.current {
  background: var(--primaryColor);
  color: white;
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
}

.queue-item-icon :deep(.image-preview) {
  width: 100%;
  height: 100%;
  --icon-font-size: 1.8em;
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
  border-radius: 1em;
  opacity: 0.7;
  font-weight: 600;
  letter-spacing: 0.02em;
}

.queue-item.current .file-type-badge {
  background: rgba(255,255,255,0.2);
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
</style>
