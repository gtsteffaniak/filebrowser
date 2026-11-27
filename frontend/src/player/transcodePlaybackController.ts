import { getTranscodeSessions, type TranscodeSession, type TranscodeStatus } from "@/api/transcode.ts";
import {
  attachDeadlineMs,
  attachHlsToVideo,
  type HlsAttachment,
  type HlsFatalData,
} from "@/player/hlsAdapter.ts";
import { continuePlay, type PlayPrime } from "@/player/playbackIntent.ts";
import { errorFields, type PlayerError } from "@/player/playerErrors.ts";
import { transcodeError, transcodeLog, transcodeWarn } from "@/player/transcodeDebug.ts";
import { isTranscodeHlsMode, type TranscodeHlsMode, type TranscodeMode } from "@/player/transcodeMode.ts";
import { TranscodeSessionController } from "@/player/transcodeSession.ts";

/** A timed-out attach is retried in place rather than falling back to native playback */
const MAX_ATTACH_RETRIES = 1;

/**
 * The media-time range a session's synthetic VOD playlist can serve: the full
 * [0, durationSec]. Every session serves the on-demand playlist - a segment
 * request for an unproduced index waits inside the backend's forward gap or
 * repositions the encode job, so coverage is never narrower than the file.
 */
export function servableCoverage(
  session?: Pick<TranscodeSession, "durationSec"> | null,
): { start: number; end: number } | null {
  const end = session?.durationSec;
  if (end === undefined || !Number.isFinite(end) || end <= 0) {
    return null;
  }
  return { start: 0, end };
}

/** Everything `start()` needs; the Vue component supplies the DOM and player hooks */
export interface TranscodeStartParams {
  source?: string;
  path?: string;
  viewToken?: string;
  mode?: TranscodeMode;
  clientId?: string;
  replaceSessionId?: string;
  startSec?: number;
  /** A `play()` started in the user's gesture, awaited once the source is attached. */
  playPrime?: PlayPrime;
  getVideoElement?: () => HTMLVideoElement | null | undefined;
  nextTick?: () => Promise<void>;
  shouldAutoplay?: boolean;
  onFirstBuffered?: () => void;
  onResumePlayback?: () => void | Promise<void>;
  onPlaybackStarted?: () => void;
  onFatal?: (data: HlsFatalData) => void;
  /** Starts a replacement session after the current one broke; resolves true when it took over. */
  onRecoverPlayback?: (context: TranscodeRecoverContext) => Promise<boolean>;
  onCapacityStatus?: (status: TranscodeStatus) => void;
  onShowError?: (err: PlayerError) => void;
  /** Called instead of `onShowError` when the attach failed, so the owner can fall back to native playback. */
  onAttachNative?: (err: PlayerError) => void;
}

/** The start parameters to retry with, moved to where playback broke. */
export interface TranscodeRecoverContext extends Omit<TranscodeStartParams, "startSec"> {
  startSec: number;
  /** The session that broke, to be replaced. */
  sessionId?: string;
}

export type TranscodeStartFailureReason =
  | "preconditions"
  | "disposed"
  | "cancelled"
  | "stale"
  | "recovered"
  | "no-video"
  | "capacity"
  | "fallback"
  | "error";

export type TranscodeStartResult =
  | { ok: true; mode: TranscodeHlsMode; session: TranscodeSession; reason?: undefined; error?: undefined }
  | { ok: false; reason: TranscodeStartFailureReason; error?: PlayerError };

export interface TranscodeStopOptions {
  stopSession?: boolean;
  keepalive?: boolean;
}

/**
 * Owns one transcode playback attempt: session start, HLS attach, cancellation,
 * and stale-async guards. Vue components provide DOM/player hooks only.
 */
export class TranscodePlaybackController {
  attempt = 0;
  attachAbort: AbortController | null = null;
  hlsCleanup: (() => void) | null = null;
  sessionController: TranscodeSessionController | null = null;
  active = false;
  disposed = false;
  startInProgress = false;
  _startChain: Promise<TranscodeStartResult> = Promise.resolve({ ok: false, reason: "cancelled" });
  _startSeq = 0;

  constructor() {
    this.reset();
  }

  reset(): void {
    this.attempt = 0;
    this.attachAbort = null;
    this.hlsCleanup = null;
    this.sessionController = null;
    this.active = false;
    this.disposed = false;
    this.startInProgress = false;
    this._startChain = Promise.resolve({ ok: false, reason: "cancelled" });
    this._startSeq += 1;
  }

  dispose({ stopSession = false, keepalive = false }: TranscodeStopOptions = {}): void {
    this.disposed = true;
    this.attempt += 1;
    this._startSeq += 1;
    this._cancelInFlight({ stopSession, keepalive });
    this.active = false;
    this.startInProgress = false;
  }

  isCurrentAttempt(attempt: number): boolean {
    return !this.disposed && attempt === this.attempt;
  }

  _cancelInFlight({ stopSession = false, keepalive = false }: TranscodeStopOptions = {}): void {
    this.attachAbort?.abort();
    this.attachAbort = null;
    this.hlsCleanup?.();
    this.hlsCleanup = null;
    if (this.sessionController) {
      if (stopSession) {
        void this.sessionController.stop({ keepalive });
      } else {
        this.sessionController.detach();
      }
      this.sessionController = null;
    }
    this.active = false;
  }

  start(params: TranscodeStartParams): Promise<TranscodeStartResult> {
    // A mode switch mid-start must not reuse the old chain.
    if (this.startInProgress) {
      transcodeLog("superseding in-flight transcode start");
      this.attempt += 1;
      this.attachAbort?.abort();
    }
    this.startInProgress = true;
    const startSeq = ++this._startSeq;
    this._startChain = this._startChain
      .catch(() => {})
      .then((): Promise<TranscodeStartResult> | TranscodeStartResult => {
        if (startSeq !== this._startSeq) {
          transcodeLog("discarding queued transcode start");
          return { ok: false, reason: "cancelled" };
        }
        return this._startOnce(params);
      });
    return this._startChain;
  }

