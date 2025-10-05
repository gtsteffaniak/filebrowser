<template>
  <!-- Floating queue button on media player-->
  <button
    v-if="showQueueButton"
    ref="queueButton"
    @click="toggleQueueList"
    class="queue-button"
    :class="{
      'dark-mode': isDarkMode,
      'active': showQueueList
    }"
    :aria-label="$t('player.QueueButtonHint')"
    :title="$t('player.QueueButtonHint')"
  >
    <i class="material-icons">queue_music</i>
    <span v-if="queueCount > 0" class="queue-count">{{ queueCount }}</span>
  </button>

  <!-- Queue prompt/window -->
  <div
    v-if="showQueueList"
    class="queue-overlay"
    @click="closeQueueList"
  >
    <div
      class="queue-prompt card floating"
      @click.stop
      :class="{ 'dark-mode': isDarkMode }"
    >
      <!-- Header  -->
      <div class="card-title">
        <h2>
          {{ $t('player.QueuePlayback') }}
          <span class="queue-count-badge">{{ queueCount }}</span>
        </h2>
      </div>

      <div class="card-content">
        <!-- Playback mode -->
        <div class="playback-controls">
          <div class="mode-display">
            <i class="material-icons">{{ currentModeIcon }}</i>
            <span class="mode-text">{{ currentModeLabel }}</span>
          </div>
        </div>

        <!-- Queue list -->
        <div class="queue-list-container" v-if="formattedQueue.length > 0">
          <div class="file-list" ref="fileListContainer">
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

        <!-- Empty state (Just in case that the queue shows nothing, but I think that will never happen) -->
        <div v-else class="empty-state">
          <i class="material-icons">queue_music</i>
          <p class="empty-title">{{ $t('player.emptyQueue') }}</p>
          <p class="empty-subtitle">{{ $t('player.changePlaybackModeHint') }}</p>
        </div>
      </div>

      <!-- Action buttons -->
      <div class="card-action">
        <button 
          @click="cyclePlaybackMode"
          class="button button--flat"
          :title="$t('player.changePlaybackMode')"
        >
          <i class="material-icons">swap_vert</i>
          <span>{{ $t('player.changePlaybackMode') }}</span>
        </button>
        
        <button 
          @click="closeQueueList" 
          class="button button--flat close-button"
          :aria-label="$t('buttons.close')"
          :title="$t('buttons.close')"
        >
          {{ $t('buttons.close') }}
        </button>
      </div>
    </div>
  </div>
</template>

<script>
import { state, getters } from "@/store";

