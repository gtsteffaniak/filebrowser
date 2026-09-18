import router from '@/router';
import { state } from '@/store';
import { buildItemUrl } from '@/utils/url.js';

/** @typedef {{ source: string, path: string, media: HTMLMediaElement, wasInlineFullscreen?: boolean, wasPlaying?: boolean }} PipSession */

/** @type {PipSession | null} */
let session = null;

/** @type {boolean} */
let programmaticPipExit = false;

/** @type {{ source: string, path: string, currentTime: number, wasPlaying: boolean, wasFullscreen: boolean, timestamp: number } | null} */
let pendingInlineResume = null;

let listenerRegistered = false;

/** @type {Set<() => void>} */
const pipAvailabilityListeners = new Set();

/** @type {Set<(snapshot: NonNullable<typeof pendingInlineResume>) => void>} */
const pendingInlineResumeListeners = new Set();

/** @type {WeakMap<HTMLMediaElement, EventListener>} */
const sessionMediaLeaveHandlers = new WeakMap();

/** @type {boolean} */
let pipHandoffInProgress = false;

/** @type {HTMLElement | null} */
let pipSessionHost = null;

function ensurePipSessionHost() {
  if (pipSessionHost || typeof document === 'undefined') {
    return pipSessionHost;
  }
  pipSessionHost = document.createElement('div');
  pipSessionHost.id = 'pip-session-host';
  pipSessionHost.setAttribute('aria-hidden', 'true');
  Object.assign(pipSessionHost.style, {
    position: 'fixed',
    width: '1px',
    height: '1px',
    opacity: '0',
    pointerEvents: 'none',
    overflow: 'hidden',
    left: '-9999px',
    top: '0',
  });
  document.body.appendChild(pipSessionHost);
  return pipSessionHost;
}

/** Keep PiP media in the document when the preview component unmounts. */
function adoptPipMediaToHost(media) {
  if (!media || typeof document === 'undefined') {
    return;
  }
  const host = ensurePipSessionHost();
  if (!host) {
    return;
  }
  if (media.parentElement !== host) {
    host.appendChild(media);
  }
}

function releasePipMediaFromHost(media) {
  if (media?.parentElement) {
    try {
      media.parentElement.removeChild(media);
    } catch (_) {
      // ignore
    }
  }
  if (pipSessionHost?.childElementCount === 0) {
    try {
      pipSessionHost.remove();
    } catch (_) {
      // ignore
    }
    pipSessionHost = null;
  }
}

function notifyPipAvailabilityChange() {
  pipAvailabilityListeners.forEach((listener) => {
    try {
      listener();
    } catch (_) {
      // ignore
    }
  });
}

/** @param {() => void} listener @returns {() => void} unsubscribe */
export function onPipAvailabilityChange(listener) {
  pipAvailabilityListeners.add(listener);
  return () => {
    pipAvailabilityListeners.delete(listener);
  };
}

function notifyPendingInlineResume(snapshot) {
  pendingInlineResumeListeners.forEach((listener) => {
    try {
      listener(snapshot);
    } catch (_) {
      // ignore
    }
  });
}

/** @param {(snapshot: NonNullable<typeof pendingInlineResume>) => void} listener @returns {() => void} unsubscribe */
export function onPendingInlineResume(listener) {
  pendingInlineResumeListeners.add(listener);
  return () => {
    pendingInlineResumeListeners.delete(listener);
  };
}

/**
 * True when another PiP window or handoff session prevents starting PiP here.
 * @param {string} source
 * @param {string} path
 * @param {HTMLMediaElement | null | undefined} [media]
 */
export function isPipBlockedForPreview(source, path, _media = null) {
  const pipEl = document.pictureInPictureElement;
  if (pipEl instanceof HTMLMediaElement) {
    return true;
  }
  if (session && !mediaMatches(source, path, session.source, session.path)) {
    return true;
  }
  return false;
}

function resolveMediaSource(source) {
  if (source) {
    return source;
  }
  return state.sources?.current ?? '';
}

function normalizeMediaPath(path) {
  if (!path) {
    return '';
  }
  return path.startsWith('/') ? path : `/${path}`;
}

function mediaMatches(source, path, otherSource, otherPath) {
  return resolveMediaSource(source) === resolveMediaSource(otherSource)
    && normalizeMediaPath(path) === normalizeMediaPath(otherPath);
}

/** @param {string} url @param {number} seconds */
export function appendMediaFragment(url, seconds) {
  if (!url || !Number.isFinite(seconds) || seconds <= 0) {
    return url;
  }
  const base = String(url).split('#')[0];
  return `${base}#t=${seconds}`;
}

