import Hls, { type ErrorData, type Fragment } from "hls.js";
import { getTranscodePlaylistURL, type TranscodeSession } from "@/api/transcode.ts";
import {
  AttachAbortError,
  AttachTimeoutError,
  errorMessage,
  NativeHlsError,
  type PlayerError,
  TranscodeRecoverError,
} from "@/player/playerErrors.ts";
import { transcodeError, transcodeLog, transcodeVerbose, transcodeWarn } from "@/player/transcoding/transcodeDebug.ts";
import { state } from "@/store/index.ts";

const ATTACH_TIMEOUT_MS = 45_000;
/** Wait for decoded dimensions before declaring attach ready (MSE can buffer without painting). */
const RENDERABLE_WAIT_MS = 8_000;
const MAX_BUFFER_APPEND_RECOVERIES = 3;
/** A mid-file start has to wait for the producer to seek, warm up and publish the first fragment that covers `startPosition`. */
const SEEK_WARMUP_MS_PER_SEC = 1_000;
const MAX_SEEK_WARMUP_MS = 45_000;
/**
 * A stall guard for a producer that stops making progress: every fragment that
 * actually arrives (or buffers) restarts this clock, so a slow-but-progressing
 * attach is never aborted by the wall-clock deadline alone.
 */
const ATTACH_STALL_TIMEOUT_MS = 30_000;
const FRAG_FIRST_BYTE_TIMEOUT_MS = 45_000;
const ATTACH_STARTUP_STALL_TIMEOUT_MS = FRAG_FIRST_BYTE_TIMEOUT_MS;
const FRAG_MAX_LOAD_MS = 120_000;

export interface HlsAttachment {
  cleanup: () => void;
  hls?: Hls;
}

/** Fatal failure reported when the browser native HLS breaks after it was ready */
export interface NativeHlsFatalData {
  fatal: true;
  type: "mediaError";
  details: "nativeHlsError";
  code: number | undefined;
  reason: string;
}

export type HlsFatalData = ErrorData | NativeHlsFatalData;

/** The attachment broke mid-playback */
export interface HlsRecoverRequest {
  startSec: number;
  sessionId?: string;
  shouldAutoplay?: boolean;
}

/** Resolves true when the replacement session took over */
export type HlsRecoverHandler = (request: HlsRecoverRequest) => Promise<boolean>;

export interface AttachHlsOptions {
  video: HTMLVideoElement;
  session: Pick<TranscodeSession, "id" | "state" | "reused" | "deliveryBaseUrl">;
  startPosition?: number;
  signal?: AbortSignal;
  onFatal?: (data: HlsFatalData) => void;
  onRecoverPlayback?: HlsRecoverHandler;
  onFirstBuffered?: (video: HTMLVideoElement) => void;
  budgetMs?: number;
}

/**
 * Total attach budget for a start position: the base deadline plus a bounded
 * allowance for the seek/producer warm-up the first fragment needs.
 */
export function attachDeadlineMs(startPosition?: number): number {
  const seek = startPosition !== undefined && Number.isFinite(startPosition) && startPosition > 0 ? startPosition : 0;
  const warmup = Math.min(seek * SEEK_WARMUP_MS_PER_SEC, MAX_SEEK_WARMUP_MS);
  return ATTACH_TIMEOUT_MS + warmup;
}

/**
 * Resolve the hard-attach deadline for one attempt. An explicit `budgetMs`  wins when finite and positive,
 * otherwise fall back to the seek-scaled default for the start position.
 */
function resolveAttachDeadlineMs(startPosition: number, budgetMs?: number): number {
  if (budgetMs !== undefined && Number.isFinite(budgetMs) && budgetMs > 0) {
    return budgetMs;
  }
  return attachDeadlineMs(startPosition);
}

const RECOVERABLE_HLS_DETAILS = new Set<string>(["bufferSeekOverHole", "bufferNudgeOnStall", "bufferStalledError"]);

