/**
 * Transcode seek-reposition helpers.
 *
 * With live-paced production the HLS playlist only covers what ffmpeg has
 * produced so far. A seek outside that coverage can never be served by the
 * current generation — the backend must reposition it at the seek point.
 * These pure helpers decide when a requested playhead needs that reposition.
 */

// How far past the produced edge a seek may target before repositioning.
// Within slack it is cheaper to let ~1x production reach the target than to
// restart ffmpeg. Kept in step with the backend's repositionForwardSlackSec.
export const TRANSCODE_SEEK_FORWARD_SLACK_SEC = 12;

// Tolerates rounding when the playhead sits on a generation's start boundary.
export const TRANSCODE_SEEK_BACKWARD_EPS_SEC = 1;

/**
 * The contiguous range the media element currently exposes, in source
 * seconds. With source-time-aligned HLS output this is the buffered/produced
 * edge — NOT the servable range (see servableCoverage): a completed
 * generation can serve its whole playlist on demand while buffered only
 * covers what hls.js has appended so far.
 * @param {HTMLMediaElement|null} el
 * @returns {{start: number, end: number} | null}
 */
export function mediaCoverage(el) {
  const seekable = el?.seekable;
  if (!seekable || seekable.length === 0) {
    return null;
  }
  try {
    return { start: seekable.start(0), end: seekable.end(seekable.length - 1) };
  } catch {
    return null;
  }
}

/**
 * The contiguous range the session's generation can serve on demand, in
 * source seconds. For a completed generation every segment exists on disk,
 * so the whole produced range [session.startSec, session.endSec] is
 * servable even though only part of it is buffered. For a live generation
 * production is ~1x-paced, so the servable end is the produced edge —
 * approximated by the buffered end (what hls.js has appended tracks
 * production closely enough for seek decisions).
 * @param {HTMLMediaElement|null} el
 * @param {object|null} session - SessionInfo; uses startSec/endSec/state/durationSec.
 * @returns {{start: number, end: number} | null}
 */
export function servableCoverage(el, session) {
  const start = Number.isFinite(session?.startSec) ? session.startSec : 0;
  if (session?.state === 'completed') {
    const end = Number.isFinite(session?.endSec)
      ? session.endSec
      : session?.durationSec;
    if (!Number.isFinite(end) || end < start) {
      return null;
    }
    return { start, end };
  }
  const buffered = mediaCoverage(el);
  if (!buffered) {
    return null;
  }
  return { start: Math.min(start, buffered.start), end: buffered.end };
}

/**
 * Reports whether the requested playhead falls outside what the current
 * generation can serve: before its start (the data was never produced) or far
 * enough past the produced edge that waiting for paced production is pointless.
 * @param {number} targetSec
 * @param {{start: number, end: number} | null} coverage
 */
export function seekOutsideCoverage(targetSec, coverage) {
  if (!Number.isFinite(targetSec) || targetSec < 0 || !coverage) {
    return false;
  }
  if (targetSec < coverage.start - TRANSCODE_SEEK_BACKWARD_EPS_SEC) {
    return true;
  }
  return targetSec > coverage.end + TRANSCODE_SEEK_FORWARD_SLACK_SEC;
}
