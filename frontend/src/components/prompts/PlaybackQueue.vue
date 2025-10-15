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
    <div v-if="formattedQueue.length > 0">
      <div class="file-list" ref="QueueList">
        <div
          v-for="(item, index) in formattedQueue"
          :key="`${item.path}-${index}`"
          class="listing-item file"
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

      <button @click="closeModal" class="button button--flat" :aria-label="$t('buttons.close')"
      :title="$t('buttons.close')"> {{ $t('buttons.close') }}
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
          console.log('PlaybackQueue prompt became visible, scrolling to current item');
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
      console.log('Navigate to item:', index);
      
      if (index === this.currentQueueIndex) {
        // Toggle play/pause for current item
        this.togglePlayPause();
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
      // Build the URL for the item
      const itemUrl = url.buildItemUrl(item.source || state.req.source, item.path);
      
      // Update the current request in the store
      mutations.replaceRequest(item);
      
      // Use router to navigate to the new item
      this.$router.replace({ path: itemUrl }).catch(err => {
        if (err.name !== 'NavigationDuplicated') {
          console.error('Router navigation error:', err);
        }
      });
    },
    
    scrollToCurrentItem() {
      this.$nextTick(() => {
        const list = this.$refs.QueueList;
        const currentItem = list.querySelector('.listing-item.current');
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
      }});
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
  margin-left: 5px;
}

.card-content {
  overflow: hidden !important;
  margin-top: 0;
  flex-direction: column;
  padding-left: 15px;
  padding-right: 15px;
  overflow-x: hidden;
}

.card-action {
  display: flex;
  justify-content: space-between;
  align-items: center;
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
}

.playback-mode {
  padding-bottom: 0.75rem;
}

.file-list {
  max-height: 400px;
  overflow-y: auto;
  align-items: center;
  border-radius: 12px;
}

.listing-item {
  display: flex;
  align-items: center;
  text-align: center;
  padding: 0.75rem 1rem;
  cursor: pointer;
  transition: background-color 0.2s ease;
  gap: 0.5rem;
  border-radius: 12px;
}

.listing-item:hover {
  background: var(--surfaceSecondary);
}

.listing-item.current {
  background: var(--primaryColor);
  color: white;
}

.listing-item.current .item-icon i,
.listing-item.current .current-indicator {
  color: white;
}

.item-icon i.material-icons {
  color: var(--textSecondary);
}

.item-name {
  flex: 1;
}

.track-number {
  color: var(--textSecondary);
  font-weight: 600;
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
}
</style>