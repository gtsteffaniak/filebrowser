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

/**
 * Resume playback once media is ready. Invokes playCall and onPlaying on success.
 */
export function resumeWhenReady(video, { playCall, onPlaying } = {}) {
  if (!video) {
    transcodeWarn('resumeWhenReady skipped — no video element');
    return;
  }
  const tryPlay = () => {
    const invoke = playCall ?? (() => video.play());
    transcodeLog('resumeWhenReady tryPlay', {
      readyState: video.readyState,
      paused: video.paused,
      hasPlayCall: Boolean(playCall),
    });
    void Promise.resolve(invoke()).then(() => {
      transcodeLog('resumeWhenReady play resolved');
      onPlaying?.();
    }).catch((err) => {
      transcodeWarn('resumeWhenReady play rejected', { name: err?.name, message: err?.message });
    });
  };

  if (video.readyState >= HTMLMediaElement.HAVE_FUTURE_DATA && !video.paused) {
    transcodeLog('resumeWhenReady skipped — already playing');
    return;
  }
  if (video.readyState >= HTMLMediaElement.HAVE_FUTURE_DATA) {
    tryPlay();
    return;
  }
  transcodeLog('resumeWhenReady waiting for canplay', { readyState: video.readyState });
  video.addEventListener('canplay', tryPlay, { once: true });
}

/** Await a primed play promise after HLS attach, or fall back to resumeWhenReady. */
export async function continuePlay(video, playPromise, { playCall, onPlaying } = {}) {
  transcodeLog('continuePlay', {
    hasPlayPromise: Boolean(playPromise),
    readyState: video?.readyState,
    paused: video?.paused,
  });
  if (playPromise) {
    try {
      await playPromise;
      transcodeLog('continuePlay primed promise resolved');
      onPlaying?.();
      return;
    } catch (err) {
      transcodeWarn('continuePlay primed promise rejected', { name: err?.name, message: err?.message });
      if (isPlayTeardownError(err)) {
        resumeWhenReady(video, { playCall, onPlaying });
        return;
      }
      if (isPlayBlockedError(err)) {
        transcodeWarn('continuePlay blocked by autoplay policy');
        return;
      }
      resumeWhenReady(video, { playCall, onPlaying });
      return;
    }
  }
  resumeWhenReady(video, { playCall, onPlaying });
}
