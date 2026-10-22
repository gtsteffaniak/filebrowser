import { transcodeLog, transcodeWarn } from '@/plyr/transcodeDebug.js';

/**
 * Helpers for preserving playback intent across src/HLS handoffs.
 * Mirrors native attachVideoStreamAndPlay / attemptPipHandoffPlay semantics.
 */

export function isPlayBlockedError(err) {
  if (!err) {
    return false;
  }
  if (err.name === 'NotAllowedError') {
    return true;
  }
  const message = String(err.message || '');
  return message.includes('user didn') || message.includes('not allowed');
}

export function isPlayTeardownError(err) {
  if (!err) {
    return false;
  }
  if (err.name === 'AbortError') {
    return true;
  }
  if (err.name === 'NotSupportedError') {
    return true;
  }
  const message = String(err.message || '');
  return message.includes('no supported source');
}

/** Call synchronously during a user gesture; returns a pending play promise. */
export function primePlay(video) {
  if (!video) {
    return null;
  }
  let promise;
  try {
    promise = video.play();
  } catch (err) {
    promise = Promise.reject(err);
  }
  // Handoff clears src shortly after; swallow interim rejection until continuePlay retries.
  void promise.catch(() => {});
  return promise;
}

/** How long resumeWhenReady waits for canplay before giving up. */
const PLAY_READY_TIMEOUT_MS = 10_000;

/**
 * True only when the element is really rendering: unpaused, not ended, and with
 * enough data buffered. `paused === false` alone is not proof — the element stays
 * unpaused across a MediaSource swap while showing a frozen frame.
 */
export function isPlaybackProgressing(video) {
  return Boolean(video)
    && !video.paused
    && !video.ended
    && video.readyState >= HTMLMediaElement.HAVE_FUTURE_DATA;
}

/**
 * Resume playback once media is ready.
 * @returns {Promise<{ played: boolean, blocked: boolean }>}
 */
export function resumeWhenReady(video, { playCall, onPlaying, isCancelled } = {}) {
  return new Promise((resolve) => {
    if (!video) {
      transcodeWarn('resumeWhenReady skipped — no video element');
      resolve({ played: false, blocked: false });
      return;
    }
    if (isPlaybackProgressing(video)) {
      transcodeLog('resumeWhenReady skipped — already playing');
      resolve({ played: true, blocked: false });
      return;
    }

    let settled = false;
    let timerId = null;
    const finish = (result) => {
      if (settled) {
        return;
      }
      settled = true;
      if (timerId !== null) {
        clearTimeout(timerId);
      }
      video.removeEventListener('canplay', tryPlay);
      resolve(result);
    };
    const tryPlay = () => {
      // The owning preview may have been closed/unmounted while we waited for
      // canplay. Never start playback on an element that is being torn down.
      if (isCancelled?.()) {
        transcodeLog('resumeWhenReady cancelled before play');
        finish({ played: false, blocked: false });
        return;
      }
      const invoke = playCall ?? (() => video.play());
      transcodeLog('resumeWhenReady tryPlay', {
        readyState: video.readyState,
        paused: video.paused,
        hasPlayCall: Boolean(playCall),
      });
      void Promise.resolve(invoke()).then(() => {
        transcodeLog('resumeWhenReady play resolved');
        onPlaying?.();
        finish({ played: true, blocked: false });
      }).catch((err) => {
        transcodeWarn('resumeWhenReady play rejected', { name: err?.name, message: err?.message });
        finish({ played: false, blocked: isPlayBlockedError(err) });
      });
    };

    timerId = setTimeout(() => {
      transcodeWarn('resumeWhenReady timed out waiting for canplay', { readyState: video.readyState });
      finish({ played: false, blocked: false });
    }, PLAY_READY_TIMEOUT_MS);

    if (video.readyState >= HTMLMediaElement.HAVE_FUTURE_DATA) {
      tryPlay();
      return;
    }
    transcodeLog('resumeWhenReady waiting for canplay', { readyState: video.readyState });
    video.addEventListener('canplay', tryPlay);
  });
}

/**
 * Await a primed play promise after HLS attach, or fall back to resumeWhenReady.
 * @returns {Promise<{ played: boolean, blocked: boolean }>}
 */
export async function continuePlay(video, playPromise, { playCall, onPlaying, isCancelled } = {}) {
  transcodeLog('continuePlay', {
    hasPlayPromise: Boolean(playPromise),
    readyState: video?.readyState,
    paused: video?.paused,
  });
  if (playPromise) {
    try {
      await playPromise;
      if (isCancelled?.()) {
        transcodeLog('continuePlay cancelled after primed promise');
        return { played: false, blocked: false };
      }
      // The primed promise may belong to a source that has since been replaced
      // (for example a transcode mode switch), so its resolution is not proof
      // that the current element is actually playing.
      if (isPlaybackProgressing(video)) {
        transcodeLog('continuePlay primed promise resolved — current source playing');
        onPlaying?.();
        return { played: true, blocked: false };
      }
      transcodeWarn('continuePlay primed promise resolved but current source is not playing', {
        readyState: video?.readyState,
        paused: video?.paused,
      });
    } catch (err) {
      transcodeWarn('continuePlay primed promise rejected', { name: err?.name, message: err?.message });
      if (isPlayBlockedError(err)) {
        transcodeWarn('continuePlay blocked by autoplay policy');
        return { played: false, blocked: true };
      }
    }
  }
  return resumeWhenReady(video, { playCall, onPlaying, isCancelled });
}