function authHeaders(): Record<string, string> {
  const headers: Record<string, string> = {};
  if (state.sessionId) {
    headers.sessionId = state.sessionId;
  }
  return headers;
}

function applyAuthToFetchParams(initParams: RequestInit): RequestInit {
  initParams.credentials = "same-origin";
  const headers = new Headers(initParams.headers);
  for (const [key, value] of Object.entries(authHeaders())) {
    headers.set(key, value);
  }
  initParams.headers = headers;
  return initParams;
}

function describeBufferedRanges(video: HTMLMediaElement): Array<[number, number]> {
  if (!video.buffered) {
    return [];
  }
  const ranges: Array<[number, number]> = [];
  for (let i = 0; i < video.buffered.length; i += 1) {
    ranges.push([video.buffered.start(i), video.buffered.end(i)]);
  }
  return ranges;
}

function describeVideo(video?: HTMLVideoElement | null) {
  if (!video) {
    return null;
  }
  return {
    readyState: video.readyState,
    networkState: video.networkState,
    paused: video.paused,
    currentTime: video.currentTime,
    duration: video.duration,
    src: video.currentSrc || video.getAttribute("src") || null,
    size: `${video.videoWidth}x${video.videoHeight}`,
    buffered: describeBufferedRanges(video),
  };
}

/**
 * MSE may report buffered fragments while videoWidth is still 0. Wait briefly for
 * a decoded frame (or time advancing) before telling the player playback can start.
 */
export function waitUntilVideoRenderable(
  video: HTMLVideoElement | null | undefined,
  signal?: AbortSignal,
  maxMs = RENDERABLE_WAIT_MS,
): Promise<boolean> {
  return new Promise((resolve) => {
    if (!video || signal?.aborted) {
      resolve(false);
      return;
    }
    const hasPicture = () =>
      video.videoWidth > 0 && video.videoHeight > 0 && video.readyState >= HTMLMediaElement.HAVE_CURRENT_DATA;
    if (hasPicture()) {
      resolve(true);
      return;
    }

    let settled = false;
    let timeoutId: ReturnType<typeof setTimeout> | undefined;
    let frameHandle: number | null = null;
    const finish = (ok: boolean) => {
      if (settled) {
        return;
      }
      settled = true;
      clearTimeout(timeoutId);
      signal?.removeEventListener("abort", onAbort);
      video.removeEventListener("loadeddata", onProgress);
      video.removeEventListener("canplay", onProgress);
      video.removeEventListener("timeupdate", onProgress);
      if (frameHandle !== null && typeof video.cancelVideoFrameCallback === "function") {
        video.cancelVideoFrameCallback(frameHandle);
      }
      frameHandle = null;
      resolve(ok);
    };
    const onAbort = () => finish(false);
    const onTimeout = () => {
      transcodeWarn("waitUntilVideoRenderable timed out", describeVideo(video));
      finish(false);
    };
    const onProgress = () => {
      if (signal?.aborted) {
        finish(false);
        return;
      }
      if (hasPicture()) {
        finish(true);
      }
    };

    signal?.addEventListener("abort", onAbort, { once: true });
    video.addEventListener("loadeddata", onProgress);
    video.addEventListener("canplay", onProgress);
    video.addEventListener("timeupdate", onProgress);
    timeoutId = setTimeout(onTimeout, maxMs);
    if (typeof video.requestVideoFrameCallback === "function") {
      frameHandle = video.requestVideoFrameCallback(() => {
        frameHandle = null;
        onProgress();
      });
    }
  });
}

function describeFrag(frag?: Fragment | null) {
  if (!frag) {
    return null;
  }
  return {
    sn: frag.sn,
    type: frag.type,
    start: frag.start,
    duration: frag.duration,
    url: frag.url,
  };
}

function describeHlsError(data: ErrorData) {
  return {
    fatal: data.fatal,
    type: data.type,
    details: data.details,
    reason: data.reason,
    response: data.response,
    frag: describeFrag(data.frag),
  };
}

