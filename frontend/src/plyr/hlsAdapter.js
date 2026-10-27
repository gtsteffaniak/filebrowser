import Hls from 'hls.js';
import { getTranscodePlaylistURL } from '@/api/transcode.js';
import { state } from '@/store/index.ts';
import { transcodeError, transcodeLog, transcodeVerbose, transcodeWarn } from '@/plyr/transcodeDebug.js';

const ATTACH_TIMEOUT_MS = 45_000;
/** Wait for decoded dimensions before declaring attach ready (MSE can buffer without painting). */
const RENDERABLE_WAIT_MS = 8_000;
const MAX_BUFFER_APPEND_RECOVERIES = 3;
/**
 * A mid-file start has to wait for the producer to seek, warm up and publish the
 * first fragment that covers `startPosition`. The base attach budget is measured
 * from attach time and knows nothing about how far into the file the seek lands,
 * so a deep resume (e.g. datasaver resuming at ~10.5s) shares the same 45s as a
 * start at 0s even though its first usable fragment is produced later. Scale the
 * budget by the seek offset, bounded, so a deep seek is not killed while the
 * producer is still working.
 */
const SEEK_WARMUP_MS_PER_SEC = 1_000;
const MAX_SEEK_WARMUP_MS = 45_000;
/**
 * A stall guard for a producer that stops making progress: every fragment that
 * actually arrives (or buffers) restarts this clock, so a slow-but-progressing
 * attach is never aborted by the wall-clock deadline alone.
 */
const ATTACH_STALL_TIMEOUT_MS = 30_000;

/**
 * Total attach budget for a start position: the base deadline plus a bounded
 * allowance for the seek/producer warm-up the first fragment needs.
 */
export function attachDeadlineMs(startPosition) {
  const seek = Number.isFinite(startPosition) && startPosition > 0 ? startPosition : 0;
  const warmup = Math.min(seek * SEEK_WARMUP_MS_PER_SEC, MAX_SEEK_WARMUP_MS);
  return ATTACH_TIMEOUT_MS + warmup;
}

/**
 * Resolve the hard-attach deadline for one attempt. An explicit `budgetMs`
 * (a shared, already-decremented retry budget) wins when finite and positive;
 * otherwise fall back to the seek-scaled default for the start position.
 */
function resolveAttachDeadlineMs(startPosition, budgetMs) {
  if (Number.isFinite(budgetMs) && budgetMs > 0) {
    return budgetMs;
  }
  return attachDeadlineMs(startPosition);
}

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
  onFirstBuffered,
  budgetMs,
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
    // The synthetic VOD playlist lists the full media duration; a single
    // buffered fragment is enough to start playback.
    const minBufferFrags = 1;
    const onAbort = () => {
      cleanup();
      const error = new Error('HLS attach aborted');
      error.name = 'AbortError';
      finish('reject', error);
    };

    // Two independent guards, both funnelling into the same timeout handler:
    // - hardTimeoutId: the absolute budget (seek-scaled) for this attach.
    // - stallTimeoutId: reset by every fragment that arrives/buffers, so a slow
    //   producer that keeps publishing is not killed by the wall clock alone.
    const attachStartedAt = performance.now();
    const deadlineMs = resolveAttachDeadlineMs(initialStart, budgetMs);
    let hardTimeoutId = null;
    let stallTimeoutId = null;

    const finish = (kind, payload) => {
      if (settled) {
        return;
      }
      settled = true;
      clearTimeout(hardTimeoutId);
      clearTimeout(stallTimeoutId);
      signal?.removeEventListener('abort', onAbort);
      if (kind === 'resolve') {
        resolve(payload);
      } else {
        reject(payload);
      }
    };

    const onAttachTimeout = () => {
      transcodeError('hls attach timed out waiting for buffered fragments', {
        sessionId: session?.id,
        playlistURL,
        startPosition: initialStart,
        deadlineMs,
        elapsedMs: Math.round(performance.now() - attachStartedAt),
        bufferedFrags,
        video: describeVideo(video),
      });
      cleanup();
      const error = new Error('HLS attach timed out');
      // Distinct name so the controller can retry a timed-out attach instead of
      // falling back to native playback (which cannot decode the source at all).
      error.name = 'AttachTimeoutError';
      finish('reject', error);
    };

    // Progress restarts the stall guard, clamped so it can never outlive the
    // hard deadline.
    const noteAttachProgress = () => {
      if (settled) {
        return;
      }
      clearTimeout(stallTimeoutId);
      const remaining = deadlineMs - (performance.now() - attachStartedAt);
      if (remaining <= 0) {
        return;
      }
      stallTimeoutId = setTimeout(onAttachTimeout, Math.min(ATTACH_STALL_TIMEOUT_MS, remaining));
    };

    let appendRecoveryAttempts = 0;
    const startPlaybackReady = (reason) => {
      if (settled || readyToPlay) {
        return;
      }
      readyToPlay = true;
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
          // (hardware→software decode retry) can keep the first byte waiting
          // well past hls.js's 10s default. Aborting at 10s cancelled the
          // request server-side and surfaced as a 503 + skipped segment.
          maxTimeToFirstByteMs: 45_000,
          maxLoadTimeMs: 120_000,
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
      // Detach before destroy: destroying with a live MediaSource attached can
      // leave the element feeding buffered audio after the preview unmounts.
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
      // First byte of the seek fragment arrived: the producer is making
      // progress, so push the stall guard out.
      noteAttachProgress();
      transcodeLog('hls frag loaded', {
        frag: describeFrag(data?.frag),
        bytes: data?.frag?.stats?.total,
      });
    });

    hls.on(Hls.Events.FRAG_BUFFERED, (_event, data) => {
      bufferedFrags += 1;
      noteAttachProgress();
      const fragInfo = describeFrag(data?.frag);
      transcodeLog('hls frag buffered', {
        frag: fragInfo,
        video: describeVideo(video),
        bufferedFrags,
        minBufferFrags,
      });
      if (readyToPlay) {
        return;
      }
      if (bufferedFrags >= minBufferFrags) {
        startPlaybackReady('buffer-satisfied');
      }
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
        // Resume at the fragment we were already waiting on. Do NOT use
        // hls.liveSyncPosition here: this is a #EXT-X-PLAYLIST-TYPE:VOD playlist
        // with #EXT-X-ENDLIST, so liveSyncPosition is the live-edge guess
        // (edge - liveSyncDurationCount*targetduration, e.g. 18.6s for this
        // 30.6s clip) — it jumps the playhead far ahead of the requested seek
        // and abandons the not-yet-ready fragment hls.js was already retrying.
        const resumeAt = Number.isFinite(data?.frag?.start)
          ? data.frag.start
          : hls.startPosition;
        try {
          hls.startLoad(Number.isFinite(resumeAt) && resumeAt > 0 ? resumeAt : -1);
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

    transcodeLog('hls attachMedia + loadSource', { playlistURL });
    hls.attachMedia(video);
    hls.loadSource(playlistURL);
  });
}

export { Hls };
