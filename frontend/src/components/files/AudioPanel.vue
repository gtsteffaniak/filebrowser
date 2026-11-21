<template>
  <div class="audio-side-panel">
    <div class="panel-tabs">
      <SlidingTabs :model-value="activeTab" :tabs="panelTabs" @update:model-value="selectTab" />
    </div>

    <div class="panel-content">
      <div v-if="activeTab === 'queue'" class="tab-queue">
        <PlaybackQueue embedded />
      </div>
      <div v-else-if="activeTab === 'lyrics'" class="tab-lyrics">
        <LyricsList :lyrics="lyrics" :player="player" />
      </div>
      <div v-else-if="activeTab === 'visualizer'" ref="visualizerContainer" class="tab-visualizer">
        <FloatingActionButton
          icon="tune"
          size="small"
          position-mode="absolute"
          :edge-offset="{ top: '0.5em', right: '0.5em' }"
          :auto-hide="false"
          @press="showVisualizerSettings"
          :label="visualizerSettingsLabel"
        />
        <canvas ref="visualizerCanvas" class="visualizer-canvas"></canvas>
      </div>
    </div>
  </div>
</template>

<script setup lang="ts">
import { computed, nextTick, onBeforeUnmount, onMounted, ref, useTemplateRef, watch } from "vue";
import type Plyr from "plyr";
import SlidingTabs from "@/components/settings/SlidingTabs.vue";
import PlaybackQueue from "@/components/prompts/PlaybackQueue.vue";
import FloatingActionButton from "@/components/settings/FloatingActionButton.vue";
import LyricsList from "@/components/files/LyricsList.vue";
import { useI18n } from "@/i18n/index.ts";
import { mutations, state } from "@/store/index.ts";
import type { LyricLines } from "@/utils/lyrics.ts";
import { visualizerConfig } from "@/utils/visualizerConfig.ts";
import type { VisualizerConfig } from "@/utils/visualizerConfig.ts";

const props = withDefaults(
  defineProps<{
    lyrics?: LyricLines;
    player?: Plyr | null;
    audioContext?: AudioContext | null;
    /** MediaElementAudioSourceNode, see https://developer.mozilla.org/en-US/docs/Web/API/MediaElementAudioSourceNode */
    audioSource?: MediaElementAudioSourceNode | null;
  }>(),
  {
    lyrics: () => [],
    player: null,
    audioContext: null,
    audioSource: null,
  }
);

const emit = defineEmits<{
  "needs-audio-graph": [];
}>();

const { t } = useI18n();

type PanelTab = "queue" | "lyrics" | "visualizer";

interface BarPosition {
  x: number;
  width: number;
  side: "left" | "right";
}

interface BarRange {
  start: number;
  end: number;
  centerHz: number;
}

interface DrawArea {
  x: number;
  y: number;
  w: number;
  h: number;
}

type AnalyserSettings = Pick<VisualizerConfig, "fftSize" | "smoothing" | "minDecibels" | "maxDecibels">;

const createFftBuffer = (length: number) => new Float32Array(length);
type FftBuffer = ReturnType<typeof createFftBuffer>;

// Keeps the legacy "plyr" prefix so saved preferences survive.
const LAST_TAB_KEY = "plyrSidePanelActiveTab";
const PANEL_TAB_IDS: readonly PanelTab[] = ["queue", "lyrics", "visualizer"];
const VIS_PAD_LEFT = 34; // px reserved on the left for Y-axis (dB) labels
const VIS_PAD_BOTTOM = 18; // px reserved at the bottom for X-axis (Hz) labels
const VIS_PAD_TOP = 10; // px top margin inside the canvas
const VIS_PAD_RIGHT = 6; // px right margin inside the canvas
const VIS_MIN_DRAW_MS = 1000 / 60 - 3; // ~70 redraws per second
const VIS_MIN_ANALYSIS_MS = 25; // ~30 analyser readings per second
const VIS_FRAME_REF_MS = 1000 / 60; // peak decay
const VIS_EASE_MS = 22; // time constant of the bars easing towards the latest reading
const analyserSmoothing = (smoothing: number) => smoothing ** 2;

