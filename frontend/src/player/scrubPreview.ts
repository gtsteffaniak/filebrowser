import type Plyr from "plyr";
import { errorName, type PlayerError } from "@/player/playerErrors.ts";
import { getters } from "@/store/index.ts";
import { fetchPreviewImage } from "@/utils/previewRequests.ts";

const SCRUB_PREVIEW_CLASS = "fb-scrub-preview";
const SCRUB_PREVIEW_VISIBLE_CLASS = "fb-scrub-preview--visible";
const DEFAULT_MIN_INTERVAL_MS = 500;
const DEFAULT_PREVIEW_WIDTH_PX = 600;
const DEFAULT_MAX_PREVIEW_HEIGHT_PX = 600;
const DEFAULT_PLACEHOLDER_ASPECT = 16 / 9;
const DEFAULT_SCRUB_PERCENT_STEP = 1;
const MAX_SCRUB_FETCH_PERCENT = 99; // 99 to never request a preview at the exact end of the media, it was causing an error.
const MIN_SCRUB_FETCH_PERCENT = 1; // same as above, but difference is that there it wasn't an error, it was showing a wrong thumbnail instead.
const MAX_CACHE_ENTRIES = 24;
const VIEWPORT_MARGIN_PX = 16;
const GAP_ABOVE_PROGRESS_PX = 14;
const ARROW_EDGE_MARGIN_PX = 14;

export interface ScrubPreviewViewport {
  progressTop?: number;
  viewportWidth?: number;
  viewportHeight?: number;
}

export interface ScrubPreviewSize {
  width: number;
  height: number;
}

export interface ScrubPreviewHorizontalBounds {
  left: number;
  right: number;
}

export interface ScrubPreviewOptions {
  buildPreviewUrl: (atPercentage: number) => string;
  formatTime?: (seconds: number) => string;
  getDuration?: () => number;
  previewWidthPx?: number;
  previewMaxHeightPx?: number;
  minIntervalMs?: number;
  scrubPercentStep?: number;
}

export function quantizeScrubPercent(percent: number, step = DEFAULT_SCRUB_PERCENT_STEP): number {
  if (!Number.isFinite(percent)) {
    return 0;
  }
  const clamped = Math.max(0, Math.min(100, percent));
  if (!(step > 0)) {
    return Math.round(clamped);
  }
  const bucket = Math.round(clamped / step) * step;
  return Math.max(0, Math.min(100, bucket));
}

export function scrubPreviewDelayMs(lastFetchAt: number, now: number, minIntervalMs = DEFAULT_MIN_INTERVAL_MS): number {
  const elapsed = now - lastFetchAt;
  return elapsed >= minIntervalMs ? 0 : minIntervalMs - elapsed;
}

export function formatScrubPreviewTime(seconds: number): string {
  if (!Number.isFinite(seconds) || seconds < 0) {
    return "0:00";
  }
  const total = Math.floor(seconds);
  const h = Math.floor(total / 3600);
  const m = Math.floor((total % 3600) / 60);
  const s = total % 60;
  const pad = (n: number) => String(n).padStart(2, "0");
  if (h > 0) {
    return `${h}:${pad(m)}:${pad(s)}`;
  }
  return `${m}:${pad(s)}`;
}

function isMouseEvent(event: Event): event is MouseEvent {
  return "clientX" in event;
}

function isTouchEvent(event: Event): event is TouchEvent {
  return "touches" in event || "changedTouches" in event;
}

export function scrubClientXFromEvent(event?: Event | null): number | null {
  if (!event) {
    return null;
  }
  if (isMouseEvent(event) && Number.isFinite(event.clientX)) {
    return event.clientX;
  }
  const touch = isTouchEvent(event) ? (event.changedTouches?.[0] ?? event.touches?.[0]) : undefined;
  return touch && Number.isFinite(touch.clientX) ? touch.clientX : null;
}

export function scrubPercentFromEvent(
  progress: HTMLElement,
  seek: HTMLInputElement,
  event: Event,
  step = DEFAULT_SCRUB_PERCENT_STEP,
): number {
  return quantizeScrubPercent(scrubRawPercentFromEvent(progress, seek, event), step);
}