export default {
  name: "PlaybackQueueUI",
  data() {
    return {
      showQueueList: false,
      plyrViewerRef: null,
      isPlaying: false,
    };
  },
  computed: {
    isDarkMode() {
      return getters.isDarkMode();
    },
    showQueueButton() {
      return state.req && 
             (state.req.type?.startsWith('audio/') || state.req.type?.startsWith('video/')) && 
             state.navigation.enabled;
    },
    playbackQueue() {
      return this.plyrViewerRef?.playbackQueue || [];
    },
    currentQueueIndex() {
      return this.plyrViewerRef?.currentQueueIndex ?? -1;
    },
    playbackMode() {
      return this.plyrViewerRef?.playbackMode || 'single';
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
      return modeLabels[this.playbackMode];
    },
    currentModeIcon() {
      const modeIcons = {
        'single': 'music_note',
        'sequential': 'playlist_play',
        'shuffle': 'shuffle',
        'loop-single': 'repeat_one',
        'loop-all': 'repeat'
      };
      return modeIcons[this.playbackMode];
    },
    formattedQueue() {
      return this.playbackQueue.map((item) => ({
        name: item.name,
        path: item.path,
        type: item.type
      }));
    }
  },
  watch: {
    // Watch for queue index changes to scroll to current item
    currentQueueIndex(newIndex, oldIndex) {
      if (this.showQueueList && newIndex !== oldIndex) {
        this.$nextTick(() => {
          this.scrollToCurrentItem();
        });
      }
    },
    // Watch for when the queue list opens to scroll to current item
    // Also prevent focus of the button for not interfere with shortcuts like pause
    showQueueList(newVal) {
      if (newVal) {
        this.$nextTick(() => {
          this.scrollToCurrentItem();
          if (this.$refs.queueButton) {
            this.$refs.queueButton.blur();
          }
        });
        // Get initial play state immediately
        this.updatePlayState();
      }
    }
  },
  mounted() {
    this.findPlyrViewer();
    document.addEventListener('keydown', this.handleKeydown);
  },
  beforeUnmount() {
    this.removePlyrEventListeners();
    document.removeEventListener('keydown', this.handleKeydown);
  },
  methods: {
    findPlyrViewer() {
      let parent = this.$parent;
      for (let depth = 0; depth < 5 && parent; depth++) {
        if (parent.$options.name === 'plyrViewer') {
          this.plyrViewerRef = parent;
          this.setupPlyrEventListeners();
          break;
        }
        parent = parent.$parent;
      }
    },

    setupPlyrEventListeners() {
      if (!this.plyrViewerRef) return;

      const player = this.plyrViewerRef.getCurrentPlayer();
      if (!player) return;

      if (this.plyrViewerRef.useDefaultMediaPlayer) {
        // HTML5 player events
        // Note: I have no tested with the HTML player, I just added this for try to support both
        player.addEventListener('play', this.handlePlayEvent);
        player.addEventListener('pause', this.handlePauseEvent);
        player.addEventListener('ended', this.handlePauseEvent);
      } else {
        // Plyr events
        const plyrInstance = player.player;
        if (plyrInstance) {
          plyrInstance.on('play', this.handlePlayEvent);
          plyrInstance.on('pause', this.handlePauseEvent);
          plyrInstance.on('ended', this.handlePauseEvent);
        }
      }
    },

    removePlyrEventListeners() {
      if (!this.plyrViewerRef) return;

      const player = this.plyrViewerRef.getCurrentPlayer();
      if (!player) return;

      if (this.plyrViewerRef.useDefaultMediaPlayer) {
        // HTML5 player events
        player.removeEventListener('play', this.handlePlayEvent);
        player.removeEventListener('pause', this.handlePauseEvent);
        player.removeEventListener('ended', this.handlePauseEvent);
      } else {
        // Plyr events
        const plyrInstance = player.player;
        if (plyrInstance) {
          plyrInstance.off('play', this.handlePlayEvent);
          plyrInstance.off('pause', this.handlePauseEvent);
          plyrInstance.off('ended', this.handlePauseEvent);
        }
      }
    },

    handlePlayEvent() {
      this.isPlaying = true;
    },

    handlePauseEvent() {
      this.isPlaying = false;
    },
    
    handleKeydown(event) {
      if (event.key === 'Escape' && this.showQueueList) {
        event.stopPropagation();
        event.preventDefault();
        this.closeQueueList();
        return;
      }
      
      if (event.key.toLowerCase() === 'q' && 
          !event.target.matches('input, textarea, [contenteditable]') &&
          this.showQueueButton &&
          !this.showQueueList) {
        event.stopPropagation();
        this.toggleQueueList();
      }
    },
    
    // Update play/pause states, this is used for update the icon and sync play/pause when clicking on the current item
    updatePlayState() {
      if (!this.plyrViewerRef) return; 
      const player = this.plyrViewerRef.getCurrentPlayer();
      if (!player) {
        this.isPlaying = false;
        return;
      }
      if (this.plyrViewerRef.useDefaultMediaPlayer) {
        this.isPlaying = !player.paused;
      } else {
        const plyrInstance = player.player;
        this.isPlaying = plyrInstance ? plyrInstance.playing : false;
      }
    },
    
    scrollToCurrentItem() {
      if (!this.$refs.fileListContainer || this.currentQueueIndex === -1) return;

      this.$nextTick(() => {
        const container = this.$refs.fileListContainer;
        const currentItem = container.querySelector('.listing-item.current');
        if (currentItem) {
          const scrollTo = currentItem.offsetTop - (container.clientHeight / 2) + (currentItem.clientHeight / 2);
          container.scrollTo({ top: scrollTo, behavior: 'smooth' });
        }
      });
    },

    toggleQueueList() {
      this.showQueueList = !this.showQueueList;
      if (this.showQueueList && !this.plyrViewerRef) {
        this.findPlyrViewer();
      }
    },
    
    closeQueueList() {
      this.showQueueList = false;
    },
        
    cyclePlaybackMode() {
      this.plyrViewerRef?.cyclePlaybackModes?.();
    },
    
    navigateToItem(index) {
      if (index === this.currentQueueIndex) {
        this.plyrViewerRef?.togglePlayPause?.();
      } else {
        this.plyrViewerRef?.navigateToQueueIndex?.(index);
      }
    },
    
    getFileIcon(item) {
      if (item.type?.startsWith('audio/')) return 'audiotrack';
      if (item.type?.startsWith('video/')) return 'movie';
      return 'insert_drive_file';
    }
  }
};
</script>

