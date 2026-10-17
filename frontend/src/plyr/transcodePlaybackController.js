import { attachHlsToVideo } from '@/plyr/hlsAdapter.js';
import { continuePlay } from '@/plyr/playbackIntent.js';
import { TranscodeSessionController } from '@/plyr/transcodeSession.js';
import { transcodeError, transcodeLog, transcodeWarn } from '@/plyr/transcodeDebug.js';
import { getTranscodeSessions } from '@/api/transcode.js';
import { isTranscodeHlsMode } from '@/plyr/transcodeMode.js';

const TRANSCODE_PROFILES = { quality: 'quality', balanced: 'balanced', datasaver: 'datasaver' };

function profileFromTranscodeMode(mode) {
  return TRANSCODE_PROFILES[mode] ?? TRANSCODE_PROFILES.quality;
}

// Within this slack past the produced edge, waiting for ~1x production beats
// restarting ffmpeg. Kept in step with the backend's repositionForwardSlackSec.
export const TRANSCODE_SEEK_FORWARD_SLACK_SEC = 12;

// Tolerates rounding when the playhead sits on a generation's start boundary.
export const TRANSCODE_SEEK_BACKWARD_EPS_SEC = 1;

/**
 * The source-second range the element currently exposes (buffered/produced
 * edge) — NOT the servable range, see servableCoverage.
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
 * The source-second range the generation can serve on demand: the full
 * [startSec, endSec] for a completed generation (every segment is on disk),
 * or up to the produced (≈buffered) edge for a live-paced one.
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
 * True when the target is before the generation start or past the produced
 * edge beyond slack — waiting for paced production would just stall.
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

// Debounce so a scrub's drag tail coalesces into one reposition request.
const SEEK_INTENT_DEBOUNCE_MS = 350;
// applyStartPosition clamps the playhead to the produced edge at attach time,
// so a reposition initially lands a few seconds before the intent. Poll until
// production passes the target, then re-apply it exactly.
const PRECISE_SEEK_POLL_MS = 400;
const PRECISE_SEEK_TIMEOUT_MS = 15_000;

/**
 * Owns one transcode playback attempt: session start, HLS attach, cancellation,
 * stale-async guards, and seek-intent handling. A seek outside the generation's
 * servable coverage repositions it via a new start at the target; seeks inside
 * coverage stay plain element seeks. Vue components provide DOM/player hooks
 * only.
 */
export class TranscodePlaybackController {
  constructor() {
    this.reset();
  }

  reset() {
    this.attempt = 0;
    this.attachAbort = null;
    this.hlsCleanup = null;
    this.sessionController = null;
    this.active = false;
    this.disposed = false;
    this.startInProgress = false;
    this._startChain = Promise.resolve();
    this._params = null;
    this._pendingSeekIntentSec = null;
    this._seekRepositionTimer = null;
    this._preciseSeekTimer = null;
  }

  dispose({ stopSession = false } = {}) {
    this.disposed = true;
    this.attempt += 1;
    this._cancelInFlight({ stopSession });
    this.active = false;
    this.startInProgress = false;
    this._clearPendingSeek();
  }

  /** Drop any pending seek intent (file switch, teardown, mode change). */
  clearPendingSeek() {
    this._clearPendingSeek();
  }

  _clearPendingSeek() {
    this._pendingSeekIntentSec = null;
    if (this._seekRepositionTimer) {
      clearTimeout(this._seekRepositionTimer);
      this._seekRepositionTimer = null;
    }
    if (this._preciseSeekTimer) {
      clearInterval(this._preciseSeekTimer);
      this._preciseSeekTimer = null;
    }
  }

  isCurrentAttempt(attempt) {
    return !this.disposed
      && attempt === this.attempt;
  }

  _cancelInFlight({ stopSession = false } = {}) {
    this.attachAbort?.abort();
    this.attachAbort = null;
    this.hlsCleanup?.();
    this.hlsCleanup = null;
    if (this.sessionController) {
      if (stopSession) {
        void this.sessionController.stop();
      } else {
        this.sessionController.detach();
      }
      this.sessionController = null;
    }
    this.active = false;
  }

  start(params) {
    // A mode switch mid-start must not reuse the old chain — it dropped
    // replaceSessionId/profile and raced the backend. Queue the latest params.
    if (this.startInProgress) {
      transcodeLog('superseding in-flight transcode start');
      this.attempt += 1;
      this.attachAbort?.abort();
    }
    this.startInProgress = true;
    this._params = params;
    this._startChain = this._startChain
      .catch(() => {})
      .then(() => this._startOnce(params));
    return this._startChain;
  }

  _videoElement() {
    return this._params?.getVideoElement?.() ?? null;
  }

