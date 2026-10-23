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

/**
 * The media-time range a session's synthetic VOD playlist can serve: the full
 * [0, durationSec]. Every session serves the on-demand playlist — a segment
 * request for an unproduced index waits inside the backend's forward gap or
 * repositions the encode job, so coverage is never narrower than the file.
 */
export function servableCoverage(session) {
  const end = session?.durationSec;
  if (!Number.isFinite(end) || end <= 0) {
    return null;
  }
  return { start: 0, end };
}

/**
 * Owns one transcode playback attempt: session start, HLS attach, cancellation,
 * and stale-async guards. Vue components provide DOM/player hooks only.
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
  }

  dispose({ stopSession = false } = {}) {
    this.disposed = true;
    this.attempt += 1;
    this._cancelInFlight({ stopSession });
    this.active = false;
    this.startInProgress = false;
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
    this._startChain = this._startChain
      .catch(() => {})
      .then(() => this._startOnce(params));
    return this._startChain;
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
        isCancelled: () => !this.isCurrentAttempt(attempt),
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
        const coverage = servableCoverage(controller.session);
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
      const vodStart = controller.session?.vodPlaylist
        && Number.isFinite(startSec)
        && startSec > 0
        ? startSec
        : -1;
      const hlsAttachPromise = attachHlsToVideo({
        video: el,
        session: controller.session,
        startPosition: vodStart,
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
    }
  }

  teardownLocal({ stopSession = false } = {}) {
    this.attempt += 1;
    this._cancelInFlight({ stopSession });
    // Invalidate any in-flight start: its finally block compares attempts and
    // would otherwise leave the flag set, blocking every future start.
    this.startInProgress = false;
  }

  updatePlayback({ currentTime, paused }) {
    this.sessionController?.updatePlayback({ currentTime, paused });
  }
}