// frequency marks shown in the X-axis (in Hz)
// they are filtered at runtime to show only those inside minFrequency and maxFrequency in the config
const FREQ_LABELS: readonly { hz: number; label: string }[] = [
  { hz: 10, label: "10" },
  { hz: 20, label: "20" },
  { hz: 50, label: "50" },
  { hz: 100, label: "100" },
  { hz: 200, label: "200" },
  { hz: 500, label: "500" },
  { hz: 1000, label: "1k" },
  { hz: 2000, label: "2k" },
  { hz: 5000, label: "5k" },
  { hz: 10000, label: "10k" },
  { hz: 20000, label: "20k" },
];

const isPanelTab = (id: string): id is PanelTab => (PANEL_TAB_IDS as readonly string[]).includes(id);

function readStoredTab(): PanelTab {
  const stored = localStorage.getItem(LAST_TAB_KEY);
  return stored && isPanelTab(stored) ? stored : "queue";
}

const activeTab = ref<PanelTab>(readStoredTab());

const visualizerContainer = useTemplateRef<HTMLElement>("visualizerContainer");
const visualizerCanvas = useTemplateRef<HTMLCanvasElement>("visualizerCanvas");

let resizeObserver: ResizeObserver | null = null;
let analyserLeft: AnalyserNode | null = null;
let analyserRight: AnalyserNode | null = null;
let splitter: ChannelSplitterNode | null = null;
let connectedSource: AudioNode | null = null;
let animationId: number | null = null;
let currentRunId = 0;
let visualizerActive = false;
let barFrequencyRanges: BarRange[] = []; // for each bar, indices into the FFT data
let barPositions: BarPosition[] = []; // for each bar, x and width in pixels for rendering
let drawArea: DrawArea = { x: 0, y: 0, w: 0, h: 0 }; // pixels inside the axis padding where the bars are drawn
let fftDataLeft: FftBuffer | null = null;
let fftDataRight: FftBuffer | null = null;
let peaksLeft: number[] = [];
let peaksRight: number[] = [];
let primaryColor = "";
let lastAppliedConfig: AnalyserSettings | null = null;
let lastDrawTime = 0; // performance.now() of the last drawn frame
let lastAnalysisTime = 0; // performance.now() of the last analyser reading
let targetsLeft: number[] = []; // level (0..1) of each bar at the last analyser reading
let targetsRight: number[] = [];
let levelsLeft: number[] = []; // level shown, eased towards the target
let levelsRight: number[] = [];
let axesCache: { key: string; canvas: HTMLCanvasElement } | null = null;
let settledFrames = 0; // consecutive frames without signal

const queueCount = computed(() => state.playbackQueue.queue.length);
const visualizerSettingsLabel = computed(() => t("player.visualizer.settings"));
const panelTabs = computed(() => [
  {
    id: "queue",
    icon: "queue_music",
    label: t("player.queue"),
    title: t("player.QueuePlayback"),
    suffix: queueCount.value > 0 ? `(${queueCount.value})` : undefined,
  },
  { id: "lyrics", icon: "lyrics", label: t("player.lyrics") },
  { id: "visualizer", icon: "equalizer", label: t("player.visualizer.title") },
]);

function selectTab(id: string) {
  if (isPanelTab(id)) activeTab.value = id;
}

function onResize() {
  if (activeTab.value === "visualizer" && analyserLeft) resizeVisualizer();
}

function onVisibilityChange() {
  if (activeTab.value !== "visualizer") return;
  if (document.hidden) {
    stopVisualizer();
  } else {
    startVisualizer();
  }
}

function onKeyDown(event: KeyboardEvent) {
  // 'E' shortcut to switch between tabs more easily
  if (event.key.toLowerCase() !== "e" || event.repeat) return;
  const target = event.target as HTMLElement | null;
  if (target?.tagName === "INPUT" || target?.tagName === "TEXTAREA") return;
  event.preventDefault();
  if (activeTab.value === "queue") {
    activeTab.value = "lyrics";
  } else if (activeTab.value === "lyrics") {
    activeTab.value = "visualizer";
  } else {
    activeTab.value = "queue";
  }
}

function showVisualizerSettings() {
  mutations.showPrompt({ name: "VisualizerSettings" });
}

const analyserSettings = ({ fftSize, smoothing, minDecibels, maxDecibels }: VisualizerConfig): AnalyserSettings => ({
  fftSize,
  smoothing,
  minDecibels,
  maxDecibels,
});

