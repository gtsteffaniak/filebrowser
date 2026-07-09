import { fetchPreviewImage } from '@/utils/previewRequests';

const SCRUB_PREVIEW_CLASS = 'fb-scrub-preview';
const SCRUB_PREVIEW_VISIBLE_CLASS = 'fb-scrub-preview--visible';
const DEFAULT_MIN_INTERVAL_MS = 500;
const DEFAULT_PREVIEW_WIDTH_PX = 600;
const DEFAULT_MAX_PREVIEW_HEIGHT_PX = 600;
const DEFAULT_PLACEHOLDER_ASPECT = 16 / 9;
const DEFAULT_SCRUB_PERCENT_STEP = 2;
const MAX_SCRUB_FETCH_PERCENT = 99; // 99 to never request a preview at the exact end of the media, it was causing an error.
const MAX_CACHE_ENTRIES = 24;
const VIEWPORT_MARGIN_PX = 16;
const GAP_ABOVE_PROGRESS_PX = 14;
const ARROW_EDGE_MARGIN_PX = 14;

/**
 * @param {number} percent
 * @param {number} [step]
 * @returns {number}
 */
export function quantizeScrubPercent(percent, step = DEFAULT_SCRUB_PERCENT_STEP) {
  if (!Number.isFinite(percent)) {
    return 0;
  }
  const clamped = Math.max(0, Math.min(100, percent));
  if (step <= 1) {
    return Math.round(clamped);
  }
  const bucket = Math.round(clamped / step) * step;
  return Math.max(0, Math.min(100, bucket));
}

/**
 * @param {number} lastFetchAt
 * @param {number} now
 * @param {number} minIntervalMs
 * @returns {number}
 */
export function scrubPreviewDelayMs(lastFetchAt, now, minIntervalMs = DEFAULT_MIN_INTERVAL_MS) {
  const elapsed = now - lastFetchAt;
  return elapsed >= minIntervalMs ? 0 : minIntervalMs - elapsed;
}

/**
 * @param {number} seconds
 * @returns {string}
 */
export function formatScrubPreviewTime(seconds) {
  if (!Number.isFinite(seconds) || seconds < 0) {
    return '0:00';
  }
  const total = Math.floor(seconds);
  const h = Math.floor(total / 3600);
  const m = Math.floor((total % 3600) / 60);
  const s = total % 60;
  const pad = (n) => String(n).padStart(2, '0');
  if (h > 0) {
    return `${h}:${pad(m)}:${pad(s)}`;
  }
  return `${m}:${pad(s)}`;
}

/**
 * @param {Event} event
 * @returns {number | null}
 */
export function scrubClientXFromEvent(event) {
  if (!event) {
    return null;
  }
  if (Number.isFinite(event.clientX)) {
    return event.clientX;
  }
  const touch = event.changedTouches?.[0] ?? event.touches?.[0];
  return touch && Number.isFinite(touch.clientX) ? touch.clientX : null;
}

/**
 * @param {HTMLElement} progress
 * @param {HTMLInputElement} seek
 * @param {Event} event
 * @param {number} [step]
 * @returns {number}
 */
export function scrubPercentFromEvent(progress, seek, event, step = DEFAULT_SCRUB_PERCENT_STEP) {
  return quantizeScrubPercent(scrubRawPercentFromEvent(progress, seek, event), step);
}

/**
 * @param {HTMLElement} progress
 * @param {HTMLInputElement} seek
 * @param {Event} event
 * @returns {number}
 */
export function scrubRawPercentFromEvent(progress, seek, event) {
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
  const attr = seek.getAttribute('seek-value');
  const raw = attr !== null && attr !== '' ? Number(attr) : Number(seek.value);
  return Number.isFinite(raw) ? Math.max(0, Math.min(100, raw)) : 0;
}

/**
 * @param {number} imageWidth
 * @param {number} imageHeight
 * @param {{
 *   progressTop?: number,
 *   viewportWidth?: number,
 *   viewportHeight?: number,
 * }} [viewport]
 * @param {number} [maxWidthPx]
 * @param {number} [maxHeightPx]
 * @returns {{ width: number, height: number } | null}
 */
