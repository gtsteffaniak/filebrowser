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
        <input type="radio" id="tab-visualizer" v-model="activeTab" value="visualizer" hidden />
        <label for="tab-visualizer" class="tab-btn" :class="{ active: activeTab === 'visualizer' }">
          <i class="material-symbols">equalizer</i>
          <span>{{ $t('player.visualizer') }}</span>
        </label>
        <div class="tab-indicator" :style="indicatorStyle"></div>
      </div>
    </div>

    <div class="panel-content">
      <div v-if="activeTab === 'queue'" class="tab-queue">
        <PlaybackQueue embedded />
      </div>
      <div v-else-if="activeTab === 'lyrics'" class="tab-lyrics">
        <!-- Lock button -->
        <button
          v-if="lyrics.length && syncedLyrics"
          type="button"
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
              v-for="(line, index) in lyrics"
              :key="index"
              :class="{ active: syncedLyrics && lyrics[index].timestamp === lyrics[activeLyricIndex]?.timestamp, 'no-seek': !syncedLyrics }"
              class="lyric-line"
              @click.stop="syncedLyrics && $emit('seek', line.timestamp)"
              @keydown.enter.stop.prevent="syncedLyrics && $emit('seek', line.timestamp)"
              @keydown.space.stop.prevent="syncedLyrics && $emit('seek', line.timestamp)"
              :role="syncedLyrics ? 'button' : undefined"
              :tabindex="syncedLyrics ? 0 : undefined"
              :aria-label="syncedLyrics ? `Seek to ${line.text}` : undefined"
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
      <div v-else-if="activeTab === 'visualizer'" class="tab-visualizer">
        <canvas ref="visualizerCanvas" class="visualizer-canvas"></canvas>
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
    player: { type: Object, default: null },
    audioContext: { type: Object, default: null },
    audioSource: { type: Object, default: null }, // MediaElementAudioSourceNode, see https://developer.mozilla.org/en-US/docs/Web/API/MediaElementAudioSourceNode
  },
  emits: ["seek"],
  data() {
    return {
      activeTab: (() => {
        const stored = sessionStorage.getItem(LAST_TAB_KEY);
        return stored && ['queue', 'lyrics', 'visualizer'].includes(stored) ? stored : 'queue';
      })(),
      lyricsScrollLocked: false,
      // Visualizer state
      visualizerAnalyser: null,
      visualizerAnimationId: null,
      visualizerActive: false,
      barFrequencyRanges: [], // For each bar { start, end } indices into the FFT data
      barPositions: [],       // For each bar { x, width } in pixels for rendering
      /**
       * Visualizer configs
       * @property {number} barCount        – Number of bars -- more bars looks better but is a bit more expensive
       * @property {number} smoothing       – 0.85–0.95 -- higher = smooth motion
       * @property {number} gain            – 0.3–1.0 -- overall amplitude (loudness)
       * @property {number} freqOffset      – 3–10 -- skips low‑frequency bins
       * @property {number} freqExponent    – 1.0–2.0 -- 1.0 = linear, >1.0 = more bars on bass
       * @property {number} lowBoost        – 0.0–0.5 -- extra gain for the first 2 (left) bars only (bass)
       * @property {number} highBoost       – 0.0–1.0 -- extra gain ramp for high bars (the bars at the right side)
       * @property {number} powerExponent   – 0.7–1.0 -- more lower makes it more dynamic
       */
      visualizerConfig: {
        barCount: 50,
        smoothing: 0.94,
        gain: 0.75,
        freqOffset: 8,
        freqExponent: 1.0,
        lowBoost: 0.10,
        highBoost: 0.5,
        powerExponent: 0.90,
      },
    };
  },
  computed: {
    darkMode() { return getters.isDarkMode(); },
    queueCount() {
      return state.playbackQueue.queue.length;
    },
    syncedLyrics() {
      return this.lyrics.length > 0 && !this.lyrics.every(line => line.timestamp === 0);
    },
    // tabs in the panel
    indicatorStyle() {
      const tabCount = 3;
      const width = 100 / tabCount;
      const tabIndex = ['queue', 'lyrics', 'visualizer'].indexOf(this.activeTab);
      return { width: `${width}%`, left: `${tabIndex * width}%` };
    },
  },
  watch: {
    activeTab(val) {
      // Persist to sessionStorage
      sessionStorage.setItem(LAST_TAB_KEY, val);
      if (val === "visualizer") {
        this.$nextTick(this.startVisualizer);
        return;
      }
      this.stopVisualizer();
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
    player: {
      handler(newPlayer, oldPlayer) {
        if (oldPlayer) {
          this.fullCleanup();
        }
        if (newPlayer && this.activeTab === 'visualizer') {
          this.$nextTick(this.startVisualizer);
        }
      },
      immediate: true,
    },
  },
  mounted() {
    document.addEventListener('keydown', this.onKeyDown);
    this.resizeObserver = new ResizeObserver(() => {
      if (this.activeTab === 'visualizer' && this.visualizerAnalyser) this.resizeVisualizer();
    });
    this.$nextTick(() => {
      const container = this.$el?.querySelector('.tab-visualizer');
      if (container) this.resizeObserver.observe(container);
    });
    this.windowResizeHandler = () => {
      if (this.activeTab === 'visualizer' && this.visualizerAnalyser) this.resizeVisualizer();
    };
    window.addEventListener('resize', this.windowResizeHandler);
  },
  beforeUnmount() {
    document.removeEventListener('keydown', this.onKeyDown);
    this.resizeObserver?.disconnect();
    window.removeEventListener('resize', this.windowResizeHandler);
    this.stopVisualizer();
    this.fullCleanup();
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
      if (this.activeTab === 'queue') {
        this.activeTab = 'lyrics';
      } else if (this.activeTab === 'lyrics') {
        this.activeTab = 'visualizer';
      } else {
        this.activeTab = 'queue';
      }
    },
    initVisualizer() {
      if (this.visualizerAnalyser) return;
      if (!this.audioContext || !this.audioSource) {
        console.warn('AudioPanel: audioContext or audioSource not provided.');
        return;
      }
      try {
        const analyser = this.audioContext.createAnalyser();
        analyser.fftSize = 256;
        analyser.smoothingTimeConstant = this.visualizerConfig.smoothing;
        // Connect the analyser to the source
        this.audioSource.connect(analyser);
        this.visualizerAnalyser = analyser;
        if (this.audioContext.state === 'suspended') {
          this.audioContext.resume();
        }
      } catch (err) {
        console.warn('Visualizer init failed:', err);
      }
    },
    startVisualizer() {
      this.initVisualizer();
      if (!this.visualizerAnalyser || this.visualizerActive) return;
      this.visualizerActive = true;
      this.resizeVisualizer();
      this.drawVisualizer();
    },
    stopVisualizer() {
      if (this.visualizerAnimationId) {
        cancelAnimationFrame(this.visualizerAnimationId);
        this.visualizerAnimationId = null;
      }
      this.visualizerActive = false;
    },
    fullCleanup() {
      this.stopVisualizer();
      if (this.visualizerAnalyser) {
        const analyser = this.visualizerAnalyser;
        try {
          this.audioSource?.disconnect(analyser);
        } catch (_) { /* ignore */ }
        try {
          analyser.disconnect();
        } catch (_) { /* ignore */ }
        this.visualizerAnalyser = null;
      }
    },

    computeGeometry() {
      const canvas = this.$refs.visualizerCanvas;
      if (!canvas) return;
      const analyser = this.visualizerAnalyser;
      if (!analyser) return;
      const width = canvas.clientWidth;
      if (width === 0) return;

      const bufferLength = analyser.frequencyBinCount; // 128
      const { barCount, freqOffset: offset, freqExponent: exponent } = this.visualizerConfig;
      const gap = 1.5;
      const barWidth = (width - (barCount - 1) * gap) / barCount;

      // Compute pixel positions for each bar
      this.barPositions = [];
      let x = 0;
      for (let i = 0; i < barCount; i++) {
        this.barPositions.push({ x, width: barWidth });
        x += barWidth + gap;
      }

      // Compute frequency ranges for each bar
      const maxBin = bufferLength - 1;
      const cutoffs = [];
      for (let i = 0; i <= barCount; i++) {
        const t = i / barCount;
        let idx = Math.floor(Math.pow(t, exponent) * (maxBin - offset)) + offset;
        idx = Math.max(offset, Math.min(maxBin, idx));
        cutoffs.push(idx);
      }
      for (let i = 1; i < cutoffs.length; i++) {
        if (cutoffs.at(i) <= cutoffs.at(i - 1)) {
          cutoffs.splice(i, 1, cutoffs.at(i - 1) + 1);
        }
      }
      if (cutoffs[cutoffs.length - 1] < maxBin) {
        cutoffs[cutoffs.length - 1] = maxBin;
      }

      this.barFrequencyRanges = [];
      for (let i = 0; i < barCount; i++) {
        const start = cutoffs.at(i);
        let end = cutoffs.at(i + 1);
        if (end <= start) end = start + 1;
        this.barFrequencyRanges.push({ start, end });
      }
    },

    drawVisualizer() {
      if (!this.visualizerActive || this.activeTab !== 'visualizer') return;
      if (!this.visualizerAnalyser) return;

      const canvas = this.$refs.visualizerCanvas;
      if (!canvas) return;
      const ctx = canvas.getContext('2d');
      const analyser = this.visualizerAnalyser;
      if (!analyser) {
        this.visualizerAnimationId = requestAnimationFrame(this.drawVisualizer);
        return;
      }

      const width = canvas.clientWidth;
      const height = canvas.clientHeight;
      if (width === 0 || height === 0) {
        this.visualizerAnimationId = requestAnimationFrame(this.drawVisualizer);
        return;
      }

      if (this.barPositions.length === 0 || this.barFrequencyRanges.length === 0) {
        this.computeGeometry();
      }

      // parse primary colour from CSS
      const color = getComputedStyle(document.documentElement)
        .getPropertyValue('--primaryColor').trim() || '#0080ff';
      let r, g, b;
      const hex = /^#?([a-f\d]{2})([a-f\d]{2})([a-f\d]{2})$/i.exec(color);
      if (hex) {
        r = parseInt(hex[1], 16);
        g = parseInt(hex[2], 16);
        b = parseInt(hex[3], 16);
      } else {
        const rgb = /^rgb\(\s*(\d+)\s*,\s*(\d+)\s*,\s*(\d+)\s*\)$/.exec(color);
        if (rgb) {
          r = parseInt(rgb[1], 10);
          g = parseInt(rgb[2], 10);
          b = parseInt(rgb[3], 10);
        } else {
          r = 0; g = 128; b = 255; // fallback
        }
      }

      // compute a "dark" version of the primary color
      const darkR = Math.max(0, r - 40);
      const darkG = Math.max(0, g - 40);
      const darkB = Math.max(0, b - 40);

      const data = new Uint8Array(analyser.frequencyBinCount);
      analyser.getByteFrequencyData(data);

      ctx.clearRect(0, 0, width, height);
      ctx.shadowColor = 'rgba(255,255,255,0.06)';
      ctx.shadowBlur = 4;

      const { barCount, gain, lowBoost, highBoost, powerExponent } = this.visualizerConfig;

      for (let i = 0; i < barCount; i++) {
        const { start, end } = this.barFrequencyRanges.at(i);
        let sum = 0;
        for (let f = start; f < end; f++) sum += data.at(f) || 0;
        const avg = sum / (end - start);
        let scaled = Math.min((avg / 255) * gain, 1);

        // low‑frequency boost (first two left bars)
        if (i < 2) {
          const factor = 1 + lowBoost * (1 - i / 2);
          scaled = Math.min(scaled * factor, 1);
        } else {
          // high‑frequency boost (last bars at the right)
          const highBoostFactor = 1 + highBoost * (i / barCount);
          scaled = Math.min(scaled * highBoostFactor, 1);
        }

        const percent = Math.pow(scaled, powerExponent);
        const barHeight = Math.max(2, percent * height);

        // set color based on bar height
        const rr = Math.round(darkR + (r - darkR) * percent);
        const gg = Math.round(darkG + (g - darkG) * percent);
        const bb = Math.round(darkB + (b - darkB) * percent);
        ctx.fillStyle = `rgb(${rr}, ${gg}, ${bb})`;

        const { x, width: bw } = this.barPositions.at(i);
        const y = height - barHeight;
        const radius = Math.min(3, bw / 2);

        // Use roundRect if available if not fallback to plain fillRect
        if (ctx.roundRect) {
          ctx.beginPath();
          ctx.roundRect(x, y, bw, barHeight, radius);
          ctx.fill();
        } else {
          ctx.fillRect(x, y, bw, barHeight);
        }
      }
      ctx.shadowBlur = 0;
      this.visualizerAnimationId = requestAnimationFrame(this.drawVisualizer);
    },

    resizeVisualizer() {
      const canvas = this.$refs.visualizerCanvas;
      if (!canvas) return;
      const container = canvas.parentElement;
      if (!container) return;
      const rect = container.getBoundingClientRect();
      const dpr = window.devicePixelRatio || 1;
      const w = rect.width;
      const h = rect.height;
      if (w > 0 && h > 0) {
        canvas.width = w * dpr;
        canvas.height = h * dpr;
        canvas.style.width = `${w}px`;
        canvas.style.height = `${h}px`;
        const ctx = canvas.getContext('2d');
        ctx.setTransform(dpr, 0, 0, dpr, 0, 0);
        this.computeGeometry();
      }
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
  padding: 0.5em;
  border: none;
  background: transparent;
  color: var(--textSecondary);
  border-radius: 0.8em;
  cursor: pointer;
  font-size: 0.9rem;
  transition: 0.2s ease;
  position: relative;
  z-index: 1;
  user-select: none;
  width: 100%;
}

.tab-btn.active {
  color: white;
}

.tab-indicator {
  position: absolute;
  top: 0; bottom: 0;
  left: 0;
  background: var(--primaryColor);
  border-radius: 0.8em;
  z-index: 0;
  transition: left 0.35s cubic-bezier(0.25, 0.8, 0.25, 1),
              width 0.35s cubic-bezier(0.25, 0.8, 0.25, 1);
  pointer-events: none;
}

.panel-content {
  flex: 1;
  overflow: hidden;
  display: flex;
  flex-direction: column;
}
.tab-queue,
.tab-lyrics,
.tab-visualizer {
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
  padding: 0.5em 0;
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

.tab-visualizer {
  display: flex;
  align-items: center;
  justify-content: center;
  height: 100%;
  padding: 0.5em;
  box-sizing: border-box;
}

.visualizer-canvas {
  width: 100%;
  height: 100%;
  border-radius: 0.8em;
  background: rgba(0, 0, 0, 0.12);
  display: block;
}
</style>