  /**
   * Record a scrub/seek intent. Live-paced generations can only serve what the
   * producer has written, so a target outside the servable coverage must
   * reposition the session — debounced so drag tails coalesce. Seeks inside
   * coverage are plain element seeks and need no action here.
   */
  notifySeekIntent(sec) {
    if (this.disposed || !Number.isFinite(sec)) {
      return;
    }
    this._pendingSeekIntentSec = Math.max(0, sec);
    if (this._seekRepositionTimer) {
      clearTimeout(this._seekRepositionTimer);
    }
    this._seekRepositionTimer = setTimeout(() => {
      this._seekRepositionTimer = null;
      this._evaluateSeekIntent();
    }, SEEK_INTENT_DEBOUNCE_MS);
  }

  _evaluateSeekIntent() {
    const intent = this._pendingSeekIntentSec;
    if (intent == null || this.disposed) {
      return;
    }
    // A start/reposition is already rebuilding the session; the intent is
    // re-evaluated against the new coverage once it lands (see _startOnce's
    // finally block).
    if (this.startInProgress) {
      return;
    }
    const el = this._videoElement();
    // Servable — not just buffered — coverage: a completed generation can
    // serve its whole playlist on demand, so a seek inside it stays an
    // in-place element seek instead of tearing the session down.
    const coverage = servableCoverage(el, this.sessionController?.session);
    if (!coverage) {
      // Nothing attached yet — keep the intent for a later evaluation.
      return;
    }
    if (!seekOutsideCoverage(intent, coverage)) {
      this._pendingSeekIntentSec = null;
      return;
    }
    this._pendingSeekIntentSec = null;
    if (!this._params) {
      return;
    }
    const wasPlaying = el ? !el.paused : true;
    transcodeLog('seek past produced coverage — repositioning transcode', {
      targetSec: intent,
      sessionId: this.sessionController?.session?.id,
    });
    this._startPreciseSeekWait(intent);
    void this.start({
      ...this._params,
      // Supersede the session currently attached; a stale stored id would 404.
      replaceSessionId: this.sessionController?.session?.id ?? this._params.replaceSessionId,
      startSec: intent,
      shouldAutoplay: wasPlaying,
    });
  }

  _startPreciseSeekWait(targetSec) {
    if (this._preciseSeekTimer) {
      clearInterval(this._preciseSeekTimer);
    }
    const deadline = Date.now() + PRECISE_SEEK_TIMEOUT_MS;
    this._preciseSeekTimer = setInterval(() => {
      const el = this._videoElement();
      if (!el || this.disposed || Date.now() > deadline) {
        this._stopPreciseSeekWait();
        return;
      }
      const coverage = mediaCoverage(el);
      if (!coverage || coverage.end < targetSec - 0.75) {
        return;
      }
      const clamped = Math.min(Math.max(targetSec, coverage.start), coverage.end);
      if (Math.abs(el.currentTime - clamped) > 0.35) {
        try {
          el.currentTime = clamped;
        } catch (_) {
          // element not seekable yet; next poll retries
          return;
        }
      }
      this._stopPreciseSeekWait();
    }, PRECISE_SEEK_POLL_MS);
  }

  _stopPreciseSeekWait() {
    if (this._preciseSeekTimer) {
      clearInterval(this._preciseSeekTimer);
      this._preciseSeekTimer = null;
    }
  }