<style scoped>
/* Float queue button */
.queue-button {
  position: fixed;
  top: 80px;
  right: 20px;
  width: 50px;
  height: 50px;
  border: none;
  border-radius: 50%;
  background: var(--background);
  color: var(--textPrimary);
  cursor: pointer;
  transition: all 0.3s cubic-bezier(0.4, 0, 0.2, 1);
  display: flex;
  align-items: center;
  justify-content: center;
  box-shadow: 0 2px 10px rgba(0, 0, 0, 0.3);
  outline: none;
  z-index: 9999;
}

.queue-button.dark-mode {
  background: var(--surfacePrimary);
}

.queue-button:hover,
.queue-button.active {
  background: var(--primaryColor);
  transform: translateY(-2px) scale(1.05);
  box-shadow: 0 8px 25px rgba(var(--primaryColor-rgb), 0.3), 0 4px 12px rgba(0, 0, 0, 0.2);
  color: white;
}

.queue-button i.material-icons {
  font-size: 24px;
  transition: transform 0.2s ease;
}

.queue-button:hover i.material-icons {
  transform: scale(1.1);
}

.queue-count {
  position: absolute;
  top: -5px;
  right: -5px;
  background: var(--accentColor);
  color: white;
  border-radius: 50%;
  width: 20px;
  height: 20px;
  font-size: 12px;
  display: flex;
  align-items: center;
  justify-content: center;
  font-weight: bold;
}

.card-title {
  padding: 1rem 1.5rem;
  background: var(--altBackground);
  border-bottom: 1px solid var(--borderColor);
  text-align: center;
}

.card-title h2 {
  margin: 0;
  font-size: 1.25rem;
  font-weight: 600;
  color: var(--textPrimary);
  display: inline-flex;
  align-items: center;
  gap: 0.5rem;
}

.queue-count-badge {
  background: var(--primaryColor);
  color: white;
  border-radius: 12px;
  padding: 2px 8px;
  font-size: 0.8rem;
  font-weight: 600;
}

/* Darken and blur the background */
.queue-overlay {
  position: fixed;
  top: 0;
  left: 0;
  right: 0;
  bottom: 0;
  background: rgba(0, 0, 0, 0.5);
  z-index: 9999 !important;
  display: flex;
  align-items: center;
  justify-content: center;
  backdrop-filter: blur(2px);
  animation: overlay-fade-in 0.2s ease-out;
}

/* Queue prompt/window */
.queue-prompt {
  background: var(--background);
  border-radius: 12px;
  box-shadow: 0 10px 30px rgba(0, 0, 0, 0.3);
  width: 90%;
  max-width: 500px;
  max-height: 70vh;
  display: flex;
  flex-direction: column;
  border: 1px solid var(--borderColor);
  animation: prompt-slide-up 0.3s cubic-bezier(0.4, 0, 0.2, 1);
  overflow: hidden;
  z-index: 10001 !important;
}

.queue-prompt.dark-mode {
  background: var(--surfacePrimary);
}

.card-content {
  flex: 1;
  overflow: hidden;
  display: flex;
  flex-direction: column;
  min-height: 0;
  padding: 1rem;
}

/* Playback controls */
.playback-controls {
  display: flex;
  justify-content: space-between;
  align-items: center;
  margin-bottom: 1rem;
  padding: 0.75rem;
  background: var(--altBackground);
  border-radius: 8px;
  border: 1px solid var(--borderColor);
}

/* Current playback mode*/
.mode-display {
  display: flex;
  align-items: center;
  gap: 0.5rem;
  font-weight: 500;
}

.mode-display i.material-icons {
  color: var(--primaryColor);
  font-size: 20px;
}

.mode-text {
  font-size: 0.95rem;
  color: var(--textPrimary);
}

/* Queue list where the items are shown*/
.queue-list-container {
  flex: 1;
  display: flex;
  flex-direction: column;
  min-height: 0;
  overflow: hidden;
}

.file-list {
  flex: 1;
  overflow-x: hidden;
  overflow-y: auto;
  border: 1px solid var(--borderColor);
  border-radius: 12px;
  background: var(--background);
}

/* Scrollbar style */
.file-list::-webkit-scrollbar {
  width: 4px !important;
}

.file-list::-webkit-scrollbar-thumb {
  background: rgba(0, 0, 0, 0.2) !important;
  border-radius: 2px !important;
}

.dark-mode .file-list::-webkit-scrollbar-thumb {
  background: rgba(255, 255, 255, 0.15) !important;
}

.file-list::-webkit-scrollbar-thumb:hover {
  background: var(--primaryColor) !important;
}

/* Listing items */
.listing-item {
  display: flex;
  align-items: center;
  padding: 0.75rem 1rem;
  cursor: pointer;
  transition: all 0.2s ease;
  border-radius: 14px;
  gap: 0.75rem;
  min-height: 50px;
  border-bottom: 1px solid var(--borderColor);
  box-sizing: border-box;
  transform-origin: center;
  border-bottom: none !important;
}

