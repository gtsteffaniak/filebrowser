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
        <button
          type="button"
          class="lyrics-lock-btn"
          :title="$t('player.visualizer')"
          :aria-label="$t('player.visualizer')"
        >
          <i class="material-symbols">tune</i>
        </button>
        <canvas ref="visualizerCanvas" class="visualizer-canvas"></canvas>
      </div>
    </div>
  </div>
</template>

<script>
import PlaybackQueue from "@/components/prompts/PlaybackQueue.vue";
import { getters, state } from "@/store";

const LAST_TAB_KEY = 'plyrSidePanelActiveTab';
const VIS_PAD_LEFT   = 34; // px reserved on the left for Y-axis (dB) labels
const VIS_PAD_BOTTOM = 18; // px reserved at the bottom for X-axis (Hz) labels
const VIS_PAD_TOP    = 10; // px top margin inside the canvas
const VIS_PAD_RIGHT  = 6;  // px right margin inside the canvas

// frequency marks shown in the X-axis (in Hz)
// they are filtered at runtime to show only those inside minFrequency and maxFrequency in the config
const FREQ_LABELS = [
  { hz: 10,    label: '10'  },
  { hz: 20,    label: '20'  },
  { hz: 50,    label: '50'  },
  { hz: 100,   label: '100' },
  { hz: 200,   label: '200' },
  { hz: 500,   label: '500' },
  { hz: 1000,  label: '1k'  },
  { hz: 2000,  label: '2k'  },
  { hz: 5000,  label: '5k'  },
  { hz: 10000, label: '10k' },
  { hz: 20000, label: '20k' },
];

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
        const stored = localStorage.getItem(LAST_TAB_KEY);
        return stored && ['queue', 'lyrics', 'visualizer'].includes(stored) ? stored : 'queue';
      })(),
      lyricsScrollLocked: false,
      // Visualizer state
      visualizerAnalyserLeft: null,
      visualizerAnalyserRight: null,
      visualizerSplitter: null,
      visualizerAnimationId: null,
      visualizerActive: false,
      barFrequencyRanges: [], // for each bar { start, end } indices into the FFT data
      barPositions: [],       // for each bar { x, width } in pixels for rendering
      drawArea: { x: 0, y: 0, w: 0, h: 0 }, // pixel inside axis padding where bars are drawn
      fftDataLeft: null,
      fftDataRight: null,
      peaksLeft: [],
      peaksRight: [],
      primaryColor: "",
      /**
       * Visualizer config
       * @property {number} barCount      – number of bars (split between L/R channels)
       * @property {number} smoothing     – higher = smoother motion, but if set too high will look a bit slow
       * @property {number} fftSize       – FFT size (must be a power of two). Larger = better frequency resolution, but slower.
       * @property {number} minFrequency  – Hz – lowest frequency shown and maps to centre of the stereo display
       * @property {number} maxFrequency  – Hz – highest frequency shown, which gets capped at the maxFreqLimit
       * @property {number} minDecibels   – dBFS – the quietest level that still shows as a bar.
       * @property {number} maxDecibels   – dBFS – the loudest level before clipping.
       * @property {boolean} showScales   – show axis labels and grid lines (if false, bars fill all the canvas)
       * @property {boolean} showPeaks    – show peak indicators above the bars
       */
      visualizerConfig: {
        // I plan to make all of this configurable in UI, so they can be adjusted to taste!
        barCount: 60,
        smoothing: 0.92,
        fftSize: 2048,
        minFrequency: 20,
        maxFrequency: 20000,
        minDecibels: -100,
        maxDecibels: 20,
        showScales: true,
        showPeaks: true,
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
      // Persist to localStorage
      localStorage.setItem(LAST_TAB_KEY, val);
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
      if (this.activeTab === 'visualizer' && this.visualizerAnalyserLeft) this.resizeVisualizer();
    });
    this.$nextTick(() => {
      const container = this.$el?.querySelector('.tab-visualizer');
      if (container) this.resizeObserver.observe(container);
    });
    this.windowResizeHandler = () => {
      if (this.activeTab === 'visualizer' && this.visualizerAnalyserLeft) this.resizeVisualizer();
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
      if (this.visualizerAnalyserLeft || this.visualizerAnalyserRight) return;
      if (!this.audioContext || !this.audioSource) {
        console.warn('AudioPanel: audioContext or audioSource not provided.');
        return;
      }
      let splitter = null;
      let analyserL = null;
      let analyserR = null;
      try {
        const ctx = this.audioContext;
        const source = this.audioSource;
        const { fftSize, smoothing, minDecibels, maxDecibels } = this.visualizerConfig;

        splitter = ctx.createChannelSplitter(2);
        source.connect(splitter);

        analyserL = ctx.createAnalyser();
        analyserL.fftSize = fftSize;
        analyserL.smoothingTimeConstant = smoothing;
        analyserL.minDecibels = minDecibels;
        analyserL.maxDecibels = maxDecibels;
        splitter.connect(analyserL, 0);

        analyserR = ctx.createAnalyser();
        analyserR.fftSize = fftSize;
        analyserR.smoothingTimeConstant = smoothing;
        analyserR.minDecibels = minDecibels;
        analyserR.maxDecibels = maxDecibels;
        splitter.connect(analyserR, 1);

        this.visualizerAnalyserLeft = analyserL;
        this.visualizerAnalyserRight = analyserR;
        this.visualizerSplitter = splitter;

        const binCount = analyserL.frequencyBinCount;
        this.fftDataLeft  = new Float32Array(binCount);
        this.fftDataRight = new Float32Array(binCount);

        // Read primary color once and store it
        const color = getComputedStyle(document.documentElement)
          .getPropertyValue('--primaryColor').trim();
        this.primaryColor = color;

        this.computeGeometry();

        if (this.audioContext.state === 'suspended') {
          this.audioContext.resume();
        }
      } catch (err) {
        // Clean up all in case it fails
        if (splitter) {
          try { this.audioSource?.disconnect(splitter); } catch (_) { /* ignore */ }
          try { splitter.disconnect(); } catch (_) { /* ignore */ }
        }
        if (analyserL) {
          try { splitter?.disconnect(analyserL); } catch (_) { /* ignore */ }
          try { analyserL.disconnect(); } catch (_) { /* ignore */ }
        }
        if (analyserR) {
          try { splitter?.disconnect(analyserR); } catch (_) { /* ignore */ }
          try { analyserR.disconnect(); } catch (_) { /* ignore */ }
        }
        this.visualizerAnalyserLeft = null;
        this.visualizerAnalyserRight = null;
        this.visualizerSplitter = null;
        this.fftDataLeft = null;
        this.fftDataRight = null;
        this.peaksLeft = [];
        this.peaksRight = [];
        console.warn('Visualizer init failed:', err);
      }
    },
    startVisualizer() {
      this.initVisualizer();
      if (!this.visualizerAnalyserLeft || !this.visualizerAnalyserRight || this.visualizerActive) return;
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
      const analysers = [this.visualizerAnalyserLeft, this.visualizerAnalyserRight];
      analysers.forEach((analyser) => {
        if (analyser) {
          try { this.audioSource?.disconnect(analyser); } catch (_) { /* ignore */ }
          try { analyser.disconnect(); } catch (_) { /* ignore */ }
        }
      });
      if (this.visualizerSplitter) {
        try { this.audioSource?.disconnect(this.visualizerSplitter); } catch (_) { /* ignore */ }
        try { this.visualizerSplitter.disconnect(); } catch (_) { /* ignore */ }
        this.visualizerSplitter = null;
      }
      this.visualizerAnalyserLeft = null;
      this.visualizerAnalyserRight = null;
      this.fftDataLeft = null;
      this.fftDataRight = null;
      this.peaksLeft = [];
      this.peaksRight = [];
    },
    computeGeometry() {
      const analyser = this.visualizerAnalyserLeft || this.visualizerAnalyserRight;
      if (!analyser) return;
      const canvas = this.$refs.visualizerCanvas;
      if (!canvas) return;
      const width  = canvas.clientWidth;
      const height = canvas.clientHeight;
      if (width === 0 || height === 0) return;

      const { barCount, minFrequency, maxFrequency, showScales } = this.visualizerConfig;
      // compute padding based on showScales
      const padLeft   = showScales ? VIS_PAD_LEFT   : 0;
      const padRight  = showScales ? VIS_PAD_RIGHT  : 0;
      const padTop    = showScales ? VIS_PAD_TOP    : 0;
      const padBottom = showScales ? VIS_PAD_BOTTOM : 0;

      // drawable area inside the axis padding – this is where bars live
      const drawW = width  - padLeft - padRight;
      const drawH = height - padTop  - padBottom;
      if (drawW <= 0 || drawH <= 0) return;
      this.drawArea = { x: padLeft, y: padTop, w: drawW, h: drawH };

      const halfCount = Math.floor(barCount / 2);

      const gap       = 1.5;
      const centerGap = gap;
      const centerX   = padLeft + drawW / 2; // pixel x of the stereo axis
      const sideWidth = drawW / 2 - centerGap / 2;
      // Bar width
      const barWidth  = (sideWidth - (halfCount - 1) * gap) / halfCount;
      if (barWidth <= 0) return;

      this.barPositions = [];

      // left-channel bars: bar 0 is nearest to centre (lowest freq), the bar halfCount-1 is at the left edge (highest freq)
      for (let i = 0; i < halfCount; i++) {
        const x = (centerX - centerGap / 2 - barWidth) - i * (barWidth + gap);
        this.barPositions.push({ x, width: barWidth, side: 'left' });
      }
      // right-channel bars: same, but at the contrary sides
      for (let i = 0; i < halfCount; i++) {
        const x = (centerX + centerGap / 2) + i * (barWidth + gap);
        this.barPositions.push({ x, width: barWidth, side: 'right' });
      }

      // compute frequency bin ranges using a true logarithmic scale.
      const sampleRate  = this.audioContext?.sampleRate ?? 44100;
      const maxFreqLimit     = sampleRate / 2;
      const binHz       = sampleRate / analyser.fftSize; // Hz per FFT bin
      const bufferLength = analyser.frequencyBinCount;   // = fftSize / 2
      const logMin = Math.log10(Math.max(1, minFrequency));
      const logMax = Math.log10(Math.min(maxFreqLimit, maxFrequency));

      this.barFrequencyRanges = [];
      for (let i = 0; i < halfCount; i++) {
        const t0 = i / halfCount;
        const t1 = (i + 1) / halfCount;
        const fStart   = Math.pow(10, logMin + t0 * (logMax - logMin));
        const fEnd     = Math.pow(10, logMin + t1 * (logMax - logMin));
        const binStart = Math.max(1, Math.round(fStart / binHz));
        const binEnd   = Math.min(bufferLength - 1, Math.round(fEnd / binHz));
        const centerHz = Math.sqrt(fStart * fEnd);
        this.barFrequencyRanges.push({
          start:    binStart,
          end:      Math.max(binStart + 1, binEnd),
          centerHz,
        });
      }
      this.peaksLeft  = new Array(halfCount).fill(0);
      this.peaksRight = new Array(halfCount).fill(0);
    },
    drawVisualizer() {
      if (!this.visualizerActive || this.activeTab !== 'visualizer') return;
      if (!this.visualizerAnalyserLeft || !this.visualizerAnalyserRight) {
        this.visualizerAnimationId = requestAnimationFrame(this.drawVisualizer);
        return;
      }

      const canvas = this.$refs.visualizerCanvas;
      if (!canvas) {
        this.visualizerAnimationId = requestAnimationFrame(this.drawVisualizer);
        return;
      }
      const ctx    = canvas.getContext('2d');
      if (!ctx) {
        this.visualizerAnimationId = requestAnimationFrame(this.drawVisualizer);
        return;
      }
      const width  = canvas.clientWidth;
      const height = canvas.clientHeight;
      if (width === 0 || height === 0) {
        this.visualizerAnimationId = requestAnimationFrame(this.drawVisualizer);
        return;
      }

      const { barCount, minDecibels, maxDecibels } = this.visualizerConfig;
      const halfCount = Math.floor(barCount / 2);

      // ensure geometry is ready before using arrays
      if (
        this.barFrequencyRanges.length !== halfCount ||
        this.barPositions.length !== halfCount * 2
      ) {
        this.computeGeometry();
        this.visualizerAnimationId = requestAnimationFrame(this.drawVisualizer);
        return;
      }

      // values are real decibel numbers (negative, e.g. -45.3 dBFS)
      // -Infinity is returned for bins with no signal
      const dataL = this.fftDataLeft;
      const dataR = this.fftDataRight;
      this.visualizerAnalyserLeft.getFloatFrequencyData(dataL);
      this.visualizerAnalyserRight.getFloatFrequencyData(dataR);

      ctx.clearRect(0, 0, width, height);
      this.drawAxes(ctx);

      ctx.shadowBlur  = 4;
      ctx.shadowColor = 'rgba(255,255,255,0.06)';

      const { y: drawY, h: drawH } = this.drawArea;
      const dbRange = maxDecibels - minDecibels;

      // low‑frequency boost
      for (let i = 0; i < halfCount; i++) {
        const { start, end } = this.barFrequencyRanges.at(i);

        // average dBFS values across all FFT bins that fall in this bar's frequency range
        let sumL = 0, sumR = 0, count = 0;
        for (let f = start; f < end; f++) {
          const dbL = dataL.at(f);
          const dbR = dataR.at(f);
          sumL += Number.isFinite(dbL) ? Math.pow(10, dbL / 10) : 0;
          sumR += Number.isFinite(dbR) ? Math.pow(10, dbR / 10) : 0;
          count++;
        }
        const avgDbL = count > 0 && sumL > 0 ? 10 * Math.log10(sumL / count) : minDecibels;
        const avgDbR = count > 0 && sumR > 0 ? 10 * Math.log10(sumR / count) : minDecibels;

        // normalise dBFS, so the bars directly reflect the audio level
        const normL = Math.max(0, Math.min(1, (avgDbL - minDecibels) / dbRange));
        const normR = Math.max(0, Math.min(1, (avgDbR - minDecibels) / dbRange));

        const barHeightL = Math.max(2, normL * drawH);
        const barHeightR = Math.max(2, normR * drawH);
        const leftBar  = this.barPositions.at(i);
        const rightBar = this.barPositions.at(halfCount + i);

        this.drawBar(ctx, leftBar.x,  drawY + drawH - barHeightL, leftBar.width,  barHeightL);
        this.drawBar(ctx, rightBar.x, drawY + drawH - barHeightR, rightBar.width, barHeightR);
        this.updatePeak(i, normL, 'left');
        this.updatePeak(i, normR, 'right');
      }
      this.drawPeaks(ctx);
      ctx.shadowBlur = 0;
      this.visualizerAnimationId = requestAnimationFrame(this.drawVisualizer);
    },
    // Draw the Y-axis (dB) grid lines & labels on the left and the X-axis (Hz) in the bottom.
    drawAxes(ctx) {
      const { showScales } = this.visualizerConfig;
      if (!showScales) return;

      const { x: drawX, y: drawY, w: drawW, h: drawH } = this.drawArea;
      const { minDecibels, maxDecibels, minFrequency, maxFrequency } = this.visualizerConfig;
      const halfCount = Math.floor(this.visualizerConfig.barCount / 2);
      const dbRange   = maxDecibels - minDecibels;

      ctx.save();
      ctx.font = '10px system-ui, -apple-system, sans-serif';

      const labelColor = 'rgba(255,255,255,0.45)';
      const gridColor  = 'rgba(255,255,255,0.07)';

      const dbStep  = 20; // dB between each grid line (gives 0, -20, -40, -60, -80, -100 for the default range)
      const dbStart = Math.ceil(minDecibels / dbStep) * dbStep;
      for (let db = dbStart; db <= maxDecibels; db += dbStep) {
        const norm = (db - minDecibels) / dbRange;
        const y    = drawY + drawH - norm * drawH;

        ctx.strokeStyle = gridColor;
        ctx.lineWidth   = 1;
        ctx.beginPath();
        ctx.moveTo(drawX,         y);
        ctx.lineTo(drawX + drawW, y);
        ctx.stroke();
        ctx.fillStyle    = labelColor;
        ctx.textAlign    = 'right';
        ctx.textBaseline = 'middle';
        ctx.fillText(`${db}`, drawX - 4, y);
      }

      // convert a target frequency (Hz) to a pixel in x by interpolating between the bar positions
      const sampleRate = this.audioContext?.sampleRate ?? 44100;
      const maxFreqLimit    = sampleRate / 2;
      const effectiveMaxFrequency = Math.min(maxFreqLimit, maxFrequency);
      const logMin  = Math.log10(Math.max(1, minFrequency));
      const logMax  = Math.log10(effectiveMaxFrequency);
      const logRange = logMax - logMin;
      const xAxisY   = drawY + drawH + 13;
      const hzToBarX = (hz, offset) => {
        const clamped = Math.max(minFrequency, Math.min(effectiveMaxFrequency, hz));
        const t       = (Math.log10(clamped) - logMin) / logRange; // 0 = low, 1 = high
        const barIdx  = t * (halfCount - 1);
        const i0 = Math.max(0, Math.min(halfCount - 2, Math.floor(barIdx)));
        const i1 = i0 + 1;
        const frac = barIdx - i0;
        const p0 = this.barPositions.at(offset + i0);
        const p1 = this.barPositions.at(offset + i1);
        if (!p0 || !p1) return null;
        const x0 = p0.x + p0.width / 2;
        const x1 = p1.x + p1.width / 2;
        return x0 + frac * (x1 - x0);
      };

      const visibleTicks = FREQ_LABELS.filter(t => t.hz >= minFrequency && t.hz <= effectiveMaxFrequency);

      ctx.fillStyle    = labelColor;
      ctx.textBaseline = 'alphabetic';
      ctx.textAlign    = 'center';

      const drawnPositions = [];

      const shouldDraw = (x) => {
        const MIN_DIST = 22; // px – if two labels are closer than this skip the second
        for (const pos of drawnPositions) {
          if (Math.abs(pos - x) < MIN_DIST) return false;
        }
        return true;
      };

      // right half - ascending hz means x increases (center to right edge)
      let lastXRight = -Infinity;
      visibleTicks.forEach(({ hz, label }) => {
        const x = hzToBarX(hz, halfCount);
        if (x === null || x - lastXRight < 20) return; // if too near on this side, skip
        if (!shouldDraw(x)) return; // also skip if is too close to a label from the other side
        drawnPositions.push(x);
        lastXRight = x;
        ctx.fillText(label, x, xAxisY);
      });

      // left half - ascending hz means x decreases (center to left edge)
      let lastXLeft = Infinity;
      visibleTicks.forEach(({ hz, label }) => {
        const x = hzToBarX(hz, 0);
        if (x === null || lastXLeft - x < 20) return;
        if (!shouldDraw(x)) return;
        drawnPositions.push(x);
        lastXLeft = x;
        ctx.fillText(label, x, xAxisY);
      });

      ctx.restore();
    },
    drawBar(ctx, x, y, width, height) {
      if (height <= 1) return;
      // Use primary color directly
      ctx.fillStyle = this.primaryColor;
      const radius = Math.min(3, width / 2);
      if (ctx.roundRect) {
        ctx.beginPath();
        ctx.roundRect(x, y, width, height, radius);
        ctx.fill();
      } else {
        ctx.fillRect(x, y, width, height);
      }
    },
    updatePeak(index, currentNorm, side) {
      const peaks = side === 'left' ? this.peaksLeft : this.peaksRight;
      const decay = 0.98;
      const prev = peaks.at(index) ?? 0;
      const newValue = Math.max(currentNorm, prev * decay);
      peaks.splice(index, 1, newValue);
    },
    drawPeaks(ctx) {
      const { showPeaks } = this.visualizerConfig;
      if (!showPeaks) return;

      const { y: drawY, h: drawH } = this.drawArea;
      const halfCount = this.peaksLeft.length;
      for (let i = 0; i < halfCount; i++) {
        const peakL = this.peaksLeft.at(i);
        const peakR = this.peaksRight.at(i);
        if (peakL > 0.01) {
          const leftBar = this.barPositions.at(i);
          const peakY = drawY + drawH - peakL * drawH;
          ctx.fillStyle = 'rgba(255,255,255,0.85)'; // white peak indicators
          ctx.fillRect(leftBar.x, peakY - 1, leftBar.width, 2);
        }
        if (peakR > 0.01) {
          const rightBar = this.barPositions.at(halfCount + i);
          const peakY = drawY + drawH - peakR * drawH;
          ctx.fillStyle = 'rgba(255,255,255,0.85)'; // white peak indicators
          ctx.fillRect(rightBar.x, peakY - 1, rightBar.width, 2);
        }
      }
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
