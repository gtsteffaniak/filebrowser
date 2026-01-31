<template>
  <div class="card-title">
    <h2>
      {{ $t('player.QueuePlayback') }}
      <span class="queue-count-badge">{{ queueCount }}</span>
    </h2>
  </div>

  <div class="card-content">
    <!-- Playback mode display -->
    <div class="playback-mode">
      <div class="mode-info">
        <i class="material-icons">{{ currentModeIcon }}</i>
        <span>{{ currentModeLabel }}</span>
      </div>
    </div>

    <!-- Queue list -->
    <div v-if="formattedQueue.length > 0" class="queue-container">
      <div class="file-list" ref="QueueList">
        <div
          v-for="(item, index) in formattedQueue"
          :key="`${item.path}-${index}`"
          class="item"
          :class="{ 'current': index === currentQueueIndex }"
          @click="navigateToItem(index)"
        >
          <div class="item-icon">
            <i class="material-icons">{{ getFileIcon(item) }}</i>
          </div>
      
          <div class="item-name">
            <span class="name">{{ item.name }}</span>
          </div>
          <div class="item-indicator">
            <span v-if="index === currentQueueIndex" class="current-track">
              <!-- eslint-disable-next-line @intlify/vue-i18n/no-raw-text -->
              <i class="material-icons">{{ isPlaying ? 'pause' : 'play_arrow' }}</i>
            </span>
            <span v-else class="track-number">{{ index + 1 }}</span>
          </div>
        </div>
      </div>
    </div>

    <!-- Empty state -->
    <div v-else class="empty">
      <i class="material-icons">queue_music</i>
      <p>{{ $t('player.emptyQueue') }}</p>
    </div>
  </div>

  <div class="card-action">
    <button class="button button--flat" @click.stop="cyclePlaybackModes" :title="$t('player.changePlaybackMode')">
      <i class="material-icons">swap_vert</i> {{ $t('player.changePlaybackMode') }}
    </button>

    <button @click="closeModal" class="button button--flat" :aria-label="$t('general.close')"
      :title="$t('general.close')"> {{ $t('general.close') }}
    </button>
  </div>
</template>

<script>
import { state, mutations } from "@/store";
import { url } from "@/utils";
export default {
  name: "PlaybackQueue",
  computed: {
    playbackQueue() {
      return state.playbackQueue?.queue || [];
    },
    currentQueueIndex() {
      return state.playbackQueue?.currentIndex ?? -1;
    },
    playbackMode() {
      return state.playbackQueue?.mode || 'single';
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
      return this.playbackQueue.map((item) => ({
        name: item.name,
        path: item.path,
        type: item.type
      }));
    },
    isPlaying() {
      return state.playbackQueue?.isPlaying || false;
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
    },
    isPromptVisible: {
      handler(newVal) {
        if (newVal) {
          // Prompt just became visible, scroll to current item
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
      // Auto-scroll when mode changes
      if (newMode !== oldMode) {
        this.$nextTick(() => {
          this.scrollToCurrentItem();
        });
      }
    },
  },
  mounted() {
    // Auto-scroll to current item when prompt opens
    this.$nextTick(() => {
      this.scrollToCurrentItem();
    });
  },
  methods: {
    cyclePlaybackModes() {
      // Cycle through modes using store mutations
      const modes = ['loop-all', 'shuffle', 'sequential', 'loop-single'];
      const currentIndex = modes.indexOf(this.playbackMode);
      const nextMode = modes[(currentIndex + 1) % modes.length];
      // Update store with new mode - this will trigger plyrViewer to rebuild queue
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
        this.navigateToIndex(index);
      }
    },
    togglePlayPause() {
      mutations.togglePlayPause();
    },
    navigateToIndex(index) {
      if (index >= 0 && index < this.playbackQueue.length) {
        const item = this.playbackQueue[index];
        // Update store with new current index
        mutations.setPlaybackQueue({
          queue: this.playbackQueue,
          currentIndex: index,
          mode: this.playbackMode
        });
        // Close the prompt
        this.closeModal();
        // Trigger actual navigation
        this.triggerNavigation(item);
      }
    },
    triggerNavigation(item) {
      url.goToItem( item.source || state.req.source, item.path, undefined );
    },
    scrollToCurrentItem() {
      if (this.queueCount === 0) return;
      this.$nextTick(() => {
        const list = this.$refs.QueueList;
        const currentItem = list.querySelector('.item.current');
        if (currentItem) {
          // Calculate the scroll position to center the current item
          const itemTop = currentItem.offsetTop;
          const itemHeight = currentItem.offsetHeight;
          const listHeight = list.clientHeight;
          const scrollTo = itemTop - (listHeight / 2) + (itemHeight / 2);          
          list.scrollTo({
            top: Math.max(0, scrollTo),
            behavior: 'smooth'
          });
        }
      });
    },
    closeModal() {
      mutations.closeHovers();
    },
    getFileIcon(item) {
      if (item.type?.startsWith('audio/')) return 'audiotrack';
      if (item.type?.startsWith('video/')) return 'movie';
    }
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

.mode-info i.material-icons {
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

.file-list {
  overflow-y: auto;
  min-height: 0;
  align-items: center;
  border-radius: 12px;
  padding: 0;
}

.item {
  display: flex;
  align-items: center;
  text-align: center;
  padding: 0.75rem 1rem;
  cursor: pointer;
  transition: background-color 0.2s ease;
  gap: 0.5rem;
  border-radius: 12px;
}

.item:hover {
  background: var(--surfaceSecondary);
}

.item.current {
  background: var(--primaryColor);
  color: white;
}

.item.current .item-icon i,
.item.current .current-indicator,
.item-indicator {
  color: white;
  user-select: none;
}

.item-icon i.material-icons {
  color: var(--textSecondary);
  user-select: none;
}

.item-name {
  flex: 1;
}

.track-number {
  color: var(--textSecondary);
  font-weight: 600;
  user-select: none;
}

.empty {
  padding: 2rem;
  text-align: center;
  color: var(--textSecondary);
}

.empty i.material-icons {
  font-size: 3rem;
  opacity: 0.5;
  margin-bottom: 1rem;
  user-select: none;
}
</style>