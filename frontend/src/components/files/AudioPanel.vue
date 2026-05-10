<template>
  <div class="audio-side-panel" :class="{ 'dark-mode': darkMode }">
    <div class="panel-tabs">
      <div class="tab-container">
        <input type="radio" id="tab-queue" v-model="activeTab" value="queue" hidden />
        <label for="tab-queue" class="tab-btn" :class="{ active: activeTab === 'queue' }">
          <i class="material-symbols">queue_music</i>
          <span>{{ $t('player.queue') }}</span>
          <!-- eslint-disable-next-line @intlify/vue-i18n/no-raw-text -->
          <span v-if="queueCount > 0">({{ queueCount }})</span>
        </label>

        <input type="radio" id="tab-lyrics" v-model="activeTab" value="lyrics" hidden />
        <label for="tab-lyrics" class="tab-btn" :class="{ active: activeTab === 'lyrics' }">
          <i class="material-symbols">lyrics</i>
          <span>{{ $t('player.lyrics') }}</span>
        </label>

        <div class="tab-indicator"></div>
      </div>
    </div>

    <div class="panel-content">
      <div v-if="activeTab === 'queue'" class="tab-queue">
        <PlaybackQueue embedded />
      </div>
      <div v-else class="tab-lyrics">
        <!-- Lock button -->
        <button
          v-if="lyrics.length && syncedLyrics"
          class="lyrics-lock-btn"
          @click="lyricsScrollLocked = !lyricsScrollLocked"
          :title="lyricsScrollLocked ? $t('player.unlockLyrics') : $t('player.lockLyrics')"
        >
          <!-- eslint-disable-next-line @intlify/vue-i18n/no-raw-text -->
          <i :class="lyricsScrollLocked ? 'material-symbols-outlined' : 'material-symbols'">{{ lyricsScrollLocked ? 'lock_open' : 'lock' }}</i>
        </button>
        <!-- Scrollable area -->
        <div class="lyrics-scrollable" ref="lyricsScrollable">
          <div v-if="lyrics.length" class="lyrics-list">
              <p
                v-for="(line, i) in lyrics"
                :key="i"
                :class="{ active: syncedLyrics && lyrics[i].timestamp === lyrics[activeLyricIndex]?.timestamp, 'no-seek': !syncedLyrics }"
                class="lyric-line"
                @click.stop="syncedLyrics && $emit('seek', line.timestamp)"
                :role="syncedLyrics ? 'button' : undefined"
                :tabindex="syncedLyrics ? 0 : undefined"
                :aria-label="syncedLyrics ? 'Seek to ' + line.text : undefined"
              >
              {{ line.text }}
            </p>
          </div>
          <div v-else class="no-lyrics">
            <i class="material-symbols">lyrics</i>
            <p>{{ $t('player.noLyrics') }}</p>
          </div>
        </div>
      </div>
    </div>
  </div>
</template>

<script>
import PlaybackQueue from "@/components/prompts/PlaybackQueue.vue";
import { getters, state } from "@/store";

const LAST_TAB_KEY = 'plyrSidePanelActiveTab';

export default {
  name: "AudioPanel",
  components: { PlaybackQueue },
  props: {
    lyrics: { type: Array, default: () => [] },
    activeLyricIndex: { type: Number, default: -1 },
  },
  emits: ["seek"],
  data() {
    return {
      activeTab: sessionStorage.getItem(LAST_TAB_KEY) === 'lyrics' ? 'lyrics' : 'queue',
      lyricsScrollLocked: false,
    };
  },
  computed: {
    darkMode() { return getters.isDarkMode(); },
    queueCount() {
      return state.playbackQueue?.queue?.length || 0;
    },
    syncedLyrics() {
      return this.lyrics.length > 0 && !this.lyrics.every(line => line.timestamp === 0);
    },
  },
  watch: {
    activeTab(val) {
      // Persist to sessionStorage
      sessionStorage.setItem(LAST_TAB_KEY, val);
      // Scroll to active line when switching to lyrics
      if (val === 'lyrics') {
        this.$nextTick(() => this.scrollToActiveLine());
      }
    },
    activeLyricIndex() {
      if (this.activeTab === "lyrics") {
        this.$nextTick(() => this.scrollToActiveLine());
      }
    },
    lyrics: {
      handler() {
        if (this.activeTab === 'lyrics' && this.lyrics.length) {
          this.$nextTick(() => this.scrollToActiveLine());
        }
      },
      immediate: true,
    },
    lyricsScrollLocked(val) {
      if (!val && this.activeTab === 'lyrics' && this.lyrics.length) {
        this.$nextTick(() => this.scrollToActiveLine());
      }
    },
  },
  mounted() {
    document.addEventListener('keydown', this.onKeyDown);
  },
  beforeUnmount() {
    document.removeEventListener('keydown', this.onKeyDown);
  },
  methods: {
    scrollToActiveLine() {
      if (this.lyricsScrollLocked) return;
      const el = this.$refs.lyricsScrollable;
      if (!el) return;
      const active = el.querySelector(".lyric-line.active");
      if (active) {
        active.scrollIntoView({ behavior: "smooth", block: "center" });
      }
    },
    onKeyDown(event) {
      // 'E' shortcut to switch between tabs more easily
      if (event.key.toLowerCase() !== 'e' || event.repeat) return;
      if (event.target.tagName === 'INPUT' || event.target.tagName === 'TEXTAREA') return;
      event.preventDefault();
      this.activeTab = this.activeTab === 'queue' ? 'lyrics' : 'queue';
    },
  },
};
</script>