export function scrubRawPercentFromEvent(progress: HTMLElement, seek: HTMLInputElement, event: Event): number {
  const clientX = scrubClientXFromEvent(event);
  if (progress && clientX !== null) {
    const rect = progress.getBoundingClientRect();
    if (rect.width > 0) {
      const raw = ((clientX - rect.left) / rect.width) * 100;
      return Math.max(0, Math.min(100, raw));
    }
  }
  if (!seek) {
    return 0;
  }
  const attr = seek.getAttribute("seek-value");
  const raw = attr !== null && attr !== "" ? Number(attr) : Number(seek.value);
  return Number.isFinite(raw) ? Math.max(0, Math.min(100, raw)) : 0;
}

export function fitScrubPreviewImageSize(
  imageWidth: number,
  imageHeight: number,
  viewport: ScrubPreviewViewport = {},
  maxWidthPx = DEFAULT_PREVIEW_WIDTH_PX,
  maxHeightPx = DEFAULT_MAX_PREVIEW_HEIGHT_PX,
): ScrubPreviewSize | null {
  if (!Number.isFinite(imageWidth) || !Number.isFinite(imageHeight) || imageWidth <= 0 || imageHeight <= 0) {
    return null;
  }

  const viewportWidth = viewport.viewportWidth ?? window.innerWidth;
  const viewportHeight = viewport.viewportHeight ?? window.innerHeight;
  const progressTop = viewport.progressTop ?? viewportHeight;

  const viewportMaxWidth = Math.max(1, viewportWidth - VIEWPORT_MARGIN_PX * 2);
  const viewportMaxHeight = Math.max(
    1,
    progressTop - VIEWPORT_MARGIN_PX - GAP_ABOVE_PROGRESS_PX,
  );
  const maxWidth = Math.max(1, Math.min(maxWidthPx, viewportMaxWidth));
  const maxHeight = Math.max(1, Math.min(maxHeightPx, viewportMaxHeight));

  const scale = Math.min(1, maxWidth / imageWidth, maxHeight / imageHeight);

  return {
    width: Math.max(1, Math.round(imageWidth * scale)),
    height: Math.max(1, Math.round(imageHeight * scale)),
  };
}

export function getScrubPreviewMount(player: Plyr): HTMLElement {
  const doc = document as Document & { webkitFullscreenElement?: Element | null };
  const fsEl = doc.fullscreenElement ?? doc.webkitFullscreenElement ?? null;
  const container = player?.elements?.container;
  if (fsEl instanceof HTMLElement) {
    if (container instanceof HTMLElement && (fsEl === container || fsEl.contains(container))) {
      return fsEl;
    }
    return fsEl;
  }
  if (container instanceof HTMLElement && player?.fullscreen?.active) {
    return container;
  }
  return container instanceof HTMLElement ? container : document.body;
}

let cachedRemPx: number | null = null;
let isResizeBound = false;

/** bounds for the popup accounting for the sidebar width if open */
export function getScrubPreviewHorizontalBounds(player: Plyr): ScrubPreviewHorizontalBounds {
  let left = 0;
  let right = window.innerWidth;

  const container = player?.elements?.container;
  if (container instanceof HTMLElement) {
    const rect = container.getBoundingClientRect();
    if (rect.width > 0) {
      left = rect.left;
      right = rect.right;
    }
  }
  if (getters.isSidebarVisible() && !player?.fullscreen?.active) {
    if (!cachedRemPx) {
      cachedRemPx = Number.parseFloat(getComputedStyle(document.documentElement).fontSize) || 16;
      if (!isResizeBound) {
        window.addEventListener("resize", () => { cachedRemPx = null; }, { passive: true });
        isResizeBound = true;
      }
    }
    left = Math.max(left, getters.sidebarWidth() * cachedRemPx);
  }
  if (right <= left) {
    // fallback to viewport when no sidebar
    return { left: 0, right: window.innerWidth };
  }
  return { left, right };
}

export function positionScrubPreviewArrow(arrowEl: HTMLElement, popup: HTMLElement, clientX: number): void {
  const popupRect = popup.getBoundingClientRect();
  const width = popupRect.width || 1;
  const margin = ARROW_EDGE_MARGIN_PX;
  const offset = Math.max(margin, Math.min(width - margin, clientX - popupRect.left));
  arrowEl.style.left = `${offset}px`;
}

