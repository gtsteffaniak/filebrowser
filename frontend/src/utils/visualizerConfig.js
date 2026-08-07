import { reactive } from 'vue';

const STORAGE_KEY = 'visualizerConfig';

const DEFAULTS = {
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

function load() {
  try {
    const stored = localStorage.getItem(STORAGE_KEY);
    if (!stored) return { ...DEFAULTS };
    return { ...DEFAULTS, ...JSON.parse(stored) };
  } catch (_) {
    return { ...DEFAULTS };
  }
}

function persist() {
  try {
    localStorage.setItem(STORAGE_KEY, JSON.stringify(visualizerConfig));
  } catch (_) { /* ignore */ }
}

export const visualizerConfig = reactive(load());

export function saveVisualizerConfig(partial) {
  Object.assign(visualizerConfig, partial);
  persist();
}

export function resetVisualizerConfig() {
  Object.assign(visualizerConfig, DEFAULTS);
  persist();
}
