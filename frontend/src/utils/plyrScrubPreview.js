import { fetchPreviewImage } from '@/utils/previewRequests';

const SCRUB_PREVIEW_CLASS = 'fb-scrub-preview';
const SCRUB_PREVIEW_VISIBLE_CLASS = 'fb-scrub-preview--visible';
const DEFAULT_MIN_INTERVAL_MS = 500;
const DEFAULT_PREVIEW_WIDTH_PX = 500;
const DEFAULT_ASPECT_RATIO = 16 / 9;
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
 * @param {number} aspectRatio
 * @param {number} preferredWidth
 * @param {{
 *   progressTop?: number,
 *   viewportWidth?: number,
 *   viewportHeight?: number,
 * }} [viewport]
 * @returns {{ width: number, height: number }}
 */
export function scrubPreviewDimensions(aspectRatio, preferredWidth = DEFAULT_PREVIEW_WIDTH_PX, viewport = {}) {
  const ratio = Number.isFinite(aspectRatio) && aspectRatio > 0
    ? aspectRatio
    : DEFAULT_ASPECT_RATIO;

  const viewportWidth = viewport.viewportWidth ?? window.innerWidth;
  const viewportHeight = viewport.viewportHeight ?? window.innerHeight;
  const progressTop = viewport.progressTop ?? viewportHeight;

  const maxWidth = Math.max(1, viewportWidth - VIEWPORT_MARGIN_PX * 2);
  const maxHeight = Math.max(
    1,
    progressTop - VIEWPORT_MARGIN_PX - GAP_ABOVE_PROGRESS_PX - TIME_LABEL_HEIGHT_PX,
  );

  let width = Math.min(preferredWidth, maxWidth);
  let height = Math.round(width / ratio);

  if (height > maxHeight) {
    height = maxHeight;
    width = Math.round(height * ratio);
  }

  if (width > maxWidth) {
    width = maxWidth;
    height = Math.round(width / ratio);
  }

  width = Math.max(1, width);
  height = Math.max(1, height);

  return { width, height };
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
 *   getAspectRatio?: () => number,
 *   previewWidthPx?: number,
 *   minIntervalMs?: number,
 * }} options
 * @returns {() => void}
 */
export function enablePlyrScrubPreview(player, options) {
  const {
    buildPreviewUrl,
    formatTime = formatScrubPreviewTime,
    getAspectRatio = () => DEFAULT_ASPECT_RATIO,
    previewWidthPx = DEFAULT_PREVIEW_WIDTH_PX,
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

  const timeEl = document.createElement('span');
  timeEl.className = 'fb-scrub-preview__time';

  frame.appendChild(img);
  popup.append(frame, timeEl);
  document.body.appendChild(popup);

  /** @type {Map<number, string>} */
  const cache = new Map();
  let scrubbing = false;
  let pendingPercent = null;
  let lastFetchedPercent = null;
  let lastFetchAt = 0;
  /** @type {ReturnType<typeof setTimeout> | null} */
  let debounceTimer = null;
  /** @type {AbortController | null} */
  let abortController = null;

  const updatePopupDimensions = () => {
    const progressRect = progress.getBoundingClientRect();
    const { width, height } = scrubPreviewDimensions(getAspectRatio(), previewWidthPx, {
      progressTop: progressRect.top,
      viewportWidth: window.innerWidth,
      viewportHeight: window.innerHeight,
    });
    frame.style.width = `${width}px`;
    frame.style.height = `${height}px`;
  };

  const hide = () => {
    popup.classList.remove(SCRUB_PREVIEW_VISIBLE_CLASS);
    img.removeAttribute('src');
  };

  const show = () => {
    popup.classList.add(SCRUB_PREVIEW_VISIBLE_CLASS);
  };

  const positionPopup = (event) => {
    const clientX = scrubClientXFromEvent(event);
    if (clientX === null) {
      return;
    }
    positionScrubPreviewPopup(popup, progress.getBoundingClientRect(), clientX);
  };

  const updateTimeLabel = (percentInt) => {
    const duration = player.duration;
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
    if (cached) {
      img.src = cached;
    }
  };

  const fetchPreview = async (percentInt) => {
    if (cache.has(percentInt)) {
      applyCachedImage(percentInt);
      return;
    }

    abortController?.abort();
    const controller = new AbortController();
    abortController = controller;

    try {
      const objectUrl = await fetchPreviewImage(buildPreviewUrl(percentInt), controller.signal);
      cache.set(percentInt, objectUrl);
      trimCache();
      if (scrubbing && pendingPercent === percentInt) {
        img.src = objectUrl;
      }
    } catch {
      // Aborts and preview failures are expected while scrubbing quickly.
    } finally {
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
      if (!scrubbing || pendingPercent !== percentInt) {
        return;
      }
      lastFetchAt = Date.now();
      lastFetchedPercent = percentInt;
      fetchPreview(percentInt);
    }, delay);
  };

  const handleScrubPosition = (event) => {
    const percentInt = scrubPercentFromEvent(progress, seek, event);
    pendingPercent = percentInt;
    updatePopupDimensions();
    positionPopup(event);
    updateTimeLabel(percentInt);
    show();
    applyCachedImage(percentInt);

    if (!scrubPercentChanged(lastFetchedPercent, percentInt)) {
      return;
    }
    queueFetch(percentInt);
  };

  const onDocumentMove = (event) => {
    if (!scrubbing) {
      return;
    }
    handleScrubPosition(event);
  };

  const onScrubMove = (event) => {
    if (!scrubbing) {
      return;
    }
    handleScrubPosition(event);
  };

  const onScrubStart = (event) => {
    scrubbing = true;
    document.addEventListener('mousemove', onDocumentMove);
    document.addEventListener('touchmove', onDocumentMove, { passive: true });
    handleScrubPosition(event);
  };

  const onScrubEnd = () => {
    scrubbing = false;
    pendingPercent = null;
    document.removeEventListener('mousemove', onDocumentMove);
    document.removeEventListener('touchmove', onDocumentMove);
    clearTimeout(debounceTimer);
    debounceTimer = null;
    abortController?.abort();
    abortController = null;
    hide();
  };

  seek.addEventListener('mousedown', onScrubStart);
  seek.addEventListener('touchstart', onScrubStart, { passive: true });
  seek.addEventListener('input', onScrubMove);
  seek.addEventListener('mouseup', onScrubEnd);
  seek.addEventListener('touchend', onScrubEnd);
  seek.addEventListener('change', onScrubEnd);

  return () => {
    onScrubEnd();
    seek.removeEventListener('mousedown', onScrubStart);
    seek.removeEventListener('touchstart', onScrubStart);
    seek.removeEventListener('input', onScrubMove);
    seek.removeEventListener('mouseup', onScrubEnd);
    seek.removeEventListener('touchend', onScrubEnd);
    seek.removeEventListener('change', onScrubEnd);
    for (const url of cache.values()) {
      URL.revokeObjectURL(url);
    }
    cache.clear();
    popup.remove();
  };
}
