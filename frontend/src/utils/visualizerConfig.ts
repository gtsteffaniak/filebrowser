import { reactive } from "vue";

export interface VisualizerConfig {
  /** number of bars (split between the L/R channels) */
  barCount: number;
  /** higher = smoother motion, but if set too high will look a bit slow */
  smoothing: number;
  /** FFT size (must be a power of two). Larger = better frequency but slower */
  fftSize: number;
  /** Hz - lowest frequency shown */
  minFrequency: number;
  /** Hz - highest frequency shown */
  maxFrequency: number;
  /** dBFS - the quietest level that still shows as a bar */
  minDecibels: number;
  /** dBFS - the loudest level before clipping */
  maxDecibels: number;
  /** show axis labels and grid lines */
  showScales: boolean;
  /** show peak indicators above the bars */
  showPeaks: boolean;
}

const STORAGE_KEY = "visualizerConfig";

const DEFAULTS: Readonly<VisualizerConfig> = {
  barCount: 60,
  smoothing: 0.92,
  fftSize: 8192,
  minFrequency: 20,
  maxFrequency: 20000,
  minDecibels: -70,
  maxDecibels: 0,
  showScales: true,
  showPeaks: true,
};

function sanitize(value: Partial<VisualizerConfig> | null): Partial<VisualizerConfig> {
  const valid: Partial<VisualizerConfig> = {};
  if (typeof value !== "object" || value === null) return valid;
  for (const key of Object.keys(DEFAULTS) as (keyof VisualizerConfig)[]) {
    const candidate = value[key];
    if (typeof candidate !== typeof DEFAULTS[key]) continue;
    if (typeof candidate === "number" && !Number.isFinite(candidate)) continue;
    Object.assign(valid, { [key]: candidate });
  }
  return valid;
}

function load(): VisualizerConfig {
  try {
    const stored = localStorage.getItem(STORAGE_KEY);
    if (!stored) return { ...DEFAULTS };
    return { ...DEFAULTS, ...sanitize(JSON.parse(stored)) };
  } catch {
    return { ...DEFAULTS };
  }
}

function persist() {
  try {
    localStorage.setItem(STORAGE_KEY, JSON.stringify(visualizerConfig));
  } catch {
    /* ignore */
  }
}

export const visualizerConfig: VisualizerConfig = reactive(load());

export function saveVisualizerConfig(partial: Partial<VisualizerConfig>) {
  Object.assign(visualizerConfig, sanitize(partial));
  persist();
}

export function resetVisualizerConfig() {
  Object.assign(visualizerConfig, DEFAULTS);
  persist();
}
