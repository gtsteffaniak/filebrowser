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
    return;
  }
  const tryPlay = () => {
    const invoke = playCall ?? (() => video.play());
    void Promise.resolve(invoke()).then(() => {
      onPlaying?.();
    }).catch((err) => {
      if (!isPlayTeardownError(err)) {
        // Blocked or failed play — leave paused without retry spam.
      }
    });
  };

  if (video.readyState >= HTMLMediaElement.HAVE_FUTURE_DATA && !video.paused) {
    return;
  }
  if (video.readyState >= HTMLMediaElement.HAVE_FUTURE_DATA) {
    tryPlay();
    return;
  }
  video.addEventListener('canplay', tryPlay, { once: true });
}

/** Await a primed play promise after HLS attach, or fall back to resumeWhenReady. */
export async function continuePlay(video, playPromise, { playCall, onPlaying } = {}) {
  if (playPromise) {
    try {
      await playPromise;
      onPlaying?.();
      return;
    } catch (err) {
      if (isPlayTeardownError(err)) {
        resumeWhenReady(video, { playCall, onPlaying });
        return;
      }
      if (isPlayBlockedError(err)) {
        return;
      }
      resumeWhenReady(video, { playCall, onPlaying });
      return;
    }
  }
  resumeWhenReady(video, { playCall, onPlaying });
}
