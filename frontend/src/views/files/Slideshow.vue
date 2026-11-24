<template>
  <div
    ref="root"
    class="slideshow"
    :class="{
      'chrome-visible': chromeVisible,
      'cursor-hidden': !chromeVisible && !paused,
      'is-paused': paused,
      'no-transition': !cfg.fade,
      'fit-contain': cfg.fitToScreen,
      'fit-cover': !cfg.fitToScreen,
      'no-enlarge': !cfg.enlargeToFill,
      'below-navbar': !isFullscreen,
    }"
    :style="{ left: moveWithSidebar ? `${sidebarWidth}em` : '0' }"
    @mousemove="onPointerMove"
    @touchstart.passive="onTouchStart"
    @touchend="onTouchEnd"
    @click="onStageClick"
  >
    <div class="slideshow-stage">
      <div
        v-for="(layer, i) in layers"
        :key="i"
        class="slide-layer"
        :class="{ active: active === i }"
      >
        <template v-if="layer.url">
          <img
            v-if="layer.kind === 'image'"
            :key="layer.key"
            :src="layer.url"
            :class="{ kb: cfg.kenBurns }"
            :style="layer.style"
            alt=""
            @load="onMediaLoaded"
            @error="onMediaError($event)"
          />
          <video
            v-else
            :key="layer.key"
            :ref="(el) => setVideoRef(el, i)"
            :src="layer.url"
            muted
            playsinline
            preload="auto"
            @ended="onVideoEnded"
            @loadeddata="onMediaLoaded"
            @error="onMediaError($event)"
          />
        </template>
      </div>
    </div>

    <div v-if="cfg.showFilename && currentSlide" class="slideshow-filename">
      {{ currentSlide.name }}
    </div>

    <div v-if="slides.length" class="slideshow-chrome slideshow-top-right">
      {{ `${index + 1}/${slides.length}${paused ? ` · ${t("general.paused", { suffix: "" })}` : ""}` }}
    </div>

    <div v-if="slides.length" class="slideshow-chrome slideshow-controls" @click.stop>
      <button type="button" class="button button--flat" :aria-label="t('general.exit', { suffix: '' })"
        :title="t('general.exit', { suffix: '' })" @click.stop="exit">
        <i class="material-symbols">{{ icons.close }}</i>
      </button>
      <button type="button" class="button button--flat" :aria-label="t('general.previous', { suffix: '' })" @click="goPrev">
        <i class="material-symbols">{{ icons.previous }}</i>
      </button>
      <button type="button" class="button button--flat" :aria-label="paused ? t('general.resume', { suffix: '' }) : t('general.pause', { suffix: '' })" @click="togglePause">
        <i class="material-symbols">{{ paused ? icons.play : icons.pause }}</i>
      </button>
      <button type="button" class="button button--flat" :aria-label="t('general.next', { suffix: '' })" @click="goNext">
        <i class="material-symbols">{{ icons.next }}</i>
      </button>
      <button type="button" class="button button--flat" :aria-label="isFullscreen ? t('player.exitFullscreen') : t('player.fullscreen')" @click="toggleFullscreen">
        <i class="material-symbols">{{ isFullscreen ? icons.exitFullscreen : icons.fullscreen }}</i>
      </button>
    </div>

    <div v-if="loading" class="slideshow-empty">
      <LoadingSpinner size="medium" />
    </div>
    <div v-else-if="!slides.length" class="slideshow-empty">
      <p>{{ loadError || t("slideshow.empty") }}</p>
      <button type="button" class="button" @click.stop="exit">{{ t("general.exit", { suffix: "" }) }}</button>
    </div>
  </div>
</template>

