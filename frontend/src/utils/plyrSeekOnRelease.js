/**
 * Plyr seeks on every range "input" while dragging, which triggers media byte-range
 * fetches for each step. Block that via config.listeners.seek and commit on "change"
 * (mouseup / touchend / keyboard release).
 */

/**
 * @param {HTMLInputElement} input
 * @returns {number | null}
 */
export function readPlyrSeekPercent(input) {
  const attr = input.getAttribute('seek-value');
  if (attr !== null && attr !== '') {
    input.removeAttribute('seek-value');
    const parsed = Number(attr);
    return Number.isFinite(parsed) ? parsed : null;
  }
  const parsed = Number(input.value);
  return Number.isFinite(parsed) ? parsed : null;
}

/**
 * @param {import('plyr').default} player
 * @param {Event} [event]
 */
export function commitPlyrSeek(player, event) {
  const seek = player?.elements?.inputs?.seek;
  const duration = player?.duration;
  if (!seek || !Number.isFinite(duration) || duration <= 0) {
    return;
  }
  const target = event?.currentTarget instanceof HTMLInputElement ? event.currentTarget : seek;
  const percent = readPlyrSeekPercent(target);
  if (percent === null) {
    return;
  }
  player.currentTime = (percent / seek.max) * duration;
}

/**
 * @param {import('plyr').default} player
 * @returns {() => void}
 */
export function enablePlyrSeekOnRelease(player) {
  const seek = player?.elements?.inputs?.seek;
  if (!seek) {
    return () => {};
  }

  let lastCommitAt = 0;

  const onCommit = (event) => {
    const now = event.timeStamp;
    if (now === lastCommitAt) {
      return;
    }
    lastCommitAt = now;
    commitPlyrSeek(player, event);
  };

  // Capture mouseup/touchend so the seek lands before Plyr's bubble handler resumes playback.
  const captureOpts = { capture: true };
  const touchCaptureOpts = { capture: true, passive: true };

  seek.addEventListener('mouseup', onCommit, captureOpts);
  seek.addEventListener('touchend', onCommit, touchCaptureOpts);
  seek.addEventListener('change', onCommit);

  return () => {
    seek.removeEventListener('mouseup', onCommit, captureOpts);
    seek.removeEventListener('touchend', onCommit, touchCaptureOpts);
    seek.removeEventListener('change', onCommit);
  };
}

/** Plyr listeners.seek hook: return false to skip seek-on-input. */
export function blockPlyrSeekOnInput() {
  return false;
}