interface NativeAttachOptions {
  video: HTMLVideoElement;
  playlistURL: string;
  initialStart: number;
  signal?: AbortSignal;
  onFirstBuffered?: (video: HTMLVideoElement) => void;
  budgetMs?: number;
  sessionId: string;
  onFatal?: (data: HlsFatalData) => void;
  onRecoverPlayback?: HlsRecoverHandler;
}

function attachNativeHls({
  video,
  playlistURL,
  initialStart,
  signal,
  onFirstBuffered,
  budgetMs,
  sessionId,
  onFatal,
  onRecoverPlayback,
}: NativeAttachOptions): Promise<HlsAttachment> {
  return new Promise((resolve, reject) => {
    const deadlineMs = resolveAttachDeadlineMs(initialStart, budgetMs);
    let settled = false;
    let hasPlayed = false;
    let timeoutId: ReturnType<typeof setTimeout> | undefined;
    const originalPreload = video.getAttribute("preload");

    const removeListeners = () => {
      clearTimeout(timeoutId);
      signal?.removeEventListener("abort", onAbort);
      video.removeEventListener("loadeddata", onReady);
      video.removeEventListener("canplay", onReady);
    };
    const releaseSource = () => {
      video.removeAttribute("src");
      if (originalPreload === null) {
        video.removeAttribute("preload");
      } else {
        video.setAttribute("preload", originalPreload);
      }
      video.load();
    };
    const cleanup = () => {
      removeListeners();
      video.removeEventListener("error", onError);
      video.removeEventListener("playing", onPlaying);
      releaseSource();
    };
    const fail = (error: Error) => {
      if (settled) {
        return;
      }
      settled = true;
      cleanup();
      reject(error);
    };

    function onAbort() {
      fail(new AttachAbortError());
    }
    function onError() {
      const mediaError = video.error;
      transcodeError("native hls playback error", {
        playlistURL,
        code: mediaError?.code,
        message: mediaError?.message,
        video: describeVideo(video),
      });
      const error = new NativeHlsError(mediaError?.message || "Native HLS playback failed", mediaError?.code);
      if (settled) {
        reportPostReadyError(error);
        return;
      }
      fail(error);
    }
    function reportPostReadyError(error: NativeHlsError) {
      const data: NativeHlsFatalData = {
        fatal: true,
        type: "mediaError",
        details: "nativeHlsError",
        code: error.code,
        reason: error.message,
      };
      const startSec = Number.isFinite(video.currentTime) ? video.currentTime : 0;
      const shouldAutoplay = hasPlayed ? !video.paused && !video.ended : undefined;
      cleanup();
      if (error.code === MediaError.MEDIA_ERR_NETWORK && onRecoverPlayback) {
        void onRecoverPlayback({ startSec, sessionId, shouldAutoplay })
          .then((ok) => {
            if (!ok) {
              onFatal?.(data);
            }
          })
          .catch(() => {
            onFatal?.(data);
          });
        return;
      }
      onFatal?.(data);
    }
    function onPlaying() {
      hasPlayed = true;
    }
    function onTimeout() {
      transcodeError("native hls attach timed out waiting for readiness", {
        playlistURL,
        deadlineMs,
        video: describeVideo(video),
      });
      fail(new AttachTimeoutError());
    }
    function onReady() {
      if (settled) {
        return;
      }
      if (signal?.aborted) {
        onAbort();
        return;
      }
      settled = true;
      removeListeners();
      try {
        onFirstBuffered?.(video);
      } finally {
        resolve({ cleanup });
      }
    }

    signal?.addEventListener("abort", onAbort, { once: true });
    video.addEventListener("loadeddata", onReady);
    video.addEventListener("canplay", onReady);
    video.addEventListener("error", onError);
    video.addEventListener("playing", onPlaying);
    timeoutId = setTimeout(onTimeout, deadlineMs);
    video.preload = "auto";
    video.src = playlistURL;
    video.load();
  });
}