export function positionScrubPreviewPopup(
  popup: HTMLElement,
  progressRect: DOMRect,
  clientX: number,
  bounds: ScrubPreviewHorizontalBounds = { left: 0, right: window.innerWidth },
): void {
  const width = popup.offsetWidth || DEFAULT_PREVIEW_WIDTH_PX;
  const half = width / 2;
  const margin = 8;
  const minX = bounds.left + half + margin;
  const maxX = bounds.right - half - margin;
  const clampedX = maxX >= minX
    ? Math.max(minX, Math.min(maxX, clientX))
    : (bounds.left + bounds.right) / 2;

  popup.style.left = `${clampedX}px`;
  popup.style.top = `${progressRect.top}px`;
}

interface ScrubPlayerInternals {
  media: HTMLMediaElement;
  elements: {
    progress?: HTMLElement | null;
    inputs?: { seek?: HTMLInputElement };
  };
}

export function enableScrubPreview(plyr: Plyr, options: ScrubPreviewOptions): () => void {
  const {
    buildPreviewUrl,
    formatTime = formatScrubPreviewTime,
    previewWidthPx = DEFAULT_PREVIEW_WIDTH_PX,
    previewMaxHeightPx = DEFAULT_MAX_PREVIEW_HEIGHT_PX,
    minIntervalMs = DEFAULT_MIN_INTERVAL_MS,
    scrubPercentStep = DEFAULT_SCRUB_PERCENT_STEP,
  } = options;

  const player = plyr as Plyr & ScrubPlayerInternals;
  const { elements } = player;
  const progress = elements?.progress;
  const seek = elements?.inputs?.seek;
  if (!progress || !seek || typeof buildPreviewUrl !== "function") {
    return () => {};
  }

  const popup = document.createElement("div");
  popup.className = SCRUB_PREVIEW_CLASS;
  popup.setAttribute("aria-hidden", "true");

  const frame = document.createElement("div");
  frame.className = "fb-scrub-preview__frame";

  const img = document.createElement("img");
  img.alt = "";
  img.decoding = "async";

  const loadingEl = document.createElement("div");
  loadingEl.className = "fb-scrub-preview__loading";
  loadingEl.hidden = true;
  loadingEl.innerHTML = '<span class="loader loader--small" aria-hidden="true"><span class="loader__comet"></span></span>';

  const timeEl = document.createElement("span");
  timeEl.className = "fb-scrub-preview__time";
  timeEl.setAttribute("aria-hidden", "true");

  // small arrow below the frame that tracks the cursor
  const arrowEl = document.createElement("div");
  arrowEl.className = "fb-scrub-preview__arrow";
  arrowEl.setAttribute("aria-hidden", "true");

  frame.append(img, loadingEl, timeEl);
  popup.append(frame, arrowEl);

  const ensurePopupMounted = () => {
    const mount = getScrubPreviewMount(player);
    if (popup.parentElement !== mount) {
      mount.appendChild(popup);
    }
  };

  ensurePopupMounted();

  const onFullscreenChange = () => {
    ensurePopupMounted();
    if (lastPositionEvent) {
      positionPopup(lastPositionEvent);
    }
  };

  player.on("enterfullscreen", onFullscreenChange);
  player.on("exitfullscreen", onFullscreenChange);
  document.addEventListener("fullscreenchange", onFullscreenChange);
  document.addEventListener("webkitfullscreenchange", onFullscreenChange);

  const cache = new Map<number, string>();
  let lastFailedPercent: number | null = null;
  let scrubbing = false;
  let hovering = false;
  let pendingPercent: number | null = null;
  /** Percent bucket currently shown in the popup frame. */
  let displayedPercent: number | null = null;
  /** Uncached bucket we're loading toward (spinner shown once per bucket). */
  let pendingVisualPercent: number | null = null;
  let lastFetchedPercent: number | null = null;
  let inFlightPercent: number | null = null;
  let lastFetchAt = 0;
  let debounceTimer: ReturnType<typeof setTimeout> | undefined;
  /** True while a throttle timer is already scheduled (do not reset on every mousemove). */
  let fetchScheduled = false;
  let abortController: AbortController | null = null;
  let lastPositionEvent: Event | null = null;

  const previewActive = () => scrubbing || hovering;

  const hasFrameImage = () => {
    const src = img.currentSrc || img.src;
    return typeof src === "string" && src.length > 0;
  };

  const getViewport = () => {
    const progressRect = progress.getBoundingClientRect();
    const bounds = getScrubPreviewHorizontalBounds(player);
    return {
      progressTop: progressRect.top,
      viewportWidth: bounds.right - bounds.left,
      viewportHeight: window.innerHeight,
    };
  };

  const applyFrameSize = (width: number, height: number) => {
    frame.style.width = `${width}px`;
    frame.style.height = `${height}px`;
  };

  const clearFrameSize = () => {
    frame.style.width = "";
    frame.style.height = "";
  };

  let lastKnownAspect: number | null = null;
  const getPlaceholderAspect = () => {
    const media = player.media;
    if (media instanceof HTMLVideoElement) {
      const w = media.videoWidth;
      const h = media.videoHeight;
      if (Number.isFinite(w) && Number.isFinite(h) && w > 0 && h > 0) {
        return w / h;
      }
    }
    return lastKnownAspect ?? DEFAULT_PLACEHOLDER_ASPECT;
  };

  /** Fit the frame from an aspect ratio alone */
  const applyFrameSizeForAspect = (aspect: number) => {
    const fitted = fitScrubPreviewImageSize(
      previewWidthPx,
      Math.round(previewWidthPx / aspect),
      getViewport(),
      previewWidthPx,
      previewMaxHeightPx,
    );
    if (fitted) {
      applyFrameSize(fitted.width, fitted.height);
    }
  };
  const applyPlaceholderFrameSize = () => {
    applyFrameSizeForAspect(getPlaceholderAspect());
  };

  const setLoading = (loading: boolean) => {
    loadingEl.hidden = !loading;
    frame.classList.toggle("fb-scrub-preview__frame--loading", loading);
    frame.classList.toggle("fb-scrub-preview__frame--empty", loading && !hasFrameImage());
  };

  const showLoadingPlaceholder = () => {
    applyPlaceholderFrameSize();
    setLoading(true);
  };

  /** Spinner over existing frame, or placeholder when nothing loaded yet. */
  const setPendingLoading = () => {
    if (hasFrameImage()) {
      setLoading(true);
      return;
    }
    showLoadingPlaceholder();
  };

  const markDisplayed = (percentInt: number) => {
    displayedPercent = percentInt;
    pendingVisualPercent = null;
    setLoading(false);
  };

  const updateFrameFromImage = () => {
    if (img.naturalWidth <= 0 || img.naturalHeight <= 0) {
      return;
    }
    lastKnownAspect = img.naturalWidth / img.naturalHeight;
    applyFrameSizeForAspect(lastKnownAspect);
    if (lastPositionEvent) {
      positionPopup(lastPositionEvent);
    }
  };

  const syncFrameToLoadedImage = () => {
    if (!img.complete || img.naturalWidth <= 0 || img.naturalHeight <= 0) {
      return false;
    }
    updateFrameFromImage();
    return true;
  };

  const onImageLoad = () => {
    if (pendingPercent !== null) {
      const expectedSrc = cache.get(pendingPercent);
      if (!expectedSrc || img.src !== expectedSrc) {
        return;
      }
      if (syncFrameToLoadedImage()) {
        markDisplayed(pendingPercent);
      }
      return;
    }
    if (syncFrameToLoadedImage()) {
      setLoading(false);
    }
  };

  img.addEventListener("load", onImageLoad);
  img.addEventListener("error", () => {
    setLoading(false);
  });

  const hide = () => {
    popup.classList.remove(SCRUB_PREVIEW_VISIBLE_CLASS);
    img.removeAttribute("src");
    clearFrameSize();
    setLoading(false);
    displayedPercent = null;
    pendingVisualPercent = null;
  };

  const show = () => {
    ensurePopupMounted();
    popup.classList.add(SCRUB_PREVIEW_VISIBLE_CLASS);
  };

  const positionPopup = (event: Event) => {
    ensurePopupMounted();
    const clientX = scrubClientXFromEvent(event);
    if (clientX === null) {
      return;
    }
    const progressRect = progress.getBoundingClientRect();
    const bounds = getScrubPreviewHorizontalBounds(player);
    positionScrubPreviewPopup(popup, progressRect, clientX, bounds);
    // clamp to the progress bar edges
    const arrowClientX = Math.max(progressRect.left, Math.min(progressRect.right, clientX));
    positionScrubPreviewArrow(arrowEl, popup, arrowClientX);
  };

  const updateTimeLabel = (percent: number) => {
    const duration = typeof options.getDuration === "function"
      ? options.getDuration()
      : player.duration;
    if (!Number.isFinite(duration) || duration <= 0) {
      timeEl.textContent = formatTime(0);
      return;
    }
    timeEl.textContent = formatTime((duration / 100) * percent);
  };

  const trimCache = () => {
    while (cache.size > MAX_CACHE_ENTRIES) {
      const oldest = cache.keys().next().value;
      if (oldest === undefined) {
        return;
      }
      const url = cache.get(oldest);
      if (url) {
        URL.revokeObjectURL(url);
      }
      cache.delete(oldest);
    }
  };

  const applyCachedImage = (percentInt: number) => {
    const cached = cache.get(percentInt);
    if (!cached) {
      return false;
    }
    if (img.src === cached && syncFrameToLoadedImage()) {
      markDisplayed(percentInt);
      return true;
    }
    img.src = cached;
    if (syncFrameToLoadedImage()) {
      markDisplayed(percentInt);
      return true;
    }
    setLoading(true);
    return true;
  };

  const clearFetchSchedule = () => {
    clearTimeout(debounceTimer);
    debounceTimer = undefined;
    fetchScheduled = false;
  };

  const scheduleFetch = () => {
    if (!previewActive() || pendingPercent === null) {
      return;
    }

    const target = pendingPercent;
    if (cache.has(target)) {
      lastFetchedPercent = target;
      applyCachedImage(target);
      return;
    }
    if (target === lastFailedPercent) {
      // Don't retry, but keep the spinner for feedback.
      setLoading(true);
      return;
    }
    if (inFlightPercent !== null || fetchScheduled) {
      return;
    }

    fetchScheduled = true;
    const delay = scrubPreviewDelayMs(lastFetchAt, Date.now(), minIntervalMs);
    debounceTimer = setTimeout(() => {
      debounceTimer = undefined;

      const nextTarget = pendingPercent;
      if (nextTarget === null || !previewActive()) {
        fetchScheduled = false;
        return;
      }
      if (cache.has(nextTarget)) {
        fetchScheduled = false;
        lastFetchedPercent = nextTarget;
        applyCachedImage(nextTarget);
        return;
      }
      if (nextTarget === lastFailedPercent) {
        fetchScheduled = false;
        setLoading(true);
        return;
      }

      // Reserve the slot and stamp the interval before any async work
      // so rapid scrub events cannot start a second request within minIntervalMs.
      lastFetchAt = Date.now();
      inFlightPercent = nextTarget;
      fetchScheduled = false;

      void fetchPreview(nextTarget).finally(() => {
        if (inFlightPercent === nextTarget) {
          inFlightPercent = null;
        }
        if (!previewActive() || pendingPercent === null) {
          return;
        }
        if (pendingPercent === lastFailedPercent) {
          setLoading(true);
          return;
        }
        if (
          pendingPercent !== lastFetchedPercent
          && !cache.has(pendingPercent)
          && inFlightPercent !== pendingPercent
        ) {
          scheduleFetch();
        }
      });
    }, delay);
  };

  const fetchPreview = async (percentInt: number): Promise<void> => {
    if (cache.has(percentInt)) {
      lastFetchedPercent = percentInt;
      applyCachedImage(percentInt);
      if (inFlightPercent === percentInt) {
        inFlightPercent = null;
      }
      return;
    }

    const url = buildPreviewUrl(percentInt);
    abortController?.abort();
    const controller = new AbortController();
    abortController = controller;
    if (inFlightPercent === null) {
      inFlightPercent = percentInt;
    }
    setPendingLoading();

    try {
      const objectUrl = await fetchPreviewImage(url, controller.signal);
      cache.set(percentInt, objectUrl);
      lastFetchedPercent = percentInt;
      if (lastFailedPercent === percentInt) {
        lastFailedPercent = null;
      }
      trimCache();
      if (previewActive() && pendingPercent === percentInt) {
        img.src = objectUrl;
        if (syncFrameToLoadedImage()) {
          markDisplayed(percentInt);
        }
      }
    } catch (err) {
      // Aborts and preview failures are expected while scrubbing quickly.
      if (errorName(err as PlayerError) !== "AbortError") {
        lastFailedPercent = percentInt;
        if (pendingPercent === percentInt) {
          setLoading(true);
        }
      }
    } finally {
      if (inFlightPercent === percentInt) {
        inFlightPercent = null;
      }
      if (abortController === controller) {
        abortController = null;
      }
    }
  };

  const handlePreviewPosition = (event: Event) => {
    const rawPercent = scrubRawPercentFromEvent(progress, seek, event);
    const percentInt = Math.max(
      MIN_SCRUB_FETCH_PERCENT,
      Math.min(quantizeScrubPercent(rawPercent, scrubPercentStep), MAX_SCRUB_FETCH_PERCENT),
    );
    lastPositionEvent = event;
    positionPopup(event);
    show();
    updateTimeLabel(rawPercent);

    // Same bucket - only reposition the popup, no image/time churn.
    if (percentInt === displayedPercent) {
      const displayedSrc = cache.get(displayedPercent);
      if (displayedSrc) {
        pendingPercent = percentInt;
        pendingVisualPercent = null;
        if (img.src !== displayedSrc) {
          applyCachedImage(percentInt);
          return;
        }
        setLoading(false);
        return;
      }
      displayedPercent = null;
    }

    pendingPercent = percentInt;
    if (lastFailedPercent !== null && lastFailedPercent !== percentInt) {
      lastFailedPercent = null;
    }

    if (cache.has(percentInt)) {
      applyCachedImage(percentInt);
      return;
    }

    if (pendingVisualPercent !== percentInt) {
      pendingVisualPercent = percentInt;
      setPendingLoading();
    } else if (hasFrameImage() && (fetchScheduled || inFlightPercent !== null)) {
      setLoading(true);
    }

    if (inFlightPercent !== null || fetchScheduled) {
      return;
    }

    scheduleFetch();
  };

  const onDocumentMove = (event: Event) => {
    if (!scrubbing) {
      return;
    }
    handlePreviewPosition(event);
  };

  const onScrubMove = (event: Event) => {
    if (!scrubbing) {
      return;
    }
    handlePreviewPosition(event);
  };

  const cancelHoverPreview = () => {
    hovering = false;
    if (scrubbing) {
      return;
    }
    pendingPercent = null;
    clearFetchSchedule();
    abortController?.abort();
    abortController = null;
    hide();
  };

  const onProgressHoverMove = (event: Event) => {
    if (scrubbing) {
      return;
    }
    hovering = true;
    handlePreviewPosition(event);
  };

  const onProgressHoverLeave = () => {
    cancelHoverPreview();
  };

  const onScrubStart = (event: Event) => {
    scrubbing = true;
    document.addEventListener("mousemove", onDocumentMove);
    document.addEventListener("touchmove", onDocumentMove, { passive: true });
    document.addEventListener("mouseup", onScrubEnd);
    document.addEventListener("touchend", onScrubEnd);
    document.addEventListener("touchcancel", onScrubEnd);
    handlePreviewPosition(event);
  };

  const onScrubEnd = () => {
    scrubbing = false;
    document.removeEventListener("mousemove", onDocumentMove);
    document.removeEventListener("touchmove", onDocumentMove);
    document.removeEventListener("mouseup", onScrubEnd);
    document.removeEventListener("touchend", onScrubEnd);
    document.removeEventListener("touchcancel", onScrubEnd);
    if (!hovering) {
      pendingPercent = null;
      clearFetchSchedule();
      abortController?.abort();
      abortController = null;
      hide();
    }
  };

  seek.addEventListener("mousedown", onScrubStart);
  seek.addEventListener("touchstart", onScrubStart, { passive: true });
  seek.addEventListener("input", onScrubMove);
  seek.addEventListener("mouseup", onScrubEnd);
  seek.addEventListener("touchend", onScrubEnd);
  seek.addEventListener("change", onScrubEnd);
  progress.addEventListener("mousemove", onProgressHoverMove);
  progress.addEventListener("mouseleave", onProgressHoverLeave);

  return () => {
    onScrubEnd();
    cancelHoverPreview();
    seek.removeEventListener("mousedown", onScrubStart);
    seek.removeEventListener("touchstart", onScrubStart);
    seek.removeEventListener("input", onScrubMove);
    seek.removeEventListener("mouseup", onScrubEnd);
    seek.removeEventListener("touchend", onScrubEnd);
    seek.removeEventListener("change", onScrubEnd);
    progress.removeEventListener("mousemove", onProgressHoverMove);
    progress.removeEventListener("mouseleave", onProgressHoverLeave);
    player.off("enterfullscreen", onFullscreenChange);
    player.off("exitfullscreen", onFullscreenChange);
    document.removeEventListener("fullscreenchange", onFullscreenChange);
    document.removeEventListener("webkitfullscreenchange", onFullscreenChange);
    for (const url of cache.values()) {
      URL.revokeObjectURL(url);
    }
    cache.clear();
    lastFailedPercent = null;
    popup.remove();
  };
}