  async _startOnce({
    source,
    path,
    viewToken,
    mode,
    clientId,
    replaceSessionId,
    startSec,
    playPrime,
    getVideoElement,
    nextTick,
    shouldAutoplay,
    onFirstBuffered,
    onResumePlayback,
    onPlaybackStarted,
    onFatal,
    onCapacityStatus,
    onShowError,
    onAttachNative,
  }) {
    if (!source || !path || !viewToken || !isTranscodeHlsMode(mode)) {
      transcodeWarn('transcode playback start skipped — invalid preconditions', {
        source,
        path,
        mode,
        hasViewToken: Boolean(viewToken),
      });
      // start() set the flag; clearing it here keeps later starts from coalescing
      // onto this already-settled chain.
      this.startInProgress = false;
      return { ok: false, reason: 'preconditions' };
    }

    if (this.disposed) {
      this.startInProgress = false;
      return { ok: false, reason: 'disposed' };
    }

    this._cancelInFlight({ stopSession: false });
    const attempt = ++this.attempt;
    const profile = profileFromTranscodeMode(mode);
    const controller = new TranscodeSessionController({
      source,
      path,
      viewToken,
      profile,
      clientId,
    });
    this.sessionController = controller;

    const resumePlayback = () => {
      const el = getVideoElement?.();
      transcodeLog('transcode resumePlayback', {
        shouldAutoplay,
        hasVideoElement: Boolean(el),
        mediaPaused: el?.paused,
        readyState: el?.readyState,
      });
      if (!shouldAutoplay) {
        return;
      }
      if (!el) {
        return;
      }
      void continuePlay(el, playPrime, {
        playCall: onResumePlayback,
        onPlaying: onPlaybackStarted,
      });
    };

    // Restore the remembered playhead after a source swap. The timeline is
    // source-relative; clamp to the session's servable range and seek directly.
    const applyStartPosition = (video) => {
      const target = Number(startSec);
      if (!video || !Number.isFinite(target) || target <= 0) {
        return;
      }
      try {
        let clamped = target;
        const coverage = servableCoverage(video, controller.session);
        if (coverage) {
          clamped = Math.min(Math.max(target, coverage.start), coverage.end);
        }
        if (Math.abs(video.currentTime - clamped) < 0.25) {
          return;
        }
        video.currentTime = clamped;
        transcodeLog('applied resume position after HLS attach', {
          requested: target,
          applied: clamped,
          currentTime: video.currentTime,
        });
      } catch (err) {
        transcodeWarn('failed to apply resume position', { target, message: err?.message });
      }
    };

    try {
      await controller.start({ replaceSessionId, startSec });
      if (!this.isCurrentAttempt(attempt)) {
        transcodeLog('discarding stale session start response', {
          sessionId: controller.session?.id,
          attempt,
          currentAttempt: this.attempt,
          disposed: this.disposed,
        });
        void controller.stop();
        return { ok: false, reason: 'stale' };
      }

      const el = getVideoElement?.();
      if (!el) {
        transcodeError('transcode playback aborted — no video element after session start', {
          sessionId: controller.session?.id,
        });
        void controller.stop();
        return { ok: false, reason: 'no-video' };
      }

      // Avoid browser autoplay/load attempts on an empty element during HLS handoff.
      el.autoplay = false;

      if (nextTick) {
        await nextTick();
      }
      if (!this.isCurrentAttempt(attempt)) {
        void controller.stop();
        return { ok: false, reason: 'stale' };
      }

      if (el.hasAttribute('src')) {
        el.removeAttribute('src');
      }

      const attachAbort = new AbortController();
      this.attachAbort = attachAbort;
      const hlsAttachPromise = attachHlsToVideo({
        video: el,
        session: controller.session,
        // startPosition mapping across a source swap is unreliable; the exact
        // playhead is restored via applyStartPosition instead.
        startPosition: -1,
        signal: attachAbort.signal,
        onFirstBuffered: () => {
          if (!this.isCurrentAttempt(attempt)) {
            return;
          }
          transcodeLog('transcode onFirstBuffered', {
            sessionId: controller.session?.id,
            attempt,
          });
          // Mark the HLS session active before resuming the element. The
          // resulting `play` event must not be mistaken for a new start.
          this.active = true;
          applyStartPosition(getVideoElement?.());
          onFirstBuffered?.();
          resumePlayback();
        },
        onFatal: (data) => {
          if (!this.isCurrentAttempt(attempt)) {
            return;
          }
          onFatal?.(data);
        },
      });

      const hlsAttach = await hlsAttachPromise;
      if (!this.isCurrentAttempt(attempt)) {
        hlsAttach.cleanup();
        return { ok: false, reason: 'stale' };
      }

      this.attachAbort = null;
      this.hlsCleanup = hlsAttach.cleanup;
      this.active = true;
      applyStartPosition(getVideoElement?.());
      transcodeLog('HLS attach complete', { sessionId: controller.session?.id });

      return { ok: true, mode, session: controller.session };
    } catch (err) {
      if (err?.name === 'AbortError' || !this.isCurrentAttempt(attempt)) {
        transcodeLog('transcode playback attempt cancelled');
        return { ok: false, reason: 'cancelled' };
      }

      transcodeError('transcode playback start failed', {
        message: err?.message,
        status: err?.status,
        type: err?.type,
        details: err?.details,
      });
      this.active = false;

      if (err?.status === 409 || err?.status === 503) {
        try {
          const status = await getTranscodeSessions();
          onCapacityStatus?.(status);
        } catch (fetchErr) {
          console.error('Failed to refresh transcode sessions after limit:', fetchErr);
        }
        return { ok: false, reason: 'capacity', error: err };
      }

      if (onAttachNative) {
        onAttachNative(err);
        return { ok: false, reason: 'fallback', error: err };
      }

      onShowError?.(err);
      return { ok: false, reason: 'error', error: err };
    } finally {
      if (attempt === this.attempt) {
        this.startInProgress = false;
      }
      // A seek intent issued while this start was in flight is checked
      // against the fresh coverage now that the flags have cleared.
      this._evaluateSeekIntent();
    }
  }

  teardownLocal({ stopSession = false } = {}) {
    this.attempt += 1;
    this._cancelInFlight({ stopSession });
    // Invalidate any in-flight start: its finally block compares attempts and
    // would otherwise leave the flag set, blocking every future start.
    this.startInProgress = false;
    this._clearPendingSeek();
  }

  updatePlayback({ currentTime, paused }) {
    this.sessionController?.updatePlayback({ currentTime, paused });
  }
}