  async _startOnce(params: TranscodeStartParams): Promise<TranscodeStartResult> {
    const {
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
      onRecoverPlayback,
      onCapacityStatus,
      onShowError,
      onAttachNative,
    } = params;
    if (!source || !path || !viewToken || !isTranscodeHlsMode(mode)) {
      transcodeWarn("transcode playback start skipped — invalid preconditions", {
        source,
        path,
        mode,
        hasViewToken: Boolean(viewToken),
      });
      // start() set the flag; clearing it here keeps later starts from coalescing onto this already-settled chain.
      this.startInProgress = false;
      return { ok: false, reason: "preconditions" };
    }

    if (this.disposed) {
      this.startInProgress = false;
      return { ok: false, reason: "disposed" };
    }

    this._cancelInFlight({ stopSession: false });
    const attempt = ++this.attempt;
    const controller = new TranscodeSessionController({
      source,
      path,
      viewToken,
      profile: mode,
      clientId,
    });
    this.sessionController = controller;

    const resumePlayback = () => {
      const el = getVideoElement?.();
      transcodeLog("transcode resumePlayback", {
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
    const applyStartPosition = (video: HTMLVideoElement | null | undefined) => {
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
        transcodeLog("applied resume position after HLS attach", {
          requested: target,
          applied: clamped,
          currentTime: video.currentTime,
        });
      } catch (err) {
        transcodeWarn("failed to apply resume position", { target, message: errorFields(err as PlayerError).message });
      }
    };

    try {
      const session = await controller.start({ replaceSessionId, startSec });
      if (!this.isCurrentAttempt(attempt)) {
        transcodeLog("discarding stale session start response", {
          sessionId: session.id,
          attempt,
          currentAttempt: this.attempt,
          disposed: this.disposed,
        });
        void controller.stop();
        return { ok: false, reason: "stale" };
      }

      const el = getVideoElement?.();
      if (!el) {
        transcodeError("transcode playback aborted — no video element after session start", {
          sessionId: session.id,
        });
        void controller.stop();
        return { ok: false, reason: "no-video" };
      }

      // Avoid browser autoplay/load attempts on an empty element during HLS handoff.
      el.autoplay = false;

      if (nextTick) {
        await nextTick();
      }
      if (!this.isCurrentAttempt(attempt)) {
        void controller.stop();
        return { ok: false, reason: "stale" };
      }

      if (el.hasAttribute("src")) {
        el.removeAttribute("src");
      }

      // Every session serves the full-length on-demand playlist, so an initial
      // load must target the fragment containing the seek. Do not gate this on
      // `session.vodPlaylist` (production session responses never set it): the
      // post-buffer `applyStartPosition` seek is not a substitute, since it warms
      // up from zero and only then seeks, forcing an extra producer reposition.
      const vodStart = startSec !== undefined && Number.isFinite(startSec) && startSec > 0 ? startSec : -1;

      // One overall budget covers every attach attempt: retries share it rather
      // than each getting a fresh seek-scaled deadline, so two attempts cannot
      // consume 2x the allowed latency.
      const attachBudgetMs = attachDeadlineMs(vodStart);
      const attachDeadlineAt = performance.now() + attachBudgetMs;

      // An attach timeout is a timing race against producer warm-up, not a
      // format problem: the same session serves the fragment moments later.
      // Retry the attach a bounded number of times before giving up, so a slow
      // first fragment does not force a fallback that cannot decode the source.
      let hlsAttach: HlsAttachment | null = null;
      for (let attachAttempt = 0; attachAttempt <= MAX_ATTACH_RETRIES; attachAttempt += 1) {
        const remainingMs = attachDeadlineAt - performance.now();
        if (attachAttempt > 0 && remainingMs <= 0) {
          transcodeWarn("HLS attach retry skipped — overall attach budget exhausted", {
            sessionId: session.id,
            attachAttempt,
            attachBudgetMs,
          });
          break;
        }
        const attachAbort = new AbortController();
        this.attachAbort = attachAbort;
        if (attachAttempt > 0) {
          transcodeLog("retrying HLS attach after timeout", {
            sessionId: session.id,
            attachAttempt,
            maxRetries: MAX_ATTACH_RETRIES,
            remainingMs: Math.round(remainingMs),
          });
        }
        try {
          hlsAttach = await attachHlsToVideo({
            video: el,
            session,
            startPosition: vodStart,
            budgetMs: remainingMs,
            signal: attachAbort.signal,
            onRecoverPlayback: onRecoverPlayback
              ? ({ startSec: recoverStartSec, sessionId, shouldAutoplay: recoverAutoplay }) =>
                  onRecoverPlayback({
                    ...params,
                    startSec: recoverStartSec,
                    sessionId,
                    shouldAutoplay: recoverAutoplay ?? shouldAutoplay,
                  })
              : undefined,
            onFirstBuffered: () => {
              if (!this.isCurrentAttempt(attempt)) {
                return;
              }
              transcodeLog("transcode onFirstBuffered", {
                sessionId: session.id,
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
              this.active = false;
              onFatal?.(data);
            },
          });
          break;
        } catch (attachErr) {
          const { name } = errorFields(attachErr as PlayerError);
          const retryable =
            (name === "AttachTimeoutError" || name === "NativeHlsError") &&
            attachAttempt < MAX_ATTACH_RETRIES &&
            this.isCurrentAttempt(attempt);
          if (!retryable) {
            throw attachErr;
          }
          transcodeWarn("HLS attach failed before ready — reattaching", {
            sessionId: session.id,
            attachAttempt,
          });
          this.attachAbort = null;
        }
      }
      if (!hlsAttach) {
        throw new Error("HLS attach failed");
      }
      if (!this.isCurrentAttempt(attempt)) {
        hlsAttach.cleanup();
        return { ok: false, reason: "stale" };
      }

      this.attachAbort = null;
      this.hlsCleanup = hlsAttach.cleanup;
      this.active = true;
      applyStartPosition(getVideoElement?.());
      transcodeLog("HLS attach complete", { sessionId: session.id });

      return { ok: true, mode, session };
    } catch (caught) {
      const err = caught as PlayerError;
      const { name, message, status, type, details } = errorFields(err);
      if (name === "AbortError" || !this.isCurrentAttempt(attempt)) {
        transcodeLog("transcode playback attempt cancelled");
        return { ok: false, reason: "cancelled" };
      }
      if (name === "TranscodeRecoverError") {
        return { ok: false, reason: "recovered" };
      }

      transcodeError("transcode playback start failed", { message, status, type, details });
      this.active = false;

      if (status === 409 || status === 503) {
        try {
          const capacity = await getTranscodeSessions();
          if (this.isCurrentAttempt(attempt)) {
            onCapacityStatus?.(capacity);
          }
        } catch (fetchErr) {
          console.error("Failed to refresh transcode sessions after limit:", fetchErr);
        }
        if (!this.isCurrentAttempt(attempt)) {
          return { ok: false, reason: "cancelled" };
        }
        return { ok: false, reason: "capacity", error: err };
      }
      if (this.sessionController === controller) {
        this.sessionController = null;
      }
      void controller.stop();

      if (onAttachNative) {
        onAttachNative(err);
        return { ok: false, reason: "fallback", error: err };
      }

      onShowError?.(err);
      return { ok: false, reason: "error", error: err };
    } finally {
      if (attempt === this.attempt) {
        this.startInProgress = false;
      }
    }
  }

  teardownLocal({ stopSession = false, keepalive = false }: TranscodeStopOptions = {}): void {
    this.attempt += 1;
    this._startSeq += 1;
    this._cancelInFlight({ stopSession, keepalive });
    // Invalidate any in-flight start: its finally block compares attempts and
    // would otherwise leave the flag set, blocking every future start.
    this.startInProgress = false;
  }

  updatePlayback({ currentTime, paused }: { currentTime?: number; paused?: boolean }): void {
    this.sessionController?.updatePlayback({ currentTime, paused });
  }
}