function configureAnalysers(analyserL: AnalyserNode, analyserR: AnalyserNode, config: VisualizerConfig) {
  const settings = analyserSettings(config);
  for (const analyser of [analyserL, analyserR]) {
    analyser.fftSize = settings.fftSize;
    analyser.smoothingTimeConstant = analyserSmoothing(settings.smoothing);
    analyser.minDecibels = settings.minDecibels;
    analyser.maxDecibels = settings.maxDecibels;
  }
  fftDataLeft = createFftBuffer(analyserL.frequencyBinCount);
  fftDataRight = createFftBuffer(analyserR.frequencyBinCount);
  lastAppliedConfig = settings;
}

function applyVisualizerConfig(config: VisualizerConfig) {
  const next = analyserSettings(config);
  const changed =
    !lastAppliedConfig || (Object.keys(next) as (keyof AnalyserSettings)[]).some((key) => next[key] !== lastAppliedConfig?.[key]);
  if (analyserLeft && analyserRight && changed) {
    try {
      configureAnalysers(analyserLeft, analyserRight, config);
    } catch (err) {
      console.warn("Visualizer config update failed:", err);
    }
  }
  computeGeometry();
}

function initVisualizer() {
  if (analyserLeft || analyserRight) return;
  const ctx = props.audioContext;
  const source = props.audioSource;
  if (!ctx || !source) {
    console.warn("AudioPanel: audioContext or audioSource not provided.");
    return;
  }
  try {
    splitter = ctx.createChannelSplitter(2);
    source.connect(splitter);
    connectedSource = source;

    analyserLeft = ctx.createAnalyser();
    splitter.connect(analyserLeft, 0);
    analyserRight = ctx.createAnalyser();
    splitter.connect(analyserRight, 1);
    configureAnalysers(analyserLeft, analyserRight, visualizerConfig);

    primaryColor = getComputedStyle(document.documentElement).getPropertyValue("--primaryColor").trim();
    computeGeometry();
    if (ctx.state === "suspended") ctx.resume().catch(() => undefined);
  } catch (err) {
    fullCleanup();
    console.warn("Visualizer init failed:", err);
  }
}

function startVisualizer() {
  initVisualizer();
  if (!analyserLeft || !analyserRight || visualizerActive) return;
  visualizerActive = true;
  lastDrawTime = 0;
  lastAnalysisTime = 0;
  currentRunId += 1;
  const container = visualizerContainer.value;
  if (container) resizeObserver?.observe(container);
  resizeVisualizer();
  drawVisualizer(currentRunId);
}

function stopVisualizer() {
  currentRunId += 1;
  if (animationId) {
    cancelAnimationFrame(animationId);
    animationId = null;
  }
  visualizerActive = false;
}

function fullCleanup() {
  stopVisualizer();
  resizeObserver?.disconnect();
  try {
    if (splitter) connectedSource?.disconnect(splitter);
  } catch {
    /* ignore */
  }
  for (const node of [splitter, analyserLeft, analyserRight]) {
    try {
      node?.disconnect();
    } catch {
      /* ignore */
    }
  }
  splitter = null;
  connectedSource = null;
  analyserLeft = null;
  analyserRight = null;
  fftDataLeft = null;
  fftDataRight = null;
  axesCache = null;
  settledFrames = 0;
  resetLevels(0);
}

function resetLevels(barsPerSide: number) {
  const zeros = () => new Array<number>(barsPerSide).fill(0);
  peaksLeft = zeros();
  peaksRight = zeros();
  targetsLeft = zeros();
  targetsRight = zeros();
  levelsLeft = zeros();
  levelsRight = zeros();
}