export function resolvePipMediaKey(source, path) {
  return {
    source: resolveMediaSource(source),
    path: normalizeMediaPath(path),
  };
}

export function hasActiveSession(source, path) {
  if (!session) {
    return false;
  }
  return mediaMatches(source, path, session.source, session.path);
}

export function hasAnyActiveSession() {
  return session !== null;
}

export function getActiveSession() {
  return session;
}

export function pendingInlineResumeFor(source, path) {
  if (!pendingInlineResume) {
    return null;
  }
  if (!mediaMatches(source, path, pendingInlineResume.source, pendingInlineResume.path)) {
    return null;
  }
  return pendingInlineResume;
}

export function getPendingInlineResume() {
  return pendingInlineResume;
}

/** Whether a pending handoff targets this preview path. */
export function pendingMatchesPreview(source, path, snapshot = pendingInlineResume) {
  if (!snapshot) {
    return false;
  }
  return mediaMatches(source, path, snapshot.source, snapshot.path);
}

export function clearPendingInlineResume() {
  pendingInlineResume = null;
}

/** Clear pending resume only when it matches the given preview. */
export function clearPendingInlineResumeFor(source, path) {
  if (!pendingInlineResume) {
    return;
  }
  if (!mediaMatches(source, path, pendingInlineResume.source, pendingInlineResume.path)) {
    return;
  }
  clearPendingInlineResume();
}

export function setPendingInlineResume(snapshot) {
  pendingInlineResume = snapshot;
}

function detachSessionMediaLeaveListener(media) {
  if (!media) {
    return;
  }
  const handler = sessionMediaLeaveHandlers.get(media);
  if (handler) {
    media.removeEventListener('leavepictureinpicture', handler);
    sessionMediaLeaveHandlers.delete(media);
  }
}

function attachSessionMediaLeaveListener(media) {
  detachSessionMediaLeaveListener(media);
  const handler = onSessionMediaLeavePip;
  sessionMediaLeaveHandlers.set(media, handler);
  media.addEventListener('leavepictureinpicture', handler);
}

function isSessionMediaLeaveEvent(event, media) {
  const target = event.target;
  if (target === media) {
    return true;
  }
  const pipEl = document.pictureInPictureElement;
  return pipEl instanceof HTMLMediaElement && pipEl === media;
}

function pauseInlineVideosExcept(exceptMedia = null) {
  if (typeof document === 'undefined') {
    return;
  }
  document.querySelectorAll('video').forEach((el) => {
    if (!(el instanceof HTMLMediaElement)) {
      return;
    }
    if (exceptMedia && el === exceptMedia) {
      return;
    }
    try {
      el.pause();
    } catch (_) {
      // ignore
    }
  });
}

function handlePipSessionClosed(media) {
  if (pipHandoffInProgress || !session || media !== session.media) {
    return;
  }
  pipHandoffInProgress = true;
  try {
    const { source, path } = session;
    const currentTime = Number.isFinite(media.currentTime) ? media.currentTime : 0;
    const wasPlaying = !media.paused;

    const resumeSnapshot = {
      source,
      path,
      currentTime,
      wasPlaying,
      wasFullscreen: false,
      timestamp: Date.now(),
    };

    detachSessionMediaLeaveListener(media);
    session = null;
    pauseInlineVideosExcept(media);
    retirePipMedia(media);
    pendingInlineResume = resumeSnapshot;
    scheduleBackToTabNavigation(source, path);
    notifyPipAvailabilityChange();
  } finally {
    pipHandoffInProgress = false;
  }
}

function onSessionMediaLeavePip(event) {
  if (programmaticPipExit) {
    return;
  }
  if (!session || !isSessionMediaLeaveEvent(event, session.media)) {
    return;
  }
  handlePipSessionClosed(session.media);
}

/**
 * Remember which media element is in PiP when leaving the preview.
 * Does not move, pause, or mutate the element — PiP keeps playing as before.
 * @param {HTMLMediaElement} media
 * @param {{ source: string, path: string }} meta
 */
export function registerSession(media, { source, path, wasInlineFullscreen = false, wasPlaying }) {
  if (!media) {
    return;
  }
  if (session?.media && session.media !== media) {
    detachSessionMediaLeaveListener(session.media);
  }
  const key = resolvePipMediaKey(source, path);
  const playing = wasPlaying !== undefined ? wasPlaying : !media.paused;
  session = {
    source: key.source,
    path: key.path,
    media,
    wasInlineFullscreen: Boolean(wasInlineFullscreen),
    wasPlaying: Boolean(playing),
  };
  adoptPipMediaToHost(media);
  attachSessionMediaLeaveListener(media);
  notifyPipAvailabilityChange();
}