<script setup lang="ts">
import {
  computed,
  nextTick,
  onBeforeUnmount,
  onMounted,
  ref,
  useTemplateRef,
  watch,
  type ComponentPublicInstance,
} from "vue";
import { useRoute } from "vue-router";
import { useI18n } from "@/i18n/index.ts";
import { state, getters } from "@/store/index.ts";
import type { FileListItem } from "@/store/types.ts";
import { router } from "@/router/index.ts";
import { ensureViewToken } from "@/api/viewToken.js";
import { buildItemUrl } from "@/utils/url.js";
import LoadingSpinner from "@/components/LoadingSpinner.vue";
import { notify } from "@/notify/index.ts";
import { slideshowConfig } from "@/utils/slideshowConfig.ts";
import {
  slidesFromListing,
  slidesFromSearch,
  shuffled,
  slideUrl,
  type SlideshowMediaKind,
  type SlideshowSlide,
} from "@/utils/slideshow.ts";

/** Directory context passed from Files.vue as `fbdata` (state.req). */
const props = defineProps<{
  fbdata: {
    source: string;
    path: string;
    items: FileListItem[];
  };
}>();

defineOptions({ name: "SlideshowView" });

const { t } = useI18n();
const route = useRoute();

const CHROME_HIDE_MS = 3500;
const POINTER_THROTTLE_MS = 100;
const SWIPE_THRESHOLD_PX = 50;

type SlideLayer = {
  kind: SlideshowMediaKind | null;
  url: string | null;
  key: number;
  style: Record<string, string>;
};

const emptyLayer = (): SlideLayer => ({ kind: null, url: null, key: 0, style: {} });

// Material Symbols ligature names. Kept out of the template so they aren't read as raw text.
const icons = {
  close: "close",
  previous: "skip_previous",
  next: "skip_next",
  play: "play_arrow",
  pause: "pause",
  fullscreen: "fullscreen",
  exitFullscreen: "fullscreen_exit",
} as const;

const cfg = slideshowConfig;

const root = useTemplateRef<HTMLElement>("root");
const slides = ref<SlideshowSlide[]>([]);
const index = ref(0);
const loading = ref(true);
const loadError = ref("");
const token = ref<string | null>(null);
const layers = ref<SlideLayer[]>([emptyLayer(), emptyLayer()]);
const active = ref(0);
const layerKey = ref(0);
const paused = ref(false);
const remaining = ref<number | null>(null);
const chromeVisible = ref(true);
const isFullscreen = ref(false);
const failStreak = ref(0);
const wakeLock = ref<WakeLockSentinel | null>(null);
const touchX = ref<number | null>(null);
const lastPointer = ref(0);

let timerHandle: ReturnType<typeof setTimeout> | null = null;
let timerStart = 0;
let hideHandle: ReturnType<typeof setTimeout> | null = null;
let advanceHandle: ReturnType<typeof setTimeout> | null = null;
let wakeLockGeneration = 0;
let loadGeneration = 0;
let disposed = false;
const videoEls = new Map<number, HTMLVideoElement>();

const sidebarShowing = computed(() => getters.isSidebarVisible());
// Only a pinned (sticky) sidebar takes up layout room. A floating sidebar overlays the slideshow.
const moveWithSidebar = computed(() => sidebarShowing.value && getters.isStickySidebar());
const sidebarWidth = computed(() => state.sidebar?.width || 20);
const currentSlide = computed(() => slides.value[index.value] ?? null);

function fullscreenActive(): boolean {
  const doc = document as Document & { webkitFullscreenElement?: Element | null };
  return !!(document.fullscreenElement || doc.webkitFullscreenElement);
}

function needsViewToken(slide: SlideshowSlide): boolean {
  return !(slide.kind === "image" && slide.preview && !getters.isShare());
}

function clearAdvanceHandle() {
  if (advanceHandle !== null) clearTimeout(advanceHandle);
  advanceHandle = null;
}

async function loadSlides() {
  const gen = ++loadGeneration;
  loading.value = true;
  loadError.value = "";
  const { source, path, items } = props.fbdata;
  let viewToken: string | null = null;
  try {
    viewToken = await ensureViewToken(source);
  } catch {
    viewToken = null;
  }

  let loaded: SlideshowSlide[] | null = null;
  if (cfg.includeSubfolders && !getters.isShare()) {
    try {
      const result = await slidesFromSearch(source, path, cfg.maxItems);
      loaded = result.slides;
      if (result.truncated) {
        notify.showSuccessToast(t("slideshow.truncated", { max: cfg.maxItems }));
      }
    } catch {
      notify.showErrorToast(t("slideshow.searchFailed"));
    }
  }
  if (!loaded) {
    loaded = slidesFromListing(items, source);
  }

  if (gen !== loadGeneration || disposed) {
    loading.value = false;
    return;
  }

  if (!viewToken && loaded.some(needsViewToken)) {
    loadError.value = t("slideshow.tokenFailed");
    slides.value = [];
    loading.value = false;
    return;
  }

  let nextSlides = cfg.shuffle ? shuffled(loaded) : loaded;

  const queryStart = (query: unknown): string => {
    if (!query || typeof query !== "object") return "";
    const start = (query as Record<string, unknown>).start;
    return typeof start === "string" ? start : "";
  };
  const startName = queryStart(route.query) || queryStart(state.route?.query);
  let start = startName ? nextSlides.findIndex((s) => s.name === startName) : -1;
  if (cfg.shuffle && start > 0) {
    const picked = nextSlides[start];
    if (picked) {
      nextSlides = [...nextSlides.slice(0, start), ...nextSlides.slice(start + 1)];
      nextSlides.unshift(picked);
      start = 0;
    }
  }

  if (gen !== loadGeneration || disposed) {
    loading.value = false;
    return;
  }

  token.value = viewToken;
  slides.value = nextSlides;
  index.value = Math.max(0, start);
  loading.value = false;
  if (!nextSlides.length) {
    loadError.value = t("slideshow.empty");
  }
}

async function reloadFromContext() {
  clearAdvanceHandle();
  clearTimer();
  remaining.value = null;
  failStreak.value = 0;
  releaseWakeLock();
  await loadSlides();
  if (disposed) return;
  if (slides.value.length) {
    paused.value = false;
    await beginSlideshow();
  }
}

function setVideoRef(el: Element | ComponentPublicInstance | null, layerIndex: number) {
  if (el instanceof HTMLVideoElement) {
    videoEls.set(layerIndex, el);
    return;
  }
  videoEls.delete(layerIndex);
}

function randomKenBurns(): Record<string, string> {
  const rand = (min: number, max: number) => min + Math.random() * (max - min);
  return {
    "--kb-x": `${rand(-5, 5).toFixed(2)}%`,
    "--kb-y": `${rand(-5, 5).toFixed(2)}%`,
    "--kb-scale": rand(1.05, 1.17).toFixed(3),
    "--kb-duration": `${cfg.duration}s`,
  };
}

async function showSlide(i: number, animate: boolean) {
  clearAdvanceHandle();
  clearTimer();
  remaining.value = null;
  index.value = i;
  const slide = slides.value[i];
  if (!slide) return;

  const target = animate ? 1 - active.value : active.value;
  layerKey.value += 1;
  const nextLayers = [...layers.value];
  nextLayers[target] = {
    kind: slide.kind,
    url: slideUrl(slide, token.value),
    key: layerKey.value,
    style: slide.kind === "image" && cfg.kenBurns ? randomKenBurns() : {},
  };
  layers.value = nextLayers;
  active.value = target;

  await nextTick();
  for (const [layer, el] of videoEls) {
    if (layer !== active.value) el.pause();
  }
  const video = videoEls.get(active.value);
  if (video && slide.kind === "video") {
    video.currentTime = 0;
    if (!paused.value) video.play().catch(() => {});
  }

  preloadNext();
}

function preloadNext() {
  const nextIndex =
    index.value + 1 < slides.value.length ? index.value + 1 : cfg.loop ? 0 : -1;
  const next = nextIndex >= 0 ? slides.value[nextIndex] : null;
  if (next?.kind === "image") {
    const url = slideUrl(next, token.value);
    if (url) new Image().src = url;
  }
}

function clearTimer() {
  if (timerHandle !== null) clearTimeout(timerHandle);
  timerHandle = null;
}

function startTimer(ms: number) {
  clearTimer();
  timerStart = Date.now();
  timerHandle = setTimeout(() => advance(), ms);
}

function scheduleCurrent() {
  clearTimer();
  const slide = currentSlide.value;
  if (!slide) return;
  if (slide.kind === "video" && cfg.playVideosToEnd) {
    remaining.value = null;
    return;
  }
  remaining.value = cfg.duration * 1000;
  if (!paused.value) startTimer(remaining.value);
}

function activeVideo(): HTMLVideoElement | null {
  return videoEls.get(active.value) ?? null;
}

function advance() {
  const last = slides.value.length - 1;
  if (index.value < last) {
    showSlide(index.value + 1, true);
  } else if (cfg.loop) {
    showSlide(0, true);
  } else {
    pause();
    return;
  }
}

function goNext() {
  if (!slides.value.length) return;
  advance();
  revealChrome();
}

function goPrev() {
  if (!slides.value.length) return;
  const last = slides.value.length - 1;
  if (index.value > 0) {
    showSlide(index.value - 1, true);
  } else if (cfg.loop) {
    showSlide(last, true);
  }
  revealChrome();
}

function togglePause() {
  if (paused.value) resume();
  else pause();
  revealChrome();
}

function pause() {
  if (timerHandle !== null) {
    const elapsed = Date.now() - timerStart;
    remaining.value = Math.max(0, (remaining.value ?? 0) - elapsed);
    clearTimer();
  }
  paused.value = true;
  activeVideo()?.pause();
  releaseWakeLock();
  revealChrome();
}

function resume() {
  paused.value = false;
  const slide = currentSlide.value;
  if (slide?.kind === "video") {
    activeVideo()?.play().catch(() => {});
  }
  if (!(slide?.kind === "video" && cfg.playVideosToEnd) && remaining.value !== null) {
    startTimer(remaining.value);
  }
  requestWakeLock();
  scheduleHide();
}

function onVideoEnded() {
  if (!paused.value && currentSlide.value?.kind === "video" && cfg.playVideosToEnd) {
    advance();
  }
}

function onMediaLoaded(event: Event) {
  const el = event.target as HTMLElement;
  if (!el.closest(".slide-layer.active")) return;
  failStreak.value = 0;
  scheduleCurrent();
}

function onMediaError(event: Event) {
  const el = event.target as HTMLElement;
  if (!el.closest(".slide-layer.active")) return;
  failStreak.value += 1;
  if (failStreak.value >= slides.value.length) {
    loadError.value = t("slideshow.allFailed");
    slides.value = [];
    pause();
    return;
  }
  if (!paused.value) {
    const failedIndex = index.value;
    clearAdvanceHandle();
    advanceHandle = setTimeout(() => {
      if (!paused.value && index.value === failedIndex) advance();
    }, 1000);
  }
}

function onStageClick() {
  togglePause();
}

function revealChrome() {
  chromeVisible.value = true;
  scheduleHide();
}

function scheduleHide() {
  if (hideHandle !== null) clearTimeout(hideHandle);
  if (paused.value) return;
  hideHandle = setTimeout(() => {
    chromeVisible.value = false;
  }, CHROME_HIDE_MS);
}

function onPointerMove() {
  const now = Date.now();
  if (now - lastPointer.value < POINTER_THROTTLE_MS) return;
  lastPointer.value = now;
  revealChrome();
}

function onTouchStart(e: TouchEvent) {
  touchX.value = e.touches[0]?.clientX ?? null;
  revealChrome();
}

function onTouchEnd(e: TouchEvent) {
  if (touchX.value === null) return;
  const dx = (e.changedTouches[0]?.clientX ?? touchX.value) - touchX.value;
  touchX.value = null;
  if (Math.abs(dx) < SWIPE_THRESHOLD_PX) return;
  if (dx < 0) goNext();
  else goPrev();
}

function onKeyDown(e: KeyboardEvent) {
  if (getters.currentPromptName()) return;
  switch (e.key) {
    case " ":
      e.preventDefault();
      togglePause();
      break;
    case "ArrowRight":
      e.preventDefault();
      goNext();
      break;
    case "ArrowLeft":
      e.preventDefault();
      goPrev();
      break;
    case "f":
    case "F":
      toggleFullscreen();
      break;
    case "Escape":
      if (!fullscreenActive()) exit();
      break;
  }
}

function enterFullscreen() {
  const el = root.value as (HTMLElement & { webkitRequestFullscreen?: () => void | Promise<void> }) | null;
  if (!el) return;
  const request = el.requestFullscreen?.bind(el) ?? el.webkitRequestFullscreen?.bind(el);
  if (!request) return;
  try {
    const result = request();
    if (result && typeof (result as Promise<void>).catch === "function") {
      (result as Promise<void>).catch(() => {});
    }
  } catch {
    /* not allowed in this context */
  }
}

function exitFullscreen() {
  const doc = document as Document & { webkitExitFullscreen?: () => void | Promise<void> };
  const requestExit =
    document.exitFullscreen?.bind(document) ?? doc.webkitExitFullscreen?.bind(document);
  if (!requestExit) return;
  try {
    const result = requestExit();
    if (result && typeof (result as Promise<void>).catch === "function") {
      (result as Promise<void>).catch(() => {});
    }
  } catch {
    /* ignore */
  }
}

function toggleFullscreen() {
  if (fullscreenActive()) exitFullscreen();
  else enterFullscreen();
}

async function requestWakeLock() {
  if (wakeLock.value || !navigator.wakeLock) return;
  const generation = wakeLockGeneration;
  try {
    const sentinel = await navigator.wakeLock.request("screen");
    if (generation !== wakeLockGeneration || paused.value || wakeLock.value) {
      sentinel.release().catch(() => {});
    } else {
      wakeLock.value = sentinel;
    }
  } catch {
    /* not available or denied */
  }
}

function releaseWakeLock() {
  wakeLockGeneration++;
  wakeLock.value?.release?.();
  wakeLock.value = null;
}

function exit() {
  pause();
  if (fullscreenActive()) exitFullscreen();
  const slide = currentSlide.value;
  const target = slide ? buildItemUrl(slide.source, slide.path) : state.route.path;
  router.replace({ path: target });
}

const keyHandler = (e: KeyboardEvent) => onKeyDown(e);
const fullscreenHandler = () => {
  isFullscreen.value = fullscreenActive();
};
const visibilityHandler = () => {
  if (document.visibilityState === "visible" && !paused.value) requestWakeLock();
};

async function beginSlideshow() {
  if (disposed || !slides.value.length) return;
  await showSlide(index.value, false);
  requestWakeLock();
  if (cfg.startFullscreen) enterFullscreen();
  scheduleHide();
}

watch(
  () =>
    [
      props.fbdata.source,
      props.fbdata.path,
      props.fbdata.items?.length ?? 0,
    ] as const,
  (current, previous) => {
    if (!previous) return;
    const [source, path, count] = current;
    const [prevSource, prevPath, prevCount] = previous;
    if (source === prevSource && path === prevPath && count === prevCount) return;
    void reloadFromContext();
  },
);

onMounted(async () => {
  window.addEventListener("keydown", keyHandler);
  document.addEventListener("fullscreenchange", fullscreenHandler);
  document.addEventListener("webkitfullscreenchange", fullscreenHandler);
  document.addEventListener("visibilitychange", visibilityHandler);

  await loadSlides();
  if (disposed) return;
  if (slides.value.length) {
    await beginSlideshow();
  }
});