function computeGeometry() {
  settledFrames = 0; // resizing clears the canvas
  const analyser = analyserLeft || analyserRight;
  if (!analyser) return;
  const canvas = visualizerCanvas.value;
  if (!canvas) return;
  const width = canvas.clientWidth;
  const height = canvas.clientHeight;
  if (width === 0 || height === 0) return;

  const { barCount, minFrequency, maxFrequency, showScales } = visualizerConfig;
  // compute padding based on showScales
  const padLeft = showScales ? VIS_PAD_LEFT : 0;
  const padRight = showScales ? VIS_PAD_RIGHT : 0;
  const padTop = showScales ? VIS_PAD_TOP : 0;
  const padBottom = showScales ? VIS_PAD_BOTTOM : 0;

  // drawable area inside the axis padding
  const drawW = width - padLeft - padRight;
  const drawH = height - padTop - padBottom;
  if (drawW <= 0 || drawH <= 0) return;
  drawArea = { x: padLeft, y: padTop, w: drawW, h: drawH };

  const halfCount = Math.floor(barCount / 2);

  const gap = 1.5;
  const centerGap = gap;
  const centerX = padLeft + drawW / 2; // pixel x of the stereo axis
  const sideWidth = drawW / 2 - centerGap / 2;
  const barWidth = (sideWidth - (halfCount - 1) * gap) / halfCount;
  if (barWidth <= 0) return;

  barPositions = [];

  // left-channel bars: bar 0 is nearest to centre (lowest freq), the bar halfCount-1 is at the left edge (highest freq)
  for (let i = 0; i < halfCount; i++) {
    const x = centerX - centerGap / 2 - barWidth - i * (barWidth + gap);
    barPositions.push({ x, width: barWidth, side: "left" });
  }
  // right-channel bars: same, but at the contrary sides
  for (let i = 0; i < halfCount; i++) {
    const x = centerX + centerGap / 2 + i * (barWidth + gap);
    barPositions.push({ x, width: barWidth, side: "right" });
  }

  // compute frequency ranges using a logarithmic scale.
  const sampleRate = props.audioContext?.sampleRate ?? 44100;
  const maxFreqLimit = sampleRate / 2;
  const binHz = sampleRate / analyser.fftSize; // Hz per FFT bin
  const bufferLength = analyser.frequencyBinCount; // = fftSize / 2
  const logMin = Math.log10(Math.max(1, minFrequency));
  const logMax = Math.log10(Math.min(maxFreqLimit, maxFrequency));

  barFrequencyRanges = [];
  for (let i = 0; i < halfCount; i++) {
    const t0 = i / halfCount;
    const t1 = (i + 1) / halfCount;
    const fStart = 10 ** (logMin + t0 * (logMax - logMin));
    const fEnd = 10 ** (logMin + t1 * (logMax - logMin));
    const binStart = Math.max(1, Math.round(fStart / binHz));
    const binEnd = Math.min(bufferLength - 1, Math.round(fEnd / binHz));
    const centerHz = Math.sqrt(fStart * fEnd);
    barFrequencyRanges.push({
      start: binStart,
      end: Math.max(binStart + 1, binEnd),
      centerHz,
    });
  }
  if (levelsLeft.length !== halfCount) resetLevels(halfCount);
}

function scheduleFrame(runId: number) {
  animationId = requestAnimationFrame(() => drawVisualizer(runId));
}

function analyse(
  analyserL: AnalyserNode,
  analyserR: AnalyserNode,
  dataL: FftBuffer,
  dataR: FftBuffer,
  halfCount: number
) {
  const { minDecibels, maxDecibels } = visualizerConfig;
  analyserL.getFloatFrequencyData(dataL);
  analyserR.getFloatFrequencyData(dataR);

  const dbRange = maxDecibels - minDecibels;
  for (let i = 0; i < halfCount; i++) {
    const range = barFrequencyRanges.at(i);
    if (!range) continue;
    // average dBFS values across all FFT bins that fall in this bar frequency range
    let sumL = 0;
    let sumR = 0;
    let count = 0;
    for (let f = range.start; f < range.end; f++) {
      const dbL = dataL.at(f) ?? Number.NEGATIVE_INFINITY;
      const dbR = dataR.at(f) ?? Number.NEGATIVE_INFINITY;
      sumL += Number.isFinite(dbL) ? 10 ** (dbL / 10) : 0;
      sumR += Number.isFinite(dbR) ? 10 ** (dbR / 10) : 0;
      count++;
    }
    const avgDbL = count > 0 && sumL > 0 ? 10 * Math.log10(sumL / count) : minDecibels;
    const avgDbR = count > 0 && sumR > 0 ? 10 * Math.log10(sumR / count) : minDecibels;
    // normalise dBFS
    targetsLeft[i] = Math.max(0, Math.min(1, (avgDbL - minDecibels) / dbRange));
    targetsRight[i] = Math.max(0, Math.min(1, (avgDbR - minDecibels) / dbRange));
  }
}

function drawVisualizer(runId: number) {
  if (runId !== currentRunId) return;
  if (!visualizerActive || activeTab.value !== "visualizer") return;
  const now = performance.now();
  const frameDelta = now - lastDrawTime;
  if (frameDelta < VIS_MIN_DRAW_MS) {
    scheduleFrame(runId);
    return;
  }
  lastDrawTime = now;

  const analyserL = analyserLeft;
  const analyserR = analyserRight;
  const dataL = fftDataLeft;
  const dataR = fftDataRight;
  if (!analyserL || !analyserR || !dataL || !dataR) {
    scheduleFrame(runId);
    return;
  }
  const canvas = visualizerCanvas.value;
  const ctx = canvas?.getContext("2d");
  if (!canvas || !ctx) {
    scheduleFrame(runId);
    return;
  }
  const width = canvas.clientWidth;
  const height = canvas.clientHeight;
  if (width === 0 || height === 0) {
    scheduleFrame(runId);
    return;
  }
  const { barCount } = visualizerConfig;
  const halfCount = Math.floor(barCount / 2);

  // ensure geometry is ready before using arrays
  if (barFrequencyRanges.length !== halfCount || barPositions.length !== halfCount * 2) {
    computeGeometry();
    scheduleFrame(runId);
    return;
  }
  // idle while paused, once the bars and peaks have settled
  const playing = props.player ? props.player.playing : true;
  if (!playing && settledFrames >= 3) {
    scheduleFrame(runId);
    return;
  }
  if (now - lastAnalysisTime >= VIS_MIN_ANALYSIS_MS) {
    lastAnalysisTime = now;
    analyse(analyserL, analyserR, dataL, dataR, halfCount);
  }
  ctx.clearRect(0, 0, width, height);
  drawAxesCached(ctx, canvas, width, height);

  const { y: drawY, h: drawH } = drawArea;
  const ease = 1 - Math.exp(-Math.min(frameDelta, 100) / VIS_EASE_MS);
  let hasSignal = false;

  for (let i = 0; i < halfCount; i++) {
    const leftBar = barPositions.at(i);
    const rightBar = barPositions.at(halfCount + i);
    if (!leftBar || !rightBar) continue;
    const targetL = targetsLeft.at(i) ?? 0;
    const targetR = targetsRight.at(i) ?? 0;
    const levelL = (levelsLeft.at(i) ?? 0) + (targetL - (levelsLeft.at(i) ?? 0)) * ease;
    const levelR = (levelsRight.at(i) ?? 0) + (targetR - (levelsRight.at(i) ?? 0)) * ease;
    levelsLeft[i] = levelL;
    levelsRight[i] = levelR;

    const barHeightL = Math.max(2, levelL * drawH);
    const barHeightR = Math.max(2, levelR * drawH);

    drawBar(ctx, leftBar.x, drawY + drawH - barHeightL, leftBar.width, barHeightL);
    drawBar(ctx, rightBar.x, drawY + drawH - barHeightR, rightBar.width, barHeightR);
    updatePeak(i, levelL, "left", frameDelta);
    updatePeak(i, levelR, "right", frameDelta);
    if (levelL > 0.002 || levelR > 0.002 || (peaksLeft.at(i) ?? 0) > 0.01 || (peaksRight.at(i) ?? 0) > 0.01) {
      hasSignal = true;
    }
  }
  settledFrames = hasSignal ? 0 : settledFrames + 1;
  drawPeaks(ctx);
  scheduleFrame(runId);
}

function drawAxesCached(ctx: CanvasRenderingContext2D, canvas: HTMLCanvasElement, width: number, height: number) {
  const { showScales, minDecibels, maxDecibels, minFrequency, maxFrequency, barCount } = visualizerConfig;
  if (!showScales) return;
  const sampleRate = props.audioContext?.sampleRate ?? 44100;
  const key = `${canvas.width}x${canvas.height}|${width}x${height}|${minDecibels}|${maxDecibels}|${minFrequency}|${maxFrequency}|${barCount}|${sampleRate}`;
  let cache = axesCache;
  if (!cache || cache.key !== key) {
    const layer = cache?.canvas ?? document.createElement("canvas");
    layer.width = canvas.width;
    layer.height = canvas.height;
    const layerCtx = layer.getContext("2d");
    if (!layerCtx) return;
    const dpr = window.devicePixelRatio || 1;
    layerCtx.setTransform(dpr, 0, 0, dpr, 0, 0);
    drawAxes(layerCtx);
    cache = { key, canvas: layer };
    axesCache = cache;
  }
  ctx.save();
  ctx.setTransform(1, 0, 0, 1, 0, 0);
  ctx.drawImage(cache.canvas, 0, 0);
  ctx.restore();
}