<style scoped>
.audio-side-panel {
  display: flex;
  flex-direction: column;
  max-height: 65vh;
  background: rgb(216 216 216);
  border-radius: 1em;
  overflow: hidden;
  box-shadow: 0 2px 10px rgba(0,0,0,0.1);
}

.audio-side-panel.dark-mode {
  background: rgb(37 49 55 / 33%);
}

.panel-tabs {
  padding: 0.5em;
  border-bottom: 1px solid var(--divider);
}

/* Container for the sliding indicator */
.tab-container {
  position: relative;
  display: flex;
  gap: 0.5em;
}

.tab-btn {
  flex: 1;
  display: flex;
  align-items: center;
  justify-content: center;
  gap: 0.3em;
  padding: 0.4em;
  border: none;
  background: transparent;
  color: var(--textSecondary);
  border-radius: 0.8em;
  cursor: pointer;
  font-size: 0.9rem;
  transition: 0.2s ease;
  position: relative;
  z-index: 1;
}

.tab-btn.active {
  color: white;
}

.tab-indicator {
  position: absolute;
  top: 0; bottom: 0;
  left: 0;
  width: 50%;
  background: var(--primaryColor);
  border-radius: 0.8em;
  z-index: 0;
  transition: left 0.35s cubic-bezier(0.25, 0.8, 0.25, 1),
              width 0.35s cubic-bezier(0.25, 0.8, 0.25, 1);
  pointer-events: none;
}

/* Shift the indicator to the right when lyrics tab is active */
.tab-container:has(#tab-lyrics:checked) .tab-indicator {
  left: 50%;
}

.panel-content {
  flex: 1;
  overflow: hidden;
  display: flex;
  flex-direction: column;
}
.tab-queue,
.tab-lyrics {
  height: 100%;
}

.tab-queue {
  display: flex;
  flex-direction: column;
}

.tab-lyrics {
  position: relative;
  display: flex;
  flex-direction: column;
}

/* Scrollable wrapper */
.lyrics-scrollable {
  flex: 1;
  overflow-y: auto;
  min-height: 0;
}

.tab-queue :deep(.card-content) {
  flex: 1;
  overflow-y: auto;
  padding: 0.5em;
  margin: 0;
}

.tab-queue :deep(.card-actions) {
  flex-shrink: 0;
  display: flex;
  justify-content: flex-end;
  padding: 0.25em;
}

.tab-queue :deep(.card-actions .button--flat) {
  background: transparent;
}

.lyrics-list {
  padding: 1em;
  text-align: center;
  color: var(--textPrimary);
}

.lyric-line {
  padding: 0.4em 0;
  opacity: 0.6;
  cursor: pointer;
  transition: opacity 0.2s;
  font-size: 1.15rem;
}

.lyric-line:hover {
  opacity: 1;
}

.lyric-line.active {
  opacity: 1;
  font-weight: bold;
  color: var(--primaryColor);
  font-size: 1.35rem;
}

.no-lyrics {
  display: flex;
  flex-direction: column;
  align-items: center;
  justify-content: center;
  height: 100%;
  color: var(--textSecondary);
  gap: 0.5em;
}

.no-lyrics i {
  font-size: 3rem;
  opacity: 0.5;
}

.lyric-line.no-seek {
  cursor: default;
}

.audio-side-panel .tab-lyrics .lyrics-lock-btn {
  position: absolute;
  top: 0.5em;
  right: 0.5em;
  z-index: 10;
  background: var(--background);
  border: 1px solid var(--divider);
  border-radius: 50%;
  width: 2em;
  height: 2em;
  display: flex;
  align-items: center;
  justify-content: center;
  cursor: pointer;
  color: var(--textSecondary);
  transition: 0.2s;
}

.audio-side-panel .tab-lyrics .lyrics-lock-btn:hover {
  background: var(--primaryColor);
  color: white;
  border-color: var(--primaryColor);
}

.tab-btn:hover:not(.active) {
  color: var(--primaryColor);
  transform: scale(1.02);
}
</style>