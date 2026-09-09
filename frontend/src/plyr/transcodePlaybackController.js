import { attachHlsToVideo } from '@/plyr/hlsAdapter.js';
import { continuePlay } from '@/plyr/playbackIntent.js';
import { profileFromTranscodeMode } from '@/plyr/transcodeProfile.js';
import { TranscodeSessionController } from '@/plyr/transcodeSession.js';
import { transcodeError, transcodeLog, transcodeWarn } from '@/plyr/transcodeDebug.js';
import { getTranscodeSessions } from '@/api/transcode.js';
import { isTranscodeHlsMode } from '@/plyr/transcodeMode.js';

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
    if (this.startInProgress) {
      transcodeLog('coalescing duplicate transcode start');
      return this._startChain;
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
      return { ok: false, reason: 'preconditions' };
    }

    if (this.disposed) {
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
      if (!shouldAutoplay) {
        return;
      }
      const el = getVideoElement?.();
      if (!el) {
        return;
      }
      void continuePlay(el, playPrime, {
        playCall: onResumePlayback,
        onPlaying: onPlaybackStarted,
      });
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
        startPosition: startSec,
        signal: attachAbort.signal,
        onFirstBuffered: () => {
          if (!this.isCurrentAttempt(attempt)) {
            return;
          }
          // Mark the HLS session active before resuming the element. The
          // resulting `play` event must not be mistaken for a new start.
          this.active = true;
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
  }

  updatePlayback({ currentTime, paused }) {
    this.sessionController?.updatePlayback({ currentTime, paused });
  }
}