/**
 * Attach hls.js to a video element for a transcode session.
 * Resolves once enough media is buffered and playback can begin safely.
 *
 * The backend serves a synthetic full-length VOD playlist; fragments carry
 * tfdt on that playlist-local timeline (-output_ts_offset), so hls.js places
 * them by playlist position and the element uses source seconds directly.
 */
export function attachHlsToVideo({
  video,
  session,
  startPosition = -1,
  signal,
  onFatal,
  onRecoverPlayback,
  onFirstBuffered,
  budgetMs,
}: AttachHlsOptions): Promise<HlsAttachment> {
  const playlistURL = getTranscodePlaylistURL(session);
  const initialStart = Number.isFinite(startPosition) && startPosition >= 0 ? startPosition : -1;

  transcodeLog("hls attach requested", {
    sessionId: session.id,
    playlistURL,
    startPosition: initialStart,
    hlsSupported: Hls.isSupported(),
    video: describeVideo(video),
    sessionState: session.state,
    reused: session.reused,
  });

  if (!Hls.isSupported()) {
    if (video.canPlayType("application/vnd.apple.mpegurl")) {
      transcodeLog("hls attach using native HLS", { playlistURL });
      if (signal?.aborted) {
        return Promise.reject(new AttachAbortError());
      }
      return attachNativeHls({
        video,
        playlistURL,
        initialStart,
        signal,
        onFirstBuffered,
        budgetMs,
        sessionId: session.id,
        onFatal,
        onRecoverPlayback,
      });
    }
    return Promise.reject(new Error("HLS not supported in this browser"));
  }
  if (signal?.aborted) {
    return Promise.reject(new AttachAbortError());
  }

  return new Promise<HlsAttachment>((resolve, reject) => {
    let settled = false;
    let readyToPlay = false;
    let startApplied = false;
    let playbackBegan = false;
    const onPlaying = () => {
      playbackBegan = true;
    };
    video.addEventListener("playing", onPlaying);
    let bufferedFrags = 0;
    // The synthetic VOD playlist lists the full media duration; a single
    // buffered fragment is enough to start playback.
    const minBufferFrags = 1;
    let startup503Retries = 0;
    let last503FragSn: Fragment["sn"] | null = null;
    let destroyed = false;

    const tryRecoverPlayback = (data: ErrorData, reason: string): boolean => {
      if (!onRecoverPlayback || destroyed) {
        return false;
      }
      const playbackStarted = startApplied;
      const currentTime = video.currentTime;
      const fragStart = data.frag?.start;
      const resumeAt =
        playbackStarted && Number.isFinite(currentTime)
          ? currentTime
          : fragStart !== undefined && Number.isFinite(fragStart)
            ? fragStart
            : Math.max(initialStart, 0);
      const shouldAutoplay = playbackBegan ? !video.paused && !video.ended : undefined;
      transcodeWarn("hls requesting transcode session recovery", {
        reason,
        resumeAt,
        sessionId: session.id,
        responseCode: data.response?.code,
      });
      cleanup();
      void onRecoverPlayback({ startSec: resumeAt, sessionId: session.id, shouldAutoplay })
        .then((ok) => {
          if (!ok) {
            onFatal?.(data);
          }
        })
        .catch(() => {
          onFatal?.(data);
        });
      rejectOnce(new TranscodeRecoverError());
      return true;
    };

    const onAbort = () => {
      cleanup();
      rejectOnce(new AttachAbortError());
    };

    // Two independent guards, both funnelling into the same timeout handler:
    // - hardTimeoutId: the absolute budget (seek-scaled) for this attach.
    // - stallTimeoutId: reset by every fragment that arrives/buffers, so a slow
    //   producer that keeps publishing is not killed by the wall clock alone.
    const attachStartedAt = performance.now();
    const deadlineMs = resolveAttachDeadlineMs(initialStart, budgetMs);
    let hardTimeoutId: ReturnType<typeof setTimeout> | undefined;
    let stallTimeoutId: ReturnType<typeof setTimeout> | undefined;
    let fragmentProgressed = false;
    let awaitingFirstByte = false;

    const settle = (): boolean => {
      if (settled) {
        return false;
      }
      settled = true;
      clearTimeout(hardTimeoutId);
      clearTimeout(stallTimeoutId);
      signal?.removeEventListener("abort", onAbort);
      return true;
    };
    const resolveOnce = (attachment: HlsAttachment) => {
      if (settle()) {
        resolve(attachment);
      }
    };
    const rejectOnce = (reason: PlayerError) => {
      if (settle()) {
        reject(reason);
      }
    };

    const onAttachTimeout = () => {
      transcodeError("hls attach timed out waiting for buffered fragments", {
        sessionId: session.id,
        playlistURL,
        startPosition: initialStart,
        deadlineMs,
        elapsedMs: Math.round(performance.now() - attachStartedAt),
        bufferedFrags,
        video: describeVideo(video),
      });
      cleanup();
      rejectOnce(new AttachTimeoutError());
    };

    // Progress restarts the stall guard, clamped so it can never outlive the hard deadline.
    const noteAttachProgress = () => {
      if (settled) {
        return;
      }
      clearTimeout(stallTimeoutId);
      const remaining = deadlineMs - (performance.now() - attachStartedAt);
      if (remaining <= 0) {
        return;
      }
      let limitMs = ATTACH_STARTUP_STALL_TIMEOUT_MS;
      if (awaitingFirstByte) {
        limitMs = FRAG_FIRST_BYTE_TIMEOUT_MS;
      } else if (fragmentProgressed) {
        limitMs = ATTACH_STALL_TIMEOUT_MS;
      }
      stallTimeoutId = setTimeout(onAttachTimeout, Math.min(limitMs, remaining));
    };
    const noteFragmentResponseStarted = () => {
      if (settled) {
        return;
      }
      awaitingFirstByte = false;
      clearTimeout(stallTimeoutId);
      const remaining = deadlineMs - (performance.now() - attachStartedAt);
      if (remaining <= 0) {
        return;
      }
      stallTimeoutId = setTimeout(onAttachTimeout, Math.min(FRAG_MAX_LOAD_MS, remaining));
    };

    let appendRecoveryAttempts = 0;
    const startPlaybackReady = (reason: string) => {
      if (settled || readyToPlay) {
        return;
      }
      readyToPlay = true;
      void waitUntilVideoRenderable(video, signal).then((renderable) => {
        if (settled) {
          return;
        }
        transcodeLog("hls attach ready", {
          reason,
          renderable,
          bufferedFrags,
          minBufferFrags,
          readyState: video.readyState,
          video: describeVideo(video),
        });
        if (!renderable) {
          transcodeWarn("hls attach: no decoded video dimensions yet", {
            sessionId: session.id,
            video: describeVideo(video),
          });
        }
        onFirstBuffered?.(video);
        startApplied = true;
        resolveOnce({ cleanup, hls });
      });
    };

    hardTimeoutId = setTimeout(onAttachTimeout, deadlineMs);
    noteAttachProgress();

    const hls = new Hls({
      enableWorker: true,
      lowLatencyMode: false,
      backBufferLength: 30,
      maxBufferLength: 30,
      maxMaxBufferLength: 60,
      startPosition: initialStart,
      manifestLoadPolicy: {
        default: {
          maxTimeToFirstByteMs: 60_000,
          maxLoadTimeMs: 60_000,
          timeoutRetry: {
            maxNumRetry: 4,
            retryDelayMs: 2000,
            maxRetryDelayMs: 8000,
          },
          errorRetry: {
            maxNumRetry: 8,
            retryDelayMs: 2000,
            maxRetryDelayMs: 8000,
          },
        },
      },
      fragLoadPolicy: {
        default: {
          // Segments are produced on demand, so a seek or a producer restart
          // (hardware -> software decode retry) can keep the first byte waiting
          // well past hls.js 10s default.
          maxTimeToFirstByteMs: FRAG_FIRST_BYTE_TIMEOUT_MS,
          maxLoadTimeMs: FRAG_MAX_LOAD_MS,
          timeoutRetry: {
            maxNumRetry: 4,
            retryDelayMs: 1000,
            maxRetryDelayMs: 8000,
          },
          errorRetry: {
            maxNumRetry: 6,
            retryDelayMs: 1000,
            maxRetryDelayMs: 8000,
          },
        },
      },
      xhrSetup(xhr: XMLHttpRequest, url: string) {
        xhr.withCredentials = true;
        for (const [key, value] of Object.entries(authHeaders())) {
          xhr.setRequestHeader(key, value);
        }
        transcodeVerbose("hls xhr open", { url });
        if (!/\.m3u8(\?|$)/.test(url)) {
          xhr.addEventListener("readystatechange", () => {
            if (xhr.readyState === XMLHttpRequest.HEADERS_RECEIVED && xhr.status >= 200 && xhr.status < 300) {
              noteFragmentResponseStarted();
            }
          });
        }
      },
      fetchSetup(context, initParams: RequestInit) {
        transcodeVerbose("hls fetch setup", { url: context.url });
        applyAuthToFetchParams(initParams);
        return new Request(context.url, initParams);
      },
    });

    const cleanup = () => {
      if (destroyed) return;
      destroyed = true;
      video.removeEventListener("playing", onPlaying);
      transcodeLog("hls destroy", { sessionId: session.id });
      try {
        hls.stopLoad();
      } catch (_) {
        // ignore
      }
      try {
        hls.detachMedia();
      } catch (_) {
        // ignore
      }
      hls.destroy();
    };

    signal?.addEventListener("abort", onAbort, { once: true });
    if (signal?.aborted) {
      onAbort();
      return;
    }

    hls.on(Hls.Events.MEDIA_ATTACHED, () => {
      transcodeLog("hls media attached", { video: describeVideo(video) });
    });

    hls.on(Hls.Events.MANIFEST_LOADING, (_event, data) => {
      transcodeLog("hls manifest loading", { url: data.url });
    });

    hls.on(Hls.Events.MANIFEST_LOADED, (_event, data) => {
      transcodeLog("hls manifest loaded", {
        levels: data.levels.length,
        url: data.url,
      });
    });

    hls.on(Hls.Events.MANIFEST_PARSED, (_event, data) => {
      transcodeLog("hls manifest parsed — calling startLoad", {
        levels: data.levels.length,
        startPosition: initialStart,
      });
      hls.startLoad(initialStart >= 0 ? initialStart : -1);
    });

    hls.on(Hls.Events.LEVEL_LOADING, (_event, data) => {
      transcodeVerbose("hls level loading", { level: data.level, url: data.url });
    });

    hls.on(Hls.Events.FRAG_LOADING, (_event, data) => {
      transcodeLog("hls frag loading", { frag: describeFrag(data.frag) });
      awaitingFirstByte = true;
      noteAttachProgress();
    });

    hls.on(Hls.Events.FRAG_LOADED, (_event, data) => {
      fragmentProgressed = true;
      awaitingFirstByte = false;
      noteAttachProgress();
      transcodeLog("hls frag loaded", {
        frag: describeFrag(data.frag),
        bytes: data.frag.stats.total,
      });
    });

    hls.on(Hls.Events.FRAG_BUFFERED, (_event, data) => {
      bufferedFrags += 1;
      fragmentProgressed = true;
      noteAttachProgress();
      transcodeLog("hls frag buffered", {
        frag: describeFrag(data.frag),
        video: describeVideo(video),
        bufferedFrags,
        minBufferFrags,
      });
      if (readyToPlay) {
        return;
      }
      if (bufferedFrags >= minBufferFrags) {
        startPlaybackReady("buffer-satisfied");
      }
    });

    hls.on(Hls.Events.ERROR, (_event, data) => {
      if (destroyed) return;
      const info = {
        ...describeHlsError(data),
        video: describeVideo(video),
      };
      const details: string = data.details;
      const type: string = data.type;
      const responseCode = data.response?.code;
      if (data.fatal) {
        transcodeError("hls fatal error", info);
        if (responseCode === 503 || responseCode === 409) {
          if (tryRecoverPlayback(data, "fatal-network")) {
            return;
          }
        }
        cleanup();
        onFatal?.(data);
        rejectOnce(data);
        return;
      }
      if (details === "bufferAppendNoProgress") {
        transcodeWarn("hls bufferAppendNoProgress", info);
        if (appendRecoveryAttempts < MAX_BUFFER_APPEND_RECOVERIES) {
          appendRecoveryAttempts += 1;
          try {
            hls.recoverMediaError();
            transcodeLog("hls recoverMediaError after bufferAppendNoProgress", {
              attempt: appendRecoveryAttempts,
            });
          } catch (err) {
            transcodeWarn("hls recoverMediaError failed", { message: errorMessage(err as PlayerError) });
          }
        }
        return;
      }
      if (RECOVERABLE_HLS_DETAILS.has(details)) {
        transcodeVerbose("hls non-fatal media recovery", info);
        return;
      }
      if (type === Hls.ErrorTypes.NETWORK_ERROR && responseCode === 409) {
        tryRecoverPlayback(data, "409-needs-restart");
        return;
      }
      // Init/segment 503 during startup is common; recover before MSE requires reset.
      if (type === Hls.ErrorTypes.NETWORK_ERROR && responseCode === 503) {
        const fragSn = data.frag?.sn ?? null;
        if (fragSn === last503FragSn) {
          startup503Retries += 1;
        } else {
          last503FragSn = fragSn;
          startup503Retries = 1;
        }
        if (startup503Retries >= 6) {
          if (tryRecoverPlayback(data, "503-exhausted")) {
            return;
          }
        }
        transcodeWarn("hls 503 during transcode startup — retrying load", info);
        // Resume at the fragment we were already waiting on. Do NOT use
        // hls.liveSyncPosition here: this is a #EXT-X-PLAYLIST-TYPE:VOD playlist
        // with #EXT-X-ENDLIST, so liveSyncPosition is the live-edge guess
        // (edge - liveSyncDurationCount*targetduration, e.g. 18.6s for this
        // 30.6s clip) — it jumps the playhead far ahead of the requested seek
        // and abandons the not-yet-ready fragment hls.js was already retrying.
        const fragStart = data.frag?.start;
        const resumeAt = fragStart !== undefined && Number.isFinite(fragStart) ? fragStart : hls.startPosition;
        try {
          hls.startLoad(Number.isFinite(resumeAt) && resumeAt > 0 ? resumeAt : -1);
        } catch (err) {
          transcodeWarn("hls startLoad retry after 503 failed", { message: errorMessage(err as PlayerError) });
        }
        return;
      }
      if (details === "mediaSourceRequiresReset" && appendRecoveryAttempts < MAX_BUFFER_APPEND_RECOVERIES) {
        appendRecoveryAttempts += 1;
        transcodeWarn("hls mediaSourceRequiresReset — recoverMediaError", info);
        try {
          hls.recoverMediaError();
        } catch (err) {
          transcodeWarn("hls recoverMediaError failed", { message: errorMessage(err as PlayerError) });
        }
        return;
      }
      transcodeWarn("hls non-fatal error", info);
    });

    transcodeLog("hls attachMedia + loadSource", { playlistURL });
    hls.attachMedia(video);
    hls.loadSource(playlistURL);
  });
}

export { Hls };
