import { reactive } from "vue";

const STORAGE_KEY = "slideshowConfig";

export const SLIDESHOW_MAX_ITEMS_HARD_CAP = 5000;

export interface SlideshowConfig {
  duration: number;
  fade: boolean;
  fitToScreen: boolean;
  enlargeToFill: boolean;
  shuffle: boolean;
  loop: boolean;
  kenBurns: boolean;
  includeVideos: boolean;
  playVideosToEnd: boolean;
  showFilename: boolean;
  startFullscreen: boolean;
  includeSubfolders: boolean;
  maxItems: number;
}

export const SLIDESHOW_DEFAULTS: SlideshowConfig = {
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

function load(): SlideshowConfig {
  try {
    const stored = localStorage.getItem(STORAGE_KEY);
    if (!stored) return { ...SLIDESHOW_DEFAULTS };
    return { ...SLIDESHOW_DEFAULTS, ...JSON.parse(stored) };
  } catch {
    return { ...SLIDESHOW_DEFAULTS };
  }
}

function persist(): void {
  try {
    localStorage.setItem(STORAGE_KEY, JSON.stringify(slideshowConfig));
  } catch {
    /* ignore */
  }
}

export const slideshowConfig = reactive(load());

export function saveSlideshowConfig(partial: Partial<SlideshowConfig>): void {
  Object.assign(slideshowConfig, partial);
  persist();
}

export function resetSlideshowConfig(): void {
  Object.assign(slideshowConfig, SLIDESHOW_DEFAULTS);
  persist();
}

/** Effective upper bound for subfolder search: the lower of the server limit and the hard cap. */
export function effectiveMaxItemsCap(serverLimit?: number): number {
  const server =
    typeof serverLimit === "number" && Number.isFinite(serverLimit) && serverLimit > 0
      ? serverLimit
      : SLIDESHOW_MAX_ITEMS_HARD_CAP;
  return Math.min(server, SLIDESHOW_MAX_ITEMS_HARD_CAP);
}