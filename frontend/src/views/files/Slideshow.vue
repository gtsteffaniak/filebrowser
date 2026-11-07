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
            @error="onMediaError"
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
            @error="onMediaError"
          />
        </template>
      </div>
    </div>

    <div v-if="cfg.showFilename && currentSlide" class="slideshow-filename">
      {{ currentSlide.name }}
    </div>

    <div class="slideshow-chrome slideshow-top-left">
      <button type="button" class="button button--flat slideshow-exit" :aria-label="$t('slideshow.exit')"
        :title="$t('slideshow.exit')" @click.stop="exit">
        <i class="material-symbols">{{ icons.close }}</i>
      </button>
    </div>

    <div v-if="slides.length" class="slideshow-chrome slideshow-top-right">
      {{ paused
        ? $t("slideshow.counterPaused", { current: index + 1, total: slides.length })
        : $t("slideshow.counter", { current: index + 1, total: slides.length }) }}
    </div>

    <div v-if="slides.length" class="slideshow-chrome slideshow-controls" @click.stop>
      <button type="button" class="button button--flat" :aria-label="$t('slideshow.previous')" @click="goPrev">
        <i class="material-symbols">{{ icons.previous }}</i>
      </button>
      <button type="button" class="button button--flat" :aria-label="paused ? $t('slideshow.play') : $t('slideshow.pause')" @click="togglePause">
        <i class="material-symbols">{{ paused ? icons.play : icons.pause }}</i>
      </button>
      <button type="button" class="button button--flat" :aria-label="$t('slideshow.next')" @click="goNext">
        <i class="material-symbols">{{ icons.next }}</i>
      </button>
      <button type="button" class="button button--flat" :aria-label="isFullscreen ? $t('slideshow.exitFullscreen') : $t('slideshow.fullscreen')" @click="toggleFullscreen">
        <i class="material-symbols">{{ isFullscreen ? icons.exitFullscreen : icons.fullscreen }}</i>
      </button>
    </div>

    <div v-if="loading" class="slideshow-empty">
      <LoadingSpinner size="medium" />
    </div>
    <div v-else-if="!slides.length" class="slideshow-empty">
      <p>{{ loadError || $t("slideshow.empty") }}</p>
      <button type="button" class="button" @click.stop="exit">{{ $t("slideshow.exit") }}</button>
    </div>
  </div>
</template>

<script>
import { state, getters } from "@/store";
import { router } from "@/router/index.ts";
import { ensureViewToken } from "@/api/viewToken.js";
import { buildItemUrl } from "@/utils/url.js";
import LoadingSpinner from "@/components/LoadingSpinner.vue";
import { notify } from "@/notify/index.ts";
import { slideshowConfig } from "@/utils/slideshowConfig.js";
import {
  slidesFromListing,
  slidesFromSearch,
  shuffled,
  slideUrl,
} from "@/utils/slideshow.js";

const CHROME_HIDE_MS = 3500;
const POINTER_THROTTLE_MS = 100;
const SWIPE_THRESHOLD_PX = 50;

const emptyLayer = () => ({ kind: null, url: null, key: 0, style: {} });

// Material Symbols ligature names. Kept out of the template so they aren't read as raw text.
const icons = {
  close: "close",
  previous: "skip_previous",
  next: "skip_next",
  play: "play_arrow",
  pause: "pause",
  fullscreen: "fullscreen",
  exitFullscreen: "fullscreen_exit",
};

