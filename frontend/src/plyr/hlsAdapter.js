import Hls from 'hls.js';
import { getTranscodePlaylistURL } from '@/api/transcode.js';
import { state } from '@/store/index.ts';
import { transcodeError, transcodeLog, transcodeVerbose, transcodeWarn } from '@/plyr/transcodeDebug.js';

const ATTACH_TIMEOUT_MS = 45_000;
/** Minimum forward buffer before starting playback on live transcode sessions. */
const MIN_LIVE_BUFFER_FRAGS = 2;
// Once the element is playable, start after this grace period rather than
// blocking on MIN_LIVE_BUFFER_FRAGS when production is slow.
const LIVE_BUFFER_GRACE_MS = 2_000;
/** Wait for decoded dimensions before declaring attach ready (MSE can buffer without painting). */
const RENDERABLE_WAIT_MS = 8_000;
const MAX_BUFFER_APPEND_RECOVERIES = 3;
const RECOVERABLE_HLS_DETAILS = new Set([
  'bufferSeekOverHole',
  'bufferNudgeOnStall',
  'bufferStalledError',
]);

function authHeaders() {
  const headers = {};
  if (state.sessionId) {
    headers.sessionId = state.sessionId;
  }
  return headers;
}

function applyAuthToFetchParams(initParams) {
  initParams.credentials = 'same-origin';
  const headers = authHeaders();
  if (initParams.headers instanceof Headers) {
    for (const [key, value] of Object.entries(headers)) {
      initParams.headers.set(key, value);
    }
  } else {
    initParams.headers = {
      ...(initParams.headers || {}),
      ...headers,
    };
  }
  return initParams;
}

function describeBufferedRanges(video) {
  if (!video?.buffered) {
    return [];
  }
  const ranges = [];
  for (let i = 0; i < video.buffered.length; i += 1) {
    ranges.push([video.buffered.start(i), video.buffered.end(i)]);
  }
  return ranges;
}

function describeVideo(video) {
  if (!video) {
    return null;
  }
  return {
    readyState: video.readyState,
    networkState: video.networkState,
    paused: video.paused,
    currentTime: video.currentTime,
    duration: video.duration,
    src: video.currentSrc || video.getAttribute('src') || null,
    size: `${video.videoWidth}x${video.videoHeight}`,
    buffered: describeBufferedRanges(video),
  };
}

/**
 * MSE may report buffered fragments while videoWidth is still 0. Wait briefly for
 * a decoded frame (or time advancing) before telling Plyr playback can start.
 */
export function waitUntilVideoRenderable(video, signal, maxMs = RENDERABLE_WAIT_MS) {
  return new Promise((resolve) => {
    if (!video) {
      resolve(false);
      return;
    }
    if (signal?.aborted) {
      resolve(false);
      return;
    }
    const startedAt = performance.now();
    let settled = false;
    const finish = (ok) => {
      if (settled) {
        return;
      }
      settled = true;
      video.removeEventListener('loadeddata', onProgress);
      video.removeEventListener('canplay', onProgress);
      video.removeEventListener('timeupdate', onProgress);
      resolve(ok);
    };
    const onProgress = () => {
      if (signal?.aborted) {
        finish(false);
        return;
      }
      const hasPicture = video.videoWidth > 0
        && video.videoHeight > 0
        && video.readyState >= HTMLMediaElement.HAVE_CURRENT_DATA;
      if (hasPicture) {
        finish(true);
        return;
      }
      if (performance.now() - startedAt >= maxMs) {
        transcodeWarn('waitUntilVideoRenderable timed out', describeVideo(video));
        finish(false);
      }
    };
    video.addEventListener('loadeddata', onProgress);
    video.addEventListener('canplay', onProgress);
    video.addEventListener('timeupdate', onProgress);
    onProgress();
    if (typeof video.requestVideoFrameCallback === 'function') {
      video.requestVideoFrameCallback(() => onProgress());
    }
  });
}

