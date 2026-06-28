import { fetchPreviewImage } from '@/utils/previewRequests';

const SCRUB_PREVIEW_CLASS = 'fb-scrub-preview';
const SCRUB_PREVIEW_VISIBLE_CLASS = 'fb-scrub-preview--visible';
const DEFAULT_MIN_INTERVAL_MS = 600;
const DEFAULT_PREVIEW_WIDTH_PX = 600;
const DEFAULT_MAX_PREVIEW_HEIGHT_PX = 600;
const DEFAULT_PLACEHOLDER_ASPECT = 16 / 9;
const MAX_CACHE_ENTRIES = 24;
const VIEWPORT_MARGIN_PX = 16;
const GAP_ABOVE_PROGRESS_PX = 14;
const TIME_LABEL_HEIGHT_PX = 28;

/**
 * @param {number} percent
 * @returns {number}
 */
export function quantizeScrubPercent(percent) {
  if (!Number.isFinite(percent)) {
    return 0;
  }
  return Math.round(Math.max(0, Math.min(100, percent)));
}

/**
 * @param {number | null} lastPercent
 * @param {number} nextPercent
 * @returns {boolean}
 */
export function scrubPercentChanged(lastPercent, nextPercent) {
  return lastPercent !== nextPercent;
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
 * @returns {number}
 */
export function scrubPercentFromEvent(progress, seek, event) {
  if (event?.currentTarget === seek) {
    const attr = seek.getAttribute('seek-value');
    const raw = attr !== null && attr !== '' ? Number(attr) : Number(seek.value);
    return quantizeScrubPercent(raw);
  }
  const clientX = scrubClientXFromEvent(event);
  if (!progress || clientX === null) {
    return 0;
  }
  const rect = progress.getBoundingClientRect();
  if (rect.width <= 0) {
    return 0;
  }
  const raw = ((clientX - rect.left) / rect.width) * 100;
  return quantizeScrubPercent(raw);
}

/**
 * Scale preview image dimensions down to fit viewport and max caps.
 * Never upscales — returned size matches the preview image when it already fits.
 *
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
    progressTop - VIEWPORT_MARGIN_PX - GAP_ABOVE_PROGRESS_PX - TIME_LABEL_HEIGHT_PX,
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
 * DOM node that should host the scrub preview so it stays visible in fullscreen.
 *
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
 * Position a fixed popup centered on the cursor, sitting above the progress bar.
 *
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
 *   previewWidthPx?: number,
 *   previewMaxHeightPx?: number,
 *   minIntervalMs?: number,
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

  frame.append(img, loadingEl);
  popup.append(frame, timeEl);

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
  let scrubbing = false;
  let hovering = false;
  let pendingPercent = null;
  let lastFetchedPercent = null;
  let inFlightPercent = null;
  let lastFetchAt = 0;
  /** @type {ReturnType<typeof setTimeout> | null} */
  let debounceTimer = null;
  /** @type {AbortController | null} */
  let abortController = null;
  /** @type {Event | null} */
  let lastPositionEvent = null;

  const previewActive = () => scrubbing || hovering;

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
  };

  const showLoadingPlaceholder = () => {
    applyPlaceholderFrameSize();
    setLoading(true);
  };

  const syncFrameToLoadedImage = () => {
    if (!img.complete || img.naturalWidth <= 0 || img.naturalHeight <= 0) {
      return false;
    }
    updateFrameFromImage();
    setLoading(false);
    return true;
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

  const onImageLoad = () => {
    if (pendingPercent !== null) {
      const expectedSrc = cache.get(pendingPercent);
      if (expectedSrc && img.src !== expectedSrc) {
        return;
      }
    }
    syncFrameToLoadedImage();
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
  };

  const updateTimeLabel = (percentInt) => {
    const duration = typeof options.getDuration === 'function'
      ? options.getDuration()
      : player.duration;
    if (!Number.isFinite(duration) || duration <= 0) {
      timeEl.textContent = formatTime(0);
      return;
    }
    timeEl.textContent = formatTime((duration / 100) * percentInt);
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
    img.src = cached;
    if (syncFrameToLoadedImage()) {
      return true;
    }
    showLoadingPlaceholder();
    return true;
  };

  const fetchPreview = async (percentInt) => {
    if (cache.has(percentInt)) {
      lastFetchedPercent = percentInt;
      applyCachedImage(percentInt);
      return;
    }

    abortController?.abort();
    const controller = new AbortController();
    abortController = controller;
    inFlightPercent = percentInt;
    showLoadingPlaceholder();

    try {
      const objectUrl = await fetchPreviewImage(buildPreviewUrl(percentInt), controller.signal);
      cache.set(percentInt, objectUrl);
      lastFetchedPercent = percentInt;
      trimCache();
      if (previewActive() && pendingPercent === percentInt) {
        img.src = objectUrl;
        syncFrameToLoadedImage();
      }
    } catch {
      // Aborts and preview failures are expected while scrubbing quickly.
    } finally {
      if (inFlightPercent === percentInt) {
        inFlightPercent = null;
      }
      if (abortController === controller) {
        abortController = null;
      }
    }
  };

  const queueFetch = (percentInt) => {
    clearTimeout(debounceTimer);
    const delay = scrubPreviewDelayMs(lastFetchAt, Date.now(), minIntervalMs);
    debounceTimer = setTimeout(() => {
      debounceTimer = null;
      if (!previewActive() || pendingPercent !== percentInt) {
        return;
      }
      lastFetchAt = Date.now();
      void fetchPreview(percentInt);
    }, delay);
  };

  const handlePreviewPosition = (event) => {
    const percentInt = scrubPercentFromEvent(progress, seek, event);
    pendingPercent = percentInt;
    lastPositionEvent = event;
    positionPopup(event);
    updateTimeLabel(percentInt);
    show();
    if (!applyCachedImage(percentInt)) {
      img.removeAttribute('src');
      showLoadingPlaceholder();
    }

    if (!scrubPercentChanged(lastFetchedPercent, percentInt) || inFlightPercent === percentInt) {
      return;
    }
    queueFetch(percentInt);
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
    clearTimeout(debounceTimer);
    debounceTimer = null;
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
      clearTimeout(debounceTimer);
      debounceTimer = null;
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
    seek.removeEventListener('touchstart', onScrubStart);
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
    popup.remove();
  };
}