export default {
  name: "SlideshowView",
  components: { LoadingSpinner },
  props: {
    // The directory being shown, provided by Files.vue as `fbdata`.
    fbdata: { type: Object, required: true },
  },
  data() {
    return {
      icons,
      slides: [],
      index: 0,
      loading: true,
      loadError: "",
      token: null,
      layers: [emptyLayer(), emptyLayer()],
      active: 0,
      layerKey: 0,
      paused: false,
      remaining: null,
      chromeVisible: true,
      isFullscreen: false,
      failStreak: 0,
      wakeLock: null,
      touchX: null,
      lastPointer: 0,
      timerHandle: null,
      timerStart: 0,
      hideHandle: null,
      advanceHandle: null,
      videoEls: {},
      keyHandler: null,
      fullscreenHandler: null,
      visibilityHandler: null,
    };
  },
  computed: {
    cfg() {
      return slideshowConfig;
    },
    sidebarShowing() {
      return getters.isSidebarVisible();
    },
    // Only a pinned (sticky) sidebar takes up layout room. A floating sidebar overlays the slideshow.
    moveWithSidebar() {
      return this.sidebarShowing && getters.isStickySidebar();
    },
    sidebarWidth() {
      return state.sidebar?.width || 20;
    },
    currentSlide() {
      return this.slides[this.index] || null;
    },
  },
  async mounted() {
    this.keyHandler = (e) => this.onKeyDown(e);
    this.fullscreenHandler = () => {
      this.isFullscreen = !!(document.fullscreenElement || document.webkitFullscreenElement);
    };
    this.visibilityHandler = () => {
      if (document.visibilityState === "visible" && !this.paused) this.requestWakeLock();
    };
    window.addEventListener("keydown", this.keyHandler);
    document.addEventListener("fullscreenchange", this.fullscreenHandler);
    document.addEventListener("webkitfullscreenchange", this.fullscreenHandler);
    document.addEventListener("visibilitychange", this.visibilityHandler);

    await this.loadSlides();
    if (this.slides.length) {
      this.showSlide(this.index, false);
      this.scheduleCurrent();
      this.requestWakeLock();
      if (this.cfg.startFullscreen) this.enterFullscreen();
      this.scheduleHide();
    }
  },
  beforeUnmount() {
    window.removeEventListener("keydown", this.keyHandler);
    document.removeEventListener("fullscreenchange", this.fullscreenHandler);
    document.removeEventListener("webkitfullscreenchange", this.fullscreenHandler);
    document.removeEventListener("visibilitychange", this.visibilityHandler);
    this.clearTimer();
    clearTimeout(this.hideHandle);
    clearTimeout(this.advanceHandle);
    this.releaseWakeLock();
    if (document.fullscreenElement || document.webkitFullscreenElement) {
      this.exitFullscreen();
    }
  },
  methods: {
    async loadSlides() {
      this.loading = true;
      const { source, path, items } = this.fbdata;
      try {
        this.token = await ensureViewToken(source);
      } catch (_) {
        this.token = null;
      }

      let slides = null;
      if (this.cfg.includeSubfolders && !getters.isShare()) {
        try {
          const result = await slidesFromSearch(source, path, this.cfg.maxItems);
          slides = result.slides;
          if (result.truncated) {
            notify.showSuccessToast(this.$t("slideshow.truncated", { max: this.cfg.maxItems }));
          }
        } catch (_) {
          notify.showErrorToast(this.$t("slideshow.searchFailed"));
        }
      }
      if (!slides) {
        slides = slidesFromListing(items, source);
      }

      if (this.cfg.shuffle) slides = shuffled(slides);

      const startName = this.$route?.query?.start ?? state.route?.query?.start ?? "";
      let start = startName ? slides.findIndex((s) => s.name === startName) : -1;
      if (this.cfg.shuffle && start > 0) {
        const [first] = slides.splice(start, 1);
        slides.unshift(first);
        start = 0;
      }

      this.slides = slides;
      this.index = Math.max(0, start);
      this.loading = false;
      if (!slides.length) {
        this.loadError = this.$t("slideshow.empty");
      }
    },

    setVideoRef(el, layerIndex) {
      if (el) this.videoEls[layerIndex] = el;
    },

    randomKenBurns() {
      const rand = (min, max) => min + Math.random() * (max - min);
      return {
        "--kb-x": `${rand(-5, 5).toFixed(2)}%`,
        "--kb-y": `${rand(-5, 5).toFixed(2)}%`,
        "--kb-scale": rand(1.05, 1.17).toFixed(3),
        "--kb-duration": `${this.cfg.duration}s`,
      };
    },

    showSlide(i, animate) {
      this.index = i;
      const slide = this.slides[i];
      if (!slide) return;

      const target = animate ? 1 - this.active : this.active;
      this.layerKey += 1;
      this.layers[target] = {
        kind: slide.kind,
        url: slideUrl(slide, this.token),
        key: this.layerKey,
        style: slide.kind === "image" && this.cfg.kenBurns ? this.randomKenBurns() : {},
      };
      this.active = target;

      this.$nextTick(() => {
        Object.entries(this.videoEls).forEach(([layer, el]) => {
          if (el && Number(layer) !== this.active) el.pause();
        });
        const video = this.videoEls[this.active];
        if (video) {
          video.currentTime = 0;
          if (!this.paused) video.play().catch(() => {});
        }
      });

      this.preloadNext();
    },

    preloadNext() {
      const nextIndex = this.index + 1 < this.slides.length ? this.index + 1 : this.cfg.loop ? 0 : -1;
      const next = nextIndex >= 0 ? this.slides[nextIndex] : null;
      if (next && next.kind === "image") {
        const url = slideUrl(next, this.token);
        if (url) new Image().src = url;
      }
    },

    clearTimer() {
      clearTimeout(this.timerHandle);
      this.timerHandle = null;
    },

    startTimer(ms) {
      this.clearTimer();
      this.timerStart = Date.now();
      this.timerHandle = setTimeout(() => this.advance(), ms);
    },

    // Videos play to their natural end when the option is on. Everything else uses the slide duration.
    scheduleCurrent() {
      this.clearTimer();
      const slide = this.currentSlide;
      if (!slide) return;
      if (slide.kind === "video" && this.cfg.playVideosToEnd) {
        this.remaining = null;
        return;
      }
      this.remaining = this.cfg.duration * 1000;
      if (!this.paused) this.startTimer(this.remaining);
    },

    activeVideo() {
      return this.videoEls[this.active] || null;
    },

    advance() {
      const last = this.slides.length - 1;
      if (this.index < last) {
        this.showSlide(this.index + 1, true);
      } else if (this.cfg.loop) {
        this.showSlide(0, true);
      } else {
        // Stop on the last slide when loop is off.
        this.pause();
        return;
      }
      this.scheduleCurrent();
    },

    goNext() {
      if (!this.slides.length) return;
      this.advance();
      this.revealChrome();
    },

    goPrev() {
      if (!this.slides.length) return;
      const last = this.slides.length - 1;
      if (this.index > 0) {
        this.showSlide(this.index - 1, true);
      } else if (this.cfg.loop) {
        this.showSlide(last, true);
      }
      this.scheduleCurrent();
      this.revealChrome();
    },

    togglePause() {
      if (this.paused) this.resume();
      else this.pause();
      this.revealChrome();
    },

    pause() {
      if (this.timerHandle) {
        const elapsed = Date.now() - this.timerStart;
        this.remaining = Math.max(0, (this.remaining ?? 0) - elapsed);
        this.clearTimer();
      }
      this.paused = true;
      this.activeVideo()?.pause();
      this.releaseWakeLock();
      this.revealChrome();
    },

    resume() {
      this.paused = false;
      const slide = this.currentSlide;
      if (slide?.kind === "video" && this.cfg.playVideosToEnd) {
        this.activeVideo()?.play().catch(() => {});
      } else if (this.remaining !== null) {
        this.startTimer(this.remaining);
      }
      this.requestWakeLock();
      this.scheduleHide();
    },

    onVideoEnded() {
      if (!this.paused && this.currentSlide?.kind === "video" && this.cfg.playVideosToEnd) {
        this.advance();
      }
    },

    onMediaLoaded() {
      this.failStreak = 0;
    },

    // Skip media that fails to load. Stop once every slide has failed in a row.
    onMediaError() {
      this.failStreak += 1;
      if (this.failStreak >= this.slides.length) {
        this.loadError = this.$t("slideshow.allFailed");
        this.slides = [];
        this.pause();
        return;
      }
      if (!this.paused) {
        clearTimeout(this.advanceHandle);
        this.advanceHandle = setTimeout(() => {
          if (!this.paused) this.advance();
        }, 1000);
      }
    },

    onStageClick() {
      this.togglePause();
    },

    revealChrome() {
      this.chromeVisible = true;
      this.scheduleHide();
    },

    scheduleHide() {
      clearTimeout(this.hideHandle);
      if (this.paused) return;
      this.hideHandle = setTimeout(() => {
        this.chromeVisible = false;
      }, CHROME_HIDE_MS);
    },

    onPointerMove() {
      const now = Date.now();
      if (now - this.lastPointer < POINTER_THROTTLE_MS) return;
      this.lastPointer = now;
      this.revealChrome();
    },

    onTouchStart(e) {
      this.touchX = e.touches[0]?.clientX ?? null;
      this.revealChrome();
    },

    onTouchEnd(e) {
      if (this.touchX === null) return;
      const dx = (e.changedTouches[0]?.clientX ?? this.touchX) - this.touchX;
      this.touchX = null;
      if (Math.abs(dx) < SWIPE_THRESHOLD_PX) return;
      if (dx < 0) this.goNext();
      else this.goPrev();
    },

    onKeyDown(e) {
      if (getters.currentPromptName?.()) return;
      switch (e.key) {
        case " ":
          e.preventDefault();
          this.togglePause();
          break;
        case "ArrowRight":
          e.preventDefault();
          this.goNext();
          break;
        case "ArrowLeft":
          e.preventDefault();
          this.goPrev();
          break;
        case "f":
        case "F":
          this.toggleFullscreen();
          break;
        case "Escape":
          // The browser exits fullscreen on Escape by itself, so only leave the slideshow when not fullscreen.
          if (!(document.fullscreenElement || document.webkitFullscreenElement)) this.exit();
          break;
      }
    },

    enterFullscreen() {
      const el = this.$refs.root;
      if (!el) return;
      const request = el.requestFullscreen || el.webkitRequestFullscreen;
      if (!request) return;
      try {
        const result = request.call(el);
        if (result && typeof result.catch === "function") result.catch(() => {});
      } catch (_) { /* not allowed in this context */ }
    },

    exitFullscreen() {
      const exit = document.exitFullscreen || document.webkitExitFullscreen;
      if (!exit) return;
      try {
        const result = exit.call(document);
        if (result && typeof result.catch === "function") result.catch(() => {});
      } catch (_) { /* ignore */ }
    },

    toggleFullscreen() {
      if (document.fullscreenElement || document.webkitFullscreenElement) this.exitFullscreen();
      else this.enterFullscreen();
    },

    async requestWakeLock() {
      if (this.wakeLock || !navigator.wakeLock) return;
      try {
        this.wakeLock = await navigator.wakeLock.request("screen");
      } catch (_) { /* not available or denied */ }
    },

    releaseWakeLock() {
      this.wakeLock?.release?.();
      this.wakeLock = null;
    },

    exit() {
      this.pause();
      if (document.fullscreenElement || document.webkitFullscreenElement) this.exitFullscreen();
      // Return to the preview of the slide that was showing last. Fall back to the folder if there is none.
      // Replace the slideshow entry so Back from that preview doesn't return here.
      const slide = this.currentSlide;
      const target = slide ? buildItemUrl(slide.source, slide.path) : state.route.path;
      router.replace({ path: target });
    },
  },
};
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

.slideshow-top-left {
  position: absolute;
  top: 1em;
  left: 1em;
}

.slideshow-top-right {
  position: absolute;
  top: 1em;
  right: 1em;
  padding: 0.4em 0.8em;
  border-radius: var(--borderRadius);
  background: rgb(0 0 0 / 55%);
}

/* Round, icon-only floating button, matching the FloatingActionButton style. */
.slideshow-exit {
  width: 2.75em;
  height: 2.75em;
  padding: 0;
  border-radius: 50%;
  background: rgb(0 0 0 / 55%);
  box-shadow: 0 2px 8px rgb(0 0 0 / 40%);
  display: inline-flex;
  align-items: center;
  justify-content: center;
}

/* Outside fullscreen the app navbar is visible, so the top buttons sit below it with extra padding. */
.slideshow.below-navbar .slideshow-top-left {
  top: calc(var(--header-height) + 1em);
  left: 1.5em;
}

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