// Draw the Y-axis (dB) grid lines & labels on the left and the X-axis (Hz) in the bottom.
function drawAxes(ctx: CanvasRenderingContext2D) {
  const { showScales, minDecibels, maxDecibels, minFrequency, maxFrequency, barCount } = visualizerConfig;
  if (!showScales) return;
  const { x: drawX, y: drawY, w: drawW, h: drawH } = drawArea;
  const halfCount = Math.floor(barCount / 2);
  const dbRange = maxDecibels - minDecibels;

  ctx.save();
  ctx.font = "10px system-ui, -apple-system, sans-serif";

  const labelColor = "rgba(255,255,255,0.45)";
  const gridColor = "rgba(255,255,255,0.07)";

  const dbStep = 20; // dB between each grid line (gives 0, -20, -40, -60, -80, -100)
  const dbStart = Math.ceil(minDecibels / dbStep) * dbStep;
  for (let db = dbStart; db <= maxDecibels; db += dbStep) {
    const norm = (db - minDecibels) / dbRange;
    const y = drawY + drawH - norm * drawH;

    ctx.strokeStyle = gridColor;
    ctx.lineWidth = 1;
    ctx.beginPath();
    ctx.moveTo(drawX, y);
    ctx.lineTo(drawX + drawW, y);
    ctx.stroke();
    ctx.fillStyle = labelColor;
    ctx.textAlign = "right";
    ctx.textBaseline = "middle";
    ctx.fillText(`${db}`, drawX - 4, y);
  }
  // convert a target frequency (Hz) to a pixel in x by interpolating between the bar positions
  const sampleRate = props.audioContext?.sampleRate ?? 44100;
  const maxFreqLimit = sampleRate / 2;
  const effectiveMaxFrequency = Math.min(maxFreqLimit, maxFrequency);
  const logMin = Math.log10(Math.max(1, minFrequency));
  const logMax = Math.log10(effectiveMaxFrequency);
  const logRange = logMax - logMin;
  const xAxisY = drawY + drawH + 13;
  const hzToBarX = (hz: number, offset: number): number | null => {
    const clamped = Math.max(minFrequency, Math.min(effectiveMaxFrequency, hz));
    const tick = (Math.log10(clamped) - logMin) / logRange; // 0 = low, 1 = high
    const barIdx = tick * (halfCount - 1);
    const i0 = Math.max(0, Math.min(halfCount - 2, Math.floor(barIdx)));
    const i1 = i0 + 1;
    const frac = barIdx - i0;
    const p0 = barPositions.at(offset + i0);
    const p1 = barPositions.at(offset + i1);
    if (!p0 || !p1) return null;
    const x0 = p0.x + p0.width / 2;
    const x1 = p1.x + p1.width / 2;
    return x0 + frac * (x1 - x0);
  };
  const visibleTicks = FREQ_LABELS.filter((tick) => tick.hz >= minFrequency && tick.hz <= effectiveMaxFrequency);

  ctx.fillStyle = labelColor;
  ctx.textBaseline = "alphabetic";
  ctx.textAlign = "center";

  const drawnPositions: number[] = [];

  const shouldDraw = (x: number) => {
    const MIN_DIST = 22; // in px - if 2 labels are closer than this skip the second
    for (const pos of drawnPositions) {
      if (Math.abs(pos - x) < MIN_DIST) return false;
    }
    return true;
  };
  // right - ascending hz means x increases (center to right edge)
  let lastXRight = Number.NEGATIVE_INFINITY;
  for (const { hz, label } of visibleTicks) {
    const x = hzToBarX(hz, halfCount);
    if (x === null || x - lastXRight < 20) continue; // if too near on this side, skip
    if (!shouldDraw(x)) continue; // also skip if is too close to a label from the other side
    drawnPositions.push(x);
    lastXRight = x;
    ctx.fillText(label, x, xAxisY);
  }
  // left - ascending hz means x decreases (center to left edge)
  let lastXLeft = Number.POSITIVE_INFINITY;
  for (const { hz, label } of visibleTicks) {
    const x = hzToBarX(hz, 0);
    if (x === null || lastXLeft - x < 20) continue;
    if (!shouldDraw(x)) continue;
    drawnPositions.push(x);
    lastXLeft = x;
    ctx.fillText(label, x, xAxisY);
  }
  ctx.restore();
}

