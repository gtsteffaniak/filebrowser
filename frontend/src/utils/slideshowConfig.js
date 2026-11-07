import { reactive } from 'vue';

const STORAGE_KEY = 'slideshowConfig';

export const SLIDESHOW_MAX_ITEMS_HARD_CAP = 5000;

export const SLIDESHOW_DEFAULTS = {
  duration: 5,
  fade: true,
  fitToScreen: true,
  enlargeToFill: true,
  shuffle: true,
  loop: true,
  kenBurns: true,
  includeVideos: false,
  playVideosToEnd: true,
  showFilename: true,
  startFullscreen: true,
  includeSubfolders: false,
  maxItems: 100,
};

function load() {
  try {
    const stored = localStorage.getItem(STORAGE_KEY);
    if (!stored) return { ...SLIDESHOW_DEFAULTS };
    return { ...SLIDESHOW_DEFAULTS, ...JSON.parse(stored) };
  } catch (_) {
    return { ...SLIDESHOW_DEFAULTS };
  }
}

function persist() {
  try {
    localStorage.setItem(STORAGE_KEY, JSON.stringify(slideshowConfig));
  } catch (_) { /* ignore */ }
}

export const slideshowConfig = reactive(load());

export function saveSlideshowConfig(partial) {
  Object.assign(slideshowConfig, partial);
  persist();
}

export function resetSlideshowConfig() {
  Object.assign(slideshowConfig, SLIDESHOW_DEFAULTS);
  persist();
}

// Effective upper bound for subfolder search: the lower of the server's
// configured search limit and the hard cap.
export function effectiveMaxItemsCap(serverLimit) {
  const server = Number.isFinite(serverLimit) && serverLimit > 0 ? serverLimit : SLIDESHOW_MAX_ITEMS_HARD_CAP;
  return Math.min(server, SLIDESHOW_MAX_ITEMS_HARD_CAP);
}