export function fitScrubPreviewImageSize(
  imageWidth,
  imageHeight,
  viewport = {},
  maxWidthPx = DEFAULT_PREVIEW_WIDTH_PX,
  maxHeightPx = DEFAULT_MAX_PREVIEW_HEIGHT_PX,
) {
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

/**
 * @param {import('plyr').default} player
 * @returns {HTMLElement}
 */
export function getScrubPreviewMount(player) {
  const fsEl = document.fullscreenElement ?? document.webkitFullscreenElement ?? null;
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

/**
 * @param {HTMLElement} arrowEl
 * @param {HTMLElement} popup
 * @param {number} clientX
 */
export function positionScrubPreviewArrow(arrowEl, popup, clientX) {
  const popupRect = popup.getBoundingClientRect();
  const width = popupRect.width || 1;
  const margin = ARROW_EDGE_MARGIN_PX;
  const offset = Math.max(margin, Math.min(width - margin, clientX - popupRect.left));
  arrowEl.style.left = `${offset}px`;
}

/**
 * @param {HTMLElement} popup
 * @param {DOMRect} progressRect
 * @param {number} clientX
 * @param {number} [viewportWidth]
 */
export function positionScrubPreviewPopup(popup, progressRect, clientX, viewportWidth = window.innerWidth) {
  const width = popup.offsetWidth || DEFAULT_PREVIEW_WIDTH_PX;
  const half = width / 2;
  const margin = 8;
  const clampedX = Math.max(half + margin, Math.min(viewportWidth - half - margin, clientX));

  popup.style.left = `${clampedX}px`;
  popup.style.top = `${progressRect.top}px`;
}

/**
 * @param {import('plyr').default} player
 * @param {{
 *   buildPreviewUrl: (atPercentage: number) => string,
 *   formatTime?: (seconds: number) => string,
 *   getDuration?: () => number,
 *   previewWidthPx?: number,
 *   previewMaxHeightPx?: number,
 *   minIntervalMs?: number,
 *   scrubPercentStep?: number,
 * }} options
 * @returns {() => void}
 */
export function enablePlyrScrubPreview(player, options) {
  const {
    buildPreviewUrl,
    formatTime = formatScrubPreviewTime,
    previewWidthPx = DEFAULT_PREVIEW_WIDTH_PX,
    previewMaxHeightPx = DEFAULT_MAX_PREVIEW_HEIGHT_PX,
    minIntervalMs = DEFAULT_MIN_INTERVAL_MS,
    scrubPercentStep = DEFAULT_SCRUB_PERCENT_STEP,
  } = options;

  const progress = player.elements?.progress;
  const seek = player.elements?.inputs?.seek;
  if (!progress || !seek || typeof buildPreviewUrl !== 'function') {
    return () => {};
  }

  const popup = document.createElement('div');
  popup.className = SCRUB_PREVIEW_CLASS;
  popup.setAttribute('aria-hidden', 'true');

  const frame = document.createElement('div');
  frame.className = 'fb-scrub-preview__frame';

  const img = document.createElement('img');
  img.alt = '';
  img.decoding = 'async';

  const loadingEl = document.createElement('div');
  loadingEl.className = 'fb-scrub-preview__loading';
  loadingEl.hidden = true;
  loadingEl.innerHTML = '<span class="loader loader--small" aria-hidden="true"><span class="loader__comet"></span></span>';

  const timeEl = document.createElement('span');
  timeEl.className = 'fb-scrub-preview__time';
  timeEl.setAttribute('aria-hidden', 'true');

  // small arrow below the frame that tracks the cursor
  const arrowEl = document.createElement('div');
  arrowEl.className = 'fb-scrub-preview__arrow';
  arrowEl.setAttribute('aria-hidden', 'true');

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

  player.on('enterfullscreen', onFullscreenChange);
  player.on('exitfullscreen', onFullscreenChange);
  document.addEventListener('fullscreenchange', onFullscreenChange);
  document.addEventListener('webkitfullscreenchange', onFullscreenChange);

  /** @type {Map<number, string>} */
  const cache = new Map();
  let lastFailedPercent = null;
  let scrubbing = false;
  let hovering = false;
  let pendingPercent = null;
  /** Percent bucket currently shown in the popup frame. */
  let displayedPercent = null;
  /** Uncached bucket we're loading toward (spinner shown once per bucket). */
  let pendingVisualPercent = null;
  let lastFetchedPercent = null;
  let inFlightPercent = null;
  let lastFetchAt = 0;
  /** @type {ReturnType<typeof setTimeout> | null} */
  let debounceTimer = null;
  /** True while a throttle timer is already scheduled (do not reset on every mousemove). */
  let fetchScheduled = false;
  /** @type {AbortController | null} */
  let abortController = null;
  /** @type {Event | null} */
  let lastPositionEvent = null;

  const previewActive = () => scrubbing || hovering;

  const hasFrameImage = () => {
    const src = img.currentSrc || img.src;
    return typeof src === 'string' && src.length > 0;
  };

  const getViewport = () => {
    const progressRect = progress.getBoundingClientRect();
    return {
      progressTop: progressRect.top,
      viewportWidth: window.innerWidth,
      viewportHeight: window.innerHeight,
    };
  };

  const applyFrameSize = (width, height) => {
    frame.style.width = `${width}px`;
    frame.style.height = `${height}px`;
  };

  const clearFrameSize = () => {
    frame.style.width = '';
    frame.style.height = '';
  };

  const applyPlaceholderFrameSize = () => {
    const fitted = fitScrubPreviewImageSize(
      previewWidthPx,
      Math.round(previewWidthPx / DEFAULT_PLACEHOLDER_ASPECT),
      getViewport(),
      previewWidthPx,
      previewMaxHeightPx,
    );
    if (fitted) {
      applyFrameSize(fitted.width, fitted.height);
    }
  };

  const setLoading = (loading) => {
    loadingEl.hidden = !loading;
    frame.classList.toggle('fb-scrub-preview__frame--loading', loading);
    frame.classList.toggle('fb-scrub-preview__frame--empty', loading && !hasFrameImage());
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

  const markDisplayed = (percentInt) => {
    displayedPercent = percentInt;
    pendingVisualPercent = null;
    setLoading(false);
  };

  const updateFrameFromImage = () => {
    const fitted = fitScrubPreviewImageSize(
      img.naturalWidth,
      img.naturalHeight,
      getViewport(),
      previewWidthPx,
      previewMaxHeightPx,
    );
    if (!fitted) {
      return;
    }
    applyFrameSize(fitted.width, fitted.height);
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
      if (expectedSrc && img.src !== expectedSrc) {
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

  img.addEventListener('load', onImageLoad);
  img.addEventListener('error', () => {
    setLoading(false);
  });

  const hide = () => {
    popup.classList.remove(SCRUB_PREVIEW_VISIBLE_CLASS);
    img.removeAttribute('src');
    clearFrameSize();
    setLoading(false);
    displayedPercent = null;
    pendingVisualPercent = null;
  };

  const show = () => {
    ensurePopupMounted();
    popup.classList.add(SCRUB_PREVIEW_VISIBLE_CLASS);
  };

  const positionPopup = (event) => {
    ensurePopupMounted();
    const clientX = scrubClientXFromEvent(event);
    if (clientX === null) {
      return;
    }
    positionScrubPreviewPopup(popup, progress.getBoundingClientRect(), clientX);
    positionScrubPreviewArrow(arrowEl, popup, clientX);
  };

  const updateTimeLabel = (percent) => {
    const duration = typeof options.getDuration === 'function'
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
      const url = cache.get(oldest);
      if (url) {
        URL.revokeObjectURL(url);
      }
      cache.delete(oldest);
    }
  };

  const applyCachedImage = (percentInt) => {
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
    debounceTimer = null;
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
      // Don't retry, but keep the spinner for feedback since can make the use think that the image fetched when isn't.
      setLoading(true);
      return;
    }
    if (inFlightPercent !== null || fetchScheduled) {
      return;
    }

    fetchScheduled = true;
    const delay = scrubPreviewDelayMs(lastFetchAt, Date.now(), minIntervalMs);
    debounceTimer = setTimeout(() => {
      fetchScheduled = false;
      debounceTimer = null;

      const nextTarget = pendingPercent;
      if (nextTarget === null || !previewActive()) {
        return;
      }
      if (cache.has(nextTarget)) {
        lastFetchedPercent = nextTarget;
        applyCachedImage(nextTarget);
        return;
      }
      if (nextTarget === lastFailedPercent) {
        setLoading(true);
        return;
      }
      if (inFlightPercent === nextTarget) {
        return;
      }

      setPendingLoading();
      lastFetchAt = Date.now();
      void fetchPreview(nextTarget).finally(() => {
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

  const fetchPreview = async (percentInt) => {
    if (cache.has(percentInt)) {
      lastFetchedPercent = percentInt;
      applyCachedImage(percentInt);
      return;
    }

    const url = buildPreviewUrl(percentInt);
    abortController?.abort();
    const controller = new AbortController();
    abortController = controller;
    inFlightPercent = percentInt;
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
      if (err?.name !== 'AbortError') {
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

  const handlePreviewPosition = (event) => {
    const rawPercent = scrubRawPercentFromEvent(progress, seek, event);
    const percentInt = Math.min(
      quantizeScrubPercent(rawPercent, scrubPercentStep),
      MAX_SCRUB_FETCH_PERCENT,
    );
    lastPositionEvent = event;
    positionPopup(event);
    show();
    updateTimeLabel(rawPercent);

    // Same bucket — only reposition the popup, no image/time churn.
    if (percentInt === displayedPercent) {
      return;
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

    if (inFlightPercent === percentInt || fetchScheduled) {
      return;
    }

    scheduleFetch();
  };

  const onDocumentMove = (event) => {
    if (!scrubbing) {
      return;
    }
    handlePreviewPosition(event);
  };

  const onScrubMove = (event) => {
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

  const onProgressHoverMove = (event) => {
    if (scrubbing) {
      return;
    }
    hovering = true;
    handlePreviewPosition(event);
  };

  const onProgressHoverLeave = () => {
    cancelHoverPreview();
  };

  const onScrubStart = (event) => {
    scrubbing = true;
    document.addEventListener('mousemove', onDocumentMove);
    document.addEventListener('touchmove', onDocumentMove, { passive: true });
    document.addEventListener('mouseup', onScrubEnd);
    document.addEventListener('touchend', onScrubEnd);
    document.addEventListener('touchcancel', onScrubEnd);
    handlePreviewPosition(event);
  };

  const onScrubEnd = () => {
    scrubbing = false;
    document.removeEventListener('mousemove', onDocumentMove);
    document.removeEventListener('touchmove', onDocumentMove, { passive: true });
    document.removeEventListener('mouseup', onScrubEnd);
    document.removeEventListener('touchend', onScrubEnd);
    document.removeEventListener('touchcancel', onScrubEnd);
    if (!hovering) {
      pendingPercent = null;
      clearFetchSchedule();
      abortController?.abort();
      abortController = null;
      hide();
    }
  };

  seek.addEventListener('mousedown', onScrubStart);
  seek.addEventListener('touchstart', onScrubStart, { passive: true });
  seek.addEventListener('input', onScrubMove);
  seek.addEventListener('mouseup', onScrubEnd);
  seek.addEventListener('touchend', onScrubEnd);
  seek.addEventListener('change', onScrubEnd);
  progress.addEventListener('mousemove', onProgressHoverMove);
  progress.addEventListener('mouseleave', onProgressHoverLeave);

  return () => {
    onScrubEnd();
    cancelHoverPreview();
    seek.removeEventListener('mousedown', onScrubStart);
    seek.removeEventListener('touchstart', onScrubStart, { passive: true });
    seek.removeEventListener('input', onScrubMove);
    seek.removeEventListener('mouseup', onScrubEnd);
    seek.removeEventListener('touchend', onScrubEnd);
    seek.removeEventListener('change', onScrubEnd);
    progress.removeEventListener('mousemove', onProgressHoverMove);
    progress.removeEventListener('mouseleave', onProgressHoverLeave);
    player.off('enterfullscreen', onFullscreenChange);
    player.off('exitfullscreen', onFullscreenChange);
    document.removeEventListener('fullscreenchange', onFullscreenChange);
    document.removeEventListener('webkitfullscreenchange', onFullscreenChange);
    for (const url of cache.values()) {
      URL.revokeObjectURL(url);
    }
    cache.clear();
    lastFailedPercent = null;
    popup.remove();
  };
}