export function clearSession() {
  if (session?.media) {
    detachSessionMediaLeaveListener(session.media);
  }
  session = null;
  notifyPipAvailabilityChange();
}

/**
 * Stop and discard an orphaned PiP media element after handoff.
 * @param {HTMLMediaElement | null | undefined} media
 */
export function retirePipMedia(media) {
  if (!media) {
    return;
  }
  detachSessionMediaLeaveListener(media);
  try {
    media.pause();
  } catch (_) {
    // ignore
  }
  media.removeAttribute('src');
  try {
    media.load();
  } catch (_) {
    // ignore
  }
  releasePipMediaFromHost(media);
}

/**
 * Whether the preview should defer binding :src until PiP handoff completes.
 * @param {string} source
 * @param {string} path
 */
export function shouldDeferVideoStreamAttach(source, path) {
  if (hasActiveSession(source, path)) {
    return true;
  }
  return pendingInlineResumeFor(source, path) !== null;
}

/**
 * Read playback position from the PiP media element, exit PiP, stop orphan, clear session.
 * @param {string} source
 * @param {string} path
 * @returns {Promise<{ currentTime: number, wasPlaying: boolean } | null>}
 */
export async function takeSessionSnapshot(source, path) {
  if (!hasActiveSession(source, path)) {
    return null;
  }
  const { media, source: sessionSource, path: sessionPath, wasInlineFullscreen = false, wasPlaying: sessionWasPlaying = true } = session;
  detachSessionMediaLeaveListener(media);
  session = null;

  const pipEl = document.pictureInPictureElement;
  const timeEl = pipEl instanceof HTMLMediaElement ? pipEl : media;
  const currentTime = Number.isFinite(timeEl?.currentTime) ? timeEl.currentTime : 0;
  const wasPlaying = pipEl instanceof HTMLMediaElement ? !pipEl.paused : sessionWasPlaying;

  if (document.pictureInPictureElement) {
    await exitPictureInPictureProgrammatically();
  }

  retirePipMedia(media);

  const snap = {
    source: sessionSource,
    path: sessionPath,
    currentTime,
    wasPlaying,
    wasFullscreen: wasInlineFullscreen,
  };
  notifyPipAvailabilityChange();
  return snap;
}

export async function exitPictureInPictureProgrammatically() {
  if (!document.pictureInPictureElement) {
    return;
  }
  programmaticPipExit = true;
  try {
    await document.exitPictureInPicture();
  } catch (_) {
    // ignore
  } finally {
    await new Promise((resolve) => {
      queueMicrotask(resolve);
    });
    programmaticPipExit = false;
  }
}

function notifyAfterHandoffNavigation(source, path) {
  requestAnimationFrame(() => {
    requestAnimationFrame(() => {
      if (
        pendingInlineResume
        && mediaMatches(source, path, pendingInlineResume.source, pendingInlineResume.path)
      ) {
        notifyPendingInlineResume(pendingInlineResume);
      }
    });
  });
}

function navigateToPipVideo(source, path, attempt = 0) {
  if (document.visibilityState !== 'visible' && attempt < 2) {
    setTimeout(() => navigateToPipVideo(source, path, attempt + 1), 50);
    return;
  }
  const expected = buildItemUrl(source, path);
  const current = state.route?.path ?? '';
  const alreadyOnTarget = current === expected || current.startsWith(`${expected}#`);
  if (alreadyOnTarget) {
    notifyAfterHandoffNavigation(source, path);
    return;
  }
  void router.push(expected);
}

function scheduleBackToTabNavigation(source, path) {
  requestAnimationFrame(() => {
    navigateToPipVideo(source, path);
  });
}

function onLeavePictureInPicture(event) {
  if (programmaticPipExit) {
    return;
  }
  if (!session) {
    return;
  }
  const target = event.target;
  if (!(target instanceof HTMLMediaElement) || !isSessionMediaLeaveEvent(event, session.media)) {
    return;
  }
  handlePipSessionClosed(session.media);
}

function onDocumentPipStateChange() {
  notifyPipAvailabilityChange();
}

export function initPipSession() {
  if (listenerRegistered || typeof document === 'undefined') {
    return;
  }
  document.addEventListener('leavepictureinpicture', onLeavePictureInPicture);
  document.addEventListener('enterpictureinpicture', onDocumentPipStateChange);
  document.addEventListener('leavepictureinpicture', onDocumentPipStateChange);
  listenerRegistered = true;
}