.listing-item:last-child {
  border-bottom: none;
}

.listing-item:hover {
  background: var(--surfaceSecondary);
  transform: scale(1.01);
  border-radius: 14px;
  position: relative;
  z-index: 1;
}

.listing-item.current {
  background: var(--primaryColor);
  color: white;
  border-radius: 14px;
}

.listing-item.current:hover {
  transform: scale(1.01);
  border-radius: 14px;
}

.listing-item.current .item-icon i,
.listing-item.current .current-track,
.listing-item.current .track-number {
  color: white;
}

.item-icon i.material-icons {
  font-size: 20px;
  color: var(--textSecondary);
}

.item-name {
  flex: 1;
  min-width: 0;
}

.name {
  font-size: 0.95rem;
  font-weight: 500;
  word-break: break-word;
  line-height: 1.3;
  overflow: hidden;
  text-overflow: ellipsis;
}

.current-track i.material-icons {
  font-size: 16px;
}

.track-number {
  font-size: 0.8rem;
  color: var(--textSecondary);
  font-weight: 600;
}

/* Empty state */
.empty-state {
  padding: 2rem;
  text-align: center;
  color: var(--textSecondary);
  display: flex;
  flex-direction: column;
  align-items: center;
  justify-content: center;
  gap: 1rem;
  flex: 1;
}

.empty-state i.material-icons {
  font-size: 3rem;
  opacity: 0.5;
}

.empty-title {
  font-size: 1.1rem;
  font-weight: 500;
  margin: 0;
}

.empty-subtitle {
  margin: 0;
  font-size: 0.9rem;
  opacity: 0.7;
}

/* Action buttons */
.card-action {
  display: flex;
  justify-content: space-between;
  align-items: center;
  padding: 1rem;
  border-top: 1px solid var(--borderColor);
  background: var(--altBackground);
}

.card-action .button {
  display: flex;
  align-items: center;
  gap: 0.5rem;
}

.close-button {
  text-align: center;
  justify-content: center;
  min-width: 80px;
}

/* Animations */
@keyframes overlay-fade-in {
  from { opacity: 0; backdrop-filter: blur(0px); }
  to { opacity: 1; backdrop-filter: blur(4px); }
}

@keyframes prompt-slide-up {
  from { opacity: 0; transform: translateY(20px); }
  to { opacity: 1; transform: translateY(0); }
}

/* Mobile */
@media (max-width: 768px) {
  .queue-button {
    width: 44px;
    height: 44px;
    top: 80px;
    right: 16px;
  }

  .queue-button i.material-icons {
    font-size: 20px;
  }

  .queue-count {
    width: 18px;
    height: 18px;
    font-size: 11px;
  }

  .queue-overlay {
    padding: 1rem;
    align-items: flex-start;
    padding-top: 2rem;
  }

  .queue-prompt {
    width: 95%;
    max-width: none; 
    max-height: 85vh; 
    margin: 0 auto; 
    border-radius: 16px;
    overflow: hidden;
  }

  .card-title,
  .card-content,
  .card-action {
    padding: 0.75rem;
  }

  .listing-item {
    padding: 0.75rem 1rem; /* Slightly more padding for touch */
    min-height: 52px;
    gap: 0.875rem;
  }

  .playback-controls {
    flex-direction: column;
    gap: 0.75rem;
    padding: 1rem;
  }

  .mode-display {
    justify-content: center;
    width: 100%;
  }

  .card-action {
    flex-direction: column;
    gap: 0.5rem;
    border-bottom-left-radius: 16px;
    border-bottom-right-radius: 16px;
  }

  .card-action .button {
    width: 100%;
    justify-content: center;
  }

  .close-button {
    order: -1;
  }

  .empty-state {
    padding: 1.5rem;
  }

  .empty-state i.material-icons {
    font-size: 2.5rem;
  }
}

@media (max-width: 480px) {
  .queue-overlay {
    padding: 0.5rem;
    padding-top: 1rem;
  }

  .queue-prompt {
    width: 100%;
    max-height: 90vh;
    border-radius: 12px; 
  }

  .card-title h2 {
    font-size: 1.1rem;
  }

  .listing-item {
    padding: 0.625rem 0.875rem;
    min-height: 48px;
  }

  .name {
    font-size: 0.9rem;
  }
}

/* Landscape orientation on mobile */
@media (max-width: 768px) and (orientation: landscape) {
  .queue-overlay {
    padding: 0.5rem;
    align-items: flex-start;
  }

  .queue-prompt {
    max-height: 90vh;
    width: 95%;
  }

  .file-list {
    max-height: 50vh;
  }
}
</style>