function describeFrag(frag) {
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

function describeHlsError(data) {
  return {
    fatal: data?.fatal,
    type: data?.type,
    details: data?.details,
    reason: data?.reason,
    response: data?.response,
    frag: describeFrag(data?.frag),
  };
}

/**
 * Pin media.duration to the probed source duration while attached — a live
 * playlist only lists produced segments, so duration would otherwise grow
 * from ~0 as ffmpeg transcodes. Returns a restore function.
 */
function pinMediaDuration(video, durationSec) {
  if (!video || !Number.isFinite(durationSec) || durationSec <= 0) {
    return null;
  }
  const real = durationSec;
  Object.defineProperty(video, 'duration', {
    configurable: true,
    get() {
      return real;
    },
  });
  return () => {
    delete video.duration;
  };
}

/**
 * Attach hls.js to a video element for a transcode session.
 * Resolves once enough media is buffered and playback can begin safely.
 *
 * go-ffmpeg v1.4.0+ publishes fMP4 with tfdt on the full-file decode timeline,
 * so the element and hls.js use source seconds directly (no timelineOffset).
 */
export function attachHlsToVideo({
  video,
  session,
  startPosition = -1,
  signal,
  onFatal,
  onFirstBuffered,
}) {
  const playlistURL = getTranscodePlaylistURL(session);
  const initialStart = Number.isFinite(startPosition) && startPosition >= 0
    ? startPosition
    : -1;

  transcodeLog('hls attach requested', {
    sessionId: session?.id,
    playlistURL,
    startPosition: initialStart,
    hlsSupported: Hls.isSupported(),
    video: describeVideo(video),
    sessionState: session?.state,
    reused: session?.reused,
  });

  if (!Hls.isSupported()) {
    if (video.canPlayType('application/vnd.apple.mpegurl')) {
      transcodeLog('hls attach using native HLS', { playlistURL });
      if (signal?.aborted) {
        const error = new Error('HLS attach aborted');
        error.name = 'AbortError';
        return Promise.reject(error);
      }
      // Native HLS has no hls.js events, so report readiness from the element.
      // onFirstBuffered marks the session active, applies the start position and
      // resumes the primed play promise; without it playback stays paused.
      const restoreDuration = session?.vodPlaylist
        ? null
        : pinMediaDuration(video, session?.durationSec);
      let reportedReady = false;
      const onReady = () => {
        if (reportedReady || signal?.aborted) {
          return;
        }
        reportedReady = true;
        onFirstBuffered?.(video);
      };
      const onAbortNative = () => {
        video.removeEventListener('loadeddata', onReady);
        video.removeEventListener('canplay', onReady);
      };
      signal?.addEventListener('abort', onAbortNative, { once: true });
      video.addEventListener('loadeddata', onReady, { once: true });
      video.addEventListener('canplay', onReady, { once: true });
      video.src = playlistURL;
      return Promise.resolve({
        cleanup: () => {
          onAbortNative();
          signal?.removeEventListener('abort', onAbortNative);
          restoreDuration?.();
          video.removeAttribute('src');
          video.load();
        },
      });
    }
    return Promise.reject(new Error('HLS not supported in this browser'));
  }
  if (signal?.aborted) {
    const error = new Error('HLS attach aborted');
    error.name = 'AbortError';
    return Promise.reject(error);
  }

  return new Promise((resolve, reject) => {
    let settled = false;
    let readyToPlay = false;
    let bufferedFrags = 0;
    const isVodPlaylist = Boolean(session?.vodPlaylist);
    const isCompletedSession = session?.state === 'completed' || session?.state === 'ready';
    const minBufferFrags = isVodPlaylist || isCompletedSession ? 1 : MIN_LIVE_BUFFER_FRAGS;
    const onAbort = () => {
      cleanup();
      const error = new Error('HLS attach aborted');
      error.name = 'AbortError';
      finish('reject', error);
    };

    const finish = (kind, payload) => {
      if (settled) {
        return;
      }
      settled = true;
      clearTimeout(timeoutId);
      signal?.removeEventListener('abort', onAbort);
      if (kind === 'resolve') {
        resolve(payload);
      } else {
        reject(payload);
      }
    };

    let graceTimerId = null;
    let appendRecoveryAttempts = 0;
    const clearGraceTimer = () => {
      if (graceTimerId !== null) {
        clearTimeout(graceTimerId);
        graceTimerId = null;
      }
    };
    const startPlaybackReady = (reason) => {
      if (settled || readyToPlay) {
        return;
      }
      readyToPlay = true;
      clearGraceTimer();
      void waitUntilVideoRenderable(video, signal).then((renderable) => {
        if (settled) {
          return;
        }
        transcodeLog('hls attach ready', {
          reason,
          renderable,
          bufferedFrags,
          minBufferFrags,
          readyState: video.readyState,
          video: describeVideo(video),
        });
        if (!renderable) {
          transcodeWarn('hls attach: no decoded video dimensions yet', {
            sessionId: session?.id,
            video: describeVideo(video),
          });
        }
        onFirstBuffered?.(video);
        finish('resolve', { cleanup, hls });
      });
    };
    const onVideoPlayable = () => scheduleGraceResolve();
    const scheduleGraceResolve = () => {
      if (settled || readyToPlay || graceTimerId !== null || bufferedFrags < 1) {
        return;
      }
      const playable = video.readyState >= HTMLMediaElement.HAVE_FUTURE_DATA
        || video.buffered?.length > 0;
      if (!playable) {
        return;
      }
      graceTimerId = setTimeout(() => {
        graceTimerId = null;
        if (bufferedFrags < 1) {
          return;
        }
        startPlaybackReady('live-buffer-grace');
      }, LIVE_BUFFER_GRACE_MS);
    };

    const timeoutId = setTimeout(() => {
      transcodeError('hls attach timed out waiting for buffered fragments', {
        sessionId: session?.id,
        playlistURL,
        startPosition: initialStart,
        video: describeVideo(video),
      });
      cleanup();
      finish('reject', new Error('HLS attach timed out'));
    }, ATTACH_TIMEOUT_MS);

    // Pin duration only for live-paced sessions (open-ended playlist →
    // MediaSource.duration is Infinity). On a completed VOD playlist a pinned
    // finite value makes hls.js skip its own duration update, leaving
    // seekable empty and silently discarding every seek.
    const restoreDuration = isVodPlaylist || isCompletedSession
      ? null
      : pinMediaDuration(video, session?.durationSec);
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
      xhrSetup(xhr, url) {
        xhr.withCredentials = true;
        for (const [key, value] of Object.entries(authHeaders())) {
          xhr.setRequestHeader(key, value);
        }
        transcodeVerbose('hls xhr open', { url });
      },
      fetchSetup(context, initParams) {
        transcodeVerbose('hls fetch setup', { url: context?.url });
        applyAuthToFetchParams(initParams);
        return new Request(context.url, initParams);
      },
    });

    const cleanup = () => {
      transcodeLog('hls destroy', { sessionId: session?.id });
      clearGraceTimer();
      restoreDuration?.();
      video.removeEventListener('canplay', onVideoPlayable);
      video.removeEventListener('loadeddata', onVideoPlayable);
      hls.destroy();
    };

    signal?.addEventListener('abort', onAbort, { once: true });
    if (signal?.aborted) {
      onAbort();
      return;
    }

    hls.on(Hls.Events.MEDIA_ATTACHED, () => {
      transcodeLog('hls media attached', { video: describeVideo(video) });
    });

    hls.on(Hls.Events.MANIFEST_LOADING, (_event, data) => {
      transcodeLog('hls manifest loading', { url: data?.url });
    });

    hls.on(Hls.Events.MANIFEST_LOADED, (_event, data) => {
      transcodeLog('hls manifest loaded', {
        levels: data?.levels?.length,
        firstLevel: data?.firstLevel,
        url: data?.url,
      });
    });

    hls.on(Hls.Events.MANIFEST_PARSED, (_event, data) => {
      transcodeLog('hls manifest parsed — calling startLoad', {
        levels: data?.levels?.length,
        startPosition: initialStart,
      });
      hls.startLoad(initialStart >= 0 ? initialStart : -1);
    });

    hls.on(Hls.Events.LEVEL_LOADING, (_event, data) => {
      transcodeVerbose('hls level loading', { level: data?.level, url: data?.url });
    });

    hls.on(Hls.Events.FRAG_LOADING, (_event, data) => {
      transcodeLog('hls frag loading', { frag: describeFrag(data?.frag) });
    });

    hls.on(Hls.Events.FRAG_LOADED, (_event, data) => {
      transcodeLog('hls frag loaded', {
        frag: describeFrag(data?.frag),
        bytes: data?.frag?.stats?.total,
      });
    });

    hls.on(Hls.Events.FRAG_BUFFERED, (_event, data) => {
      bufferedFrags += 1;
      const fragInfo = describeFrag(data?.frag);
      transcodeLog('hls frag buffered', {
        frag: fragInfo,
        video: describeVideo(video),
        bufferedFrags,
        minBufferFrags,
        sessionStartSec: session?.startSec,
        // If frag.start ≈ 0 while sessionStartSec is large, hls.js is on playlist-local
        // time and absolute startSec math in the player will be wrong.
        timelineCheck: bufferedFrags === 1 ? {
          fragStart: fragInfo?.start,
          videoCurrentTime: video?.currentTime,
          sessionStartSec: session?.startSec,
        } : undefined,
      });
      if (readyToPlay) {
        return;
      }
      if (bufferedFrags >= minBufferFrags) {
        startPlaybackReady('buffer-satisfied');
        return;
      }
      scheduleGraceResolve();
    });

    hls.on(Hls.Events.ERROR, (_event, data) => {
      const info = {
        ...describeHlsError(data),
        video: describeVideo(video),
      };
      if (data?.fatal) {
        transcodeError('hls fatal error', info);
        cleanup();
        onFatal?.(data);
        finish('reject', data);
        return;
      }
      if (data?.details === 'bufferAppendNoProgress') {
        transcodeWarn('hls bufferAppendNoProgress', info);
        if (appendRecoveryAttempts < MAX_BUFFER_APPEND_RECOVERIES) {
          appendRecoveryAttempts += 1;
          try {
            hls.recoverMediaError();
            transcodeLog('hls recoverMediaError after bufferAppendNoProgress', {
              attempt: appendRecoveryAttempts,
            });
          } catch (err) {
            transcodeWarn('hls recoverMediaError failed', { message: err?.message });
          }
        }
        return;
      }
      if (RECOVERABLE_HLS_DETAILS.has(data?.details)) {
        transcodeVerbose('hls non-fatal media recovery', info);
        return;
      }
      // Init/segment 503 during startup is common; recover before MSE requires reset.
      if (data?.type === Hls.ErrorTypes.NETWORK_ERROR && data?.response?.code === 503) {
        transcodeWarn('hls 503 during transcode startup — retrying load', info);
        try {
          hls.startLoad(hls.liveSyncPosition ?? -1);
        } catch (err) {
          transcodeWarn('hls startLoad retry after 503 failed', { message: err?.message });
        }
        return;
      }
      if (data?.details === 'mediaSourceRequiresReset' && appendRecoveryAttempts < MAX_BUFFER_APPEND_RECOVERIES) {
        appendRecoveryAttempts += 1;
        transcodeWarn('hls mediaSourceRequiresReset — recoverMediaError', info);
        try {
          hls.recoverMediaError();
        } catch (err) {
          transcodeWarn('hls recoverMediaError failed', { message: err?.message });
        }
        return;
      }
      transcodeWarn('hls non-fatal error', info);
    });

    video.addEventListener('canplay', onVideoPlayable);
    video.addEventListener('loadeddata', onVideoPlayable);

    transcodeLog('hls attachMedia + loadSource', { playlistURL });
    hls.attachMedia(video);
    hls.loadSource(playlistURL);
  });
}

export { Hls };