onBeforeUnmount(() => {
  disposed = true;
  loadGeneration++;
  window.removeEventListener("keydown", keyHandler);
  document.removeEventListener("fullscreenchange", fullscreenHandler);
  document.removeEventListener("webkitfullscreenchange", fullscreenHandler);
  document.removeEventListener("visibilitychange", visibilityHandler);
  clearTimer();
  if (hideHandle !== null) clearTimeout(hideHandle);
  clearAdvanceHandle();
  releaseWakeLock();
  if (fullscreenActive()) exitFullscreen();
});
</script>

<style scoped>
.slideshow {
  position: fixed;
  top: 0;
  right: 0;
  bottom: 0;
  /* left is set inline so the stage makes room for a pinned sidebar. */
  z-index: 2000;
  background: #000;
  color: #fff;
  overflow: hidden;
  user-select: none;
}

.slideshow.cursor-hidden {
  cursor: none;
}

.slideshow-stage {
  position: absolute;
  inset: 0;
}

.slide-layer {
  position: absolute;
  inset: 0;
  opacity: 0;
  transition: opacity 1.2s ease-in-out;
  display: flex;
  align-items: center;
  justify-content: center;
}

.slide-layer.active {
  opacity: 1;
  z-index: 1;
}

.slideshow.no-transition .slide-layer {
  transition: none;
}

.slide-layer img,
.slide-layer video {
  width: 100%;
  height: 100%;
  object-fit: contain;
}

.slideshow.fit-cover .slide-layer img,
.slideshow.fit-cover .slide-layer video {
  object-fit: cover;
}

/* Never display media larger than its natural size. Cover and fill only apply when enlarging. */
.slideshow.no-enlarge .slide-layer img,
.slideshow.no-enlarge .slide-layer video {
  width: auto;
  height: auto;
  max-width: 100%;
  max-height: 100%;
  object-fit: contain;
}

/* Ken Burns: one shared keyframe driven by per-slide custom properties. */
.slide-layer img.kb {
  animation: slideshow-kb var(--kb-duration, 5s) ease-in-out forwards;
}

.slideshow.is-paused .slide-layer img.kb {
  animation-play-state: paused;
}

@keyframes slideshow-kb {
  from {
    transform: scale(1) translate(0, 0);
  }
  to {
    transform: scale(var(--kb-scale, 1.1)) translate(var(--kb-x, 0), var(--kb-y, 0));
  }
}

.slideshow-chrome {
  opacity: 0;
  transition: opacity 0.4s;
  z-index: 2;
}

.slideshow.chrome-visible .slideshow-chrome,
.slideshow.is-paused .slideshow-chrome {
  opacity: 1;
}

.slideshow-top-right {
  position: absolute;
  top: 1em;
  right: 1em;
  padding: 0.4em 0.8em;
  border-radius: var(--borderRadius);
  background: rgb(0 0 0 / 55%);
}

/* Outside fullscreen the app navbar is visible, so the top counter sits below it with extra padding. */
.slideshow.below-navbar .slideshow-top-right {
  top: calc(var(--header-height) + 1em);
  right: 1.5em;
}

.slideshow-controls {
  position: absolute;
  bottom: 1.5em;
  left: 50%;
  transform: translateX(-50%);
  display: flex;
  gap: 0.4em;
  padding: 0.4em;
  border-radius: var(--borderRadius);
  background: rgb(0 0 0 / 55%);
  backdrop-filter: blur(8px);
}

.slideshow .button {
  color: #fff;
}

.slideshow-filename {
  position: absolute;
  bottom: 6em;
  left: 0;
  right: 0;
  font-size: 1.5rem;
  font-weight: 600;
  text-align: center;
  text-shadow: 0 1px 4px rgb(0 0 0 / 90%);
  z-index: 2;
  pointer-events: none;
}

.slideshow-empty {
  position: absolute;
  inset: 0;
  display: flex;
  flex-direction: column;
  align-items: center;
  justify-content: center;
  gap: 1em;
  z-index: 3;
}
</style>