function drawBar(ctx: CanvasRenderingContext2D, x: number, y: number, width: number, height: number) {
  if (height <= 1) return;
  ctx.fillStyle = primaryColor;
  const radius = Math.min(3, width / 2);
  if (ctx.roundRect) {
    ctx.beginPath();
    ctx.roundRect(x, y, width, height, radius);
    ctx.fill();
  } else {
    ctx.fillRect(x, y, width, height);
  }
}

function updatePeak(index: number, currentNorm: number, side: "left" | "right", frameDelta: number) {
  const peaks = side === "left" ? peaksLeft : peaksRight;
  // 0.98 per 60fps frame, scaled by elapsed time so the fall speed is frame-rate independent
  const decay = 0.98 ** (Math.min(frameDelta, 250) / VIS_FRAME_REF_MS);
  const prev = peaks.at(index) ?? 0;
  peaks[index] = Math.max(currentNorm, prev * decay);
}

function drawPeaks(ctx: CanvasRenderingContext2D) {
  const { showPeaks } = visualizerConfig;
  if (!showPeaks) return;
  const { y: drawY, h: drawH } = drawArea;
  const halfCount = peaksLeft.length;
  for (let i = 0; i < halfCount; i++) {
    const peakL = peaksLeft.at(i) ?? 0;
    const peakR = peaksRight.at(i) ?? 0;
    if (peakL > 0.01) {
      const leftBar = barPositions.at(i);
      if (leftBar) {
        const peakY = drawY + drawH - peakL * drawH;
        ctx.fillStyle = "rgba(255,255,255,0.85)"; // white peak indicators
        ctx.fillRect(leftBar.x, peakY - 1, leftBar.width, 2);
      }
    }
    if (peakR > 0.01) {
      const rightBar = barPositions.at(halfCount + i);
      if (rightBar) {
        const peakY = drawY + drawH - peakR * drawH;
        ctx.fillStyle = "rgba(255,255,255,0.85)"; // white peak indicators
        ctx.fillRect(rightBar.x, peakY - 1, rightBar.width, 2);
      }
    }
  }
}

function resizeVisualizer() {
  const canvas = visualizerCanvas.value;
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
    canvas.getContext("2d")?.setTransform(dpr, 0, 0, dpr, 0, 0);
    computeGeometry();
  }
}

watch(activeTab, (val) => {
  // Persist to localStorage
  localStorage.setItem(LAST_TAB_KEY, val);
  if (val === "visualizer") {
    if (!props.audioContext || !props.audioSource) {
      emit("needs-audio-graph");
    }
    nextTick(startVisualizer);
    return;
  }
  fullCleanup();
});

watch(visualizerConfig, (config) => applyVisualizerConfig(config), { deep: true });

watch(
  () => props.player,
  (newPlayer, oldPlayer) => {
    if (oldPlayer) {
      fullCleanup();
    }
    if (newPlayer && activeTab.value === "visualizer") {
      if (!props.audioContext || !props.audioSource) {
        emit("needs-audio-graph");
      }
      nextTick(startVisualizer);
    }
  },
  { immediate: true }
);

onMounted(() => {
  document.addEventListener("keydown", onKeyDown);
  window.addEventListener("resize", onResize);
  document.addEventListener("visibilitychange", onVisibilityChange);
  resizeObserver = new ResizeObserver(onResize);
});

onBeforeUnmount(() => {
  document.removeEventListener("keydown", onKeyDown);
  window.removeEventListener("resize", onResize);
  document.removeEventListener("visibilitychange", onVisibilityChange);
  fullCleanup();
  resizeObserver = null;
});
</script>

<style scoped>
.audio-side-panel {
  display: flex;
  flex-direction: column;
  max-height: 65vh;
  background: color-mix(in srgb, var(--alt-background) 20%, var(--background) 20%);
  border-radius: var(--borderRadius);
  overflow: hidden;
  box-shadow: var(--surfaceElevationShadow);
}

.panel-tabs {
  padding: 0.5em;
  border-bottom: 1px solid var(--divider);
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
  display: flex;
  flex-direction: column;
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

.tab-visualizer {
  position: relative;
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
  border-radius: var(--borderRadius);
  background: rgb(0 0 0 / 12%);
  display: block;
}
</style>
