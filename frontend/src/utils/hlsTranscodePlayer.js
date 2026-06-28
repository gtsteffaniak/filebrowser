import Hls from 'hls.js';
import {
  hlsPlayerTuning,
  normalizeHLSConfig,
  parseHLSConfigFromHeaders,
  parseHLSConfigFromPlaylist,
} from '@/utils/hlsTranscodeConfig';

const TRANSCODE_START_TIMEOUT_MS = 30_000;
/** Retry transient segment failures (504/5xx) before fatal error. */
const SEGMENT_RETRY_MAX = 3;
const SEGMENT_RETRY_BASE_MS = 750;
/** Abort if the same fragment SN is loaded this many times in a short window. */
const FRAG_LOAD_LOOP_MAX = 8;
const FRAG_LOAD_LOOP_WINDOW_MS = 5000;

const HLS_TRANSCODE_DEBUG = typeof localStorage !== 'undefined'
  && localStorage.getItem('hlsTranscodeDebug') === '1';

const HLS_TRANSCODE_AUDIT = typeof localStorage !== 'undefined'
  && (localStorage.getItem('hlsTranscodeAudit') === '1' || HLS_TRANSCODE_DEBUG);

/** Log playhead jumps larger than this during normal playback (not user seek). */
const PLAYHEAD_JUMP_SEC = 0.35;

const HLS_TRANSCODE_ALWAYS_LOG = /startup timing|fatal|error|loop|timeout|buffer append|buffer stalled|starting playback|cleanup|aborted|playhead jump/i;

function hlsLog(message, detail) {
  if (!HLS_TRANSCODE_DEBUG && !HLS_TRANSCODE_ALWAYS_LOG.test(message)) {
    return;
  }
  if (detail === undefined) {
    console.info('[hls-transcode]', message);
    return;
  }
  console.info('[hls-transcode]', message, detail);
}

function createPlaybackTiming() {
  const t0 = performance.now();
  /** @type {Map<string, number>} */
  const marks = new Map();
  return {
    mark(name) {
      const elapsed = Math.round(performance.now() - t0);
      marks.set(name, elapsed);
      return elapsed;
    },
    elapsed() {
      return Math.round(performance.now() - t0);
    },
    summary(extra = {}) {
      return { ...Object.fromEntries(marks), totalMs: this.elapsed(), ...extra };
    },
  };
}

function fragLoadTiming(stats) {
  if (!stats?.loading) {
    return null;
  }
  const loading = stats.loading;
  const ttfbMs = loading.first !== null && loading.first !== undefined
    && loading.start !== null && loading.start !== undefined
    ? Math.round(loading.first - loading.start)
    : null;
  const loadMs = loading.end !== null && loading.end !== undefined
    && loading.start !== null && loading.start !== undefined
    ? Math.round(loading.end - loading.start)
    : null;
  return {
    ttfbMs,
    loadMs,
    totalMs: stats.total !== null && stats.total !== undefined ? Math.round(stats.total) : loadMs,
    bytes: stats.loaded ?? stats.totalBytes ?? null,
  };
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

/** Detect forward/backward playhead jumps while playing (MSE timeline gaps). */
function installPlayheadAudit(videoEl, { onCleanup } = {}) {
  if (!HLS_TRANSCODE_AUDIT || !videoEl) {
    return () => {};
  }
  const audit = {
    jumps: [],
    samples: [],
    startWall: performance.now(),
  };
  if (typeof window !== 'undefined') {
    window.__hlsTranscodePlayheadAudit = audit;
  }

  let lastT = 0;
  let lastWall = performance.now();
  let seeking = false;

  const onSeeking = () => { seeking = true; };
  const onSeeked = () => { seeking = false; lastT = videoEl.currentTime; lastWall = performance.now(); };

  const poll = setInterval(() => {
    if (!videoEl.isConnected) {
      return;
    }
    const now = performance.now();
    const t = videoEl.currentTime;
    const wallDelta = (now - lastWall) / 1000;
    const playDelta = t - lastT;
    if (lastT > 0 && !videoEl.paused && !seeking && !videoEl.seeking) {
      const forwardJump = playDelta - wallDelta;
      if (playDelta < -0.05) {
        const entry = { kind: 'backward', at: t, delta: playDelta, wallMs: now - audit.startWall };
        audit.jumps.push(entry);
        hlsLog('playhead jump backward', entry);
      } else if (forwardJump > PLAYHEAD_JUMP_SEC) {
        const entry = { kind: 'forward', at: t, delta: forwardJump, wallMs: now - audit.startWall };
        audit.jumps.push(entry);
        hlsLog('playhead jump forward', entry);
      }
      audit.samples.push({ t, wallMs: now - audit.startWall, playDelta, wallDelta });
    }
    lastT = t;
    lastWall = now;
  }, 100);

  videoEl.addEventListener('seeking', onSeeking);
  videoEl.addEventListener('seeked', onSeeked);
  hlsLog('playhead audit enabled — inspect window.__hlsTranscodePlayheadAudit');

  const stop = () => {
    clearInterval(poll);
    videoEl.removeEventListener('seeking', onSeeking);
    videoEl.removeEventListener('seeked', onSeeked);
    onCleanup?.(audit);
  };
  return stop;
}

/** Safari native HLS handles fMP4 VOD; Chromium "native" HLS does not. */
function shouldUseNativeHls(videoEl) {
  if (!videoEl.canPlayType('application/vnd.apple.mpegurl')) {
    return false;
  }
  const ua = navigator.userAgent;
  return /iPad|iPhone|iPod/.test(ua) || (
    /^((?!chrome|android).)*safari/i.test(ua)
  );
}

/**
 * Plays on-demand HLS transcode output via hls.js (or native HLS on Safari).
 * Returns a controller with destroy() for cleanup.
 */
export function startHlsTranscodePlayback(videoEl, url, {
  signal,
  onLoadingChange,
  onFirstBuffered,
  onFatalError,
  onSeek,
  onSessionReady,
  startPosition = -1,
  sessionId = null,
  hlsConfig = null,
} = {}) {
  if (!videoEl) {
    return Promise.reject(new Error('video element required'));
  }

  hlsLog('starting playback', { url, startPosition, hlsConfig });

  const timing = createPlaybackTiming();
  timing.mark('start');
  let segmentLoadCount = 0;
  let deliveryTuning = hlsPlayerTuning(hlsConfig);

  let hls = null;
  let destroyed = false;
  let loadingHidden = false;
  let watchdog = null;
  let rejectPromise = null;
  let stopPlayheadAudit = null;

  const setLoading = (loading) => {
    hlsLog('loading state', { loading });
    onLoadingChange?.(loading);
  };

  const initialStartPosition = Number.isFinite(startPosition) && startPosition >= 0
    ? startPosition
    : -1;

  const hideLoadingOnce = () => {
    if (loadingHidden) {
      return;
    }
    loadingHidden = true;
    setLoading(false);
  };

  const clearWatchdog = () => {
    if (watchdog !== null) {
      clearTimeout(watchdog);
      watchdog = null;
    }
  };

  const cleanup = () => {
    if (destroyed) {
      return;
    }
    destroyed = true;
    hlsLog('cleanup', {
      videoInDom: videoEl?.isConnected,
      readyState: videoEl?.readyState,
    });
    clearWatchdog();
    stopPlayheadAudit?.();
    stopPlayheadAudit = null;
    signal?.removeEventListener('abort', onAbort);
    hls?.destroy();
    hls = null;
    // Caller resets the video element when tearing down MSE mode. Avoid video.load()
    // here — it races with Plyr and causes bufferAppendError on subsequent segments.
  };

  const onAbort = () => {
    hlsLog('aborted by signal');
    cleanup();
    rejectPromise?.(new DOMException('Aborted', 'AbortError'));
  };

  signal?.addEventListener('abort', onAbort, { once: true });
  if (signal?.aborted) {
    return Promise.reject(new DOMException('Aborted', 'AbortError'));
  }

  setLoading(true);
  watchdog = setTimeout(() => {
    hlsLog('start timeout', { ms: TRANSCODE_START_TIMEOUT_MS });
    cleanup();
    setLoading(false);
    const err = new Error('transcode start timeout');
    err.code = 'TRANSCODE_TIMEOUT';
    rejectPromise?.(err);
  }, TRANSCODE_START_TIMEOUT_MS);

  const finishWatchdogOnReady = () => {
    clearWatchdog();
    hideLoadingOnce();
  };

  if (shouldUseNativeHls(videoEl) && !Hls.isSupported()) {
    hlsLog('using native HLS (Safari)');
    return new Promise((resolve, reject) => {
      rejectPromise = reject;
      const seekTo = (position) => {
        if (destroyed || !videoEl) {
          return;
        }
        if (Number.isFinite(position) && position >= 0) {
          try {
            videoEl.currentTime = position;
          } catch {
            /* buffer not ready */
          }
        }
      };
      const onReady = () => {
        hlsLog('native HLS canplay');
        if (initialStartPosition >= 0) {
          seekTo(initialStartPosition);
        }
        finishWatchdogOnReady();
        resolve({ destroy: cleanup, seekTo });
      };
      const onError = () => {
        hlsLog('native HLS error', { code: videoEl.error?.code, message: videoEl.error?.message });
        cleanup();
        setLoading(false);
        reject(new Error('native HLS playback failed'));
      };
      videoEl.addEventListener('canplay', onReady, { once: true });
      videoEl.addEventListener('error', onError, { once: true });
      videoEl.src = url;
    });
  }

  if (!Hls.isSupported()) {
    clearWatchdog();
    setLoading(false);
    return Promise.reject(new Error('HLS not supported'));
  }

  hlsLog('using hls.js');

  return new Promise((resolve, reject) => {
    rejectPromise = reject;
    let sessionReadyNotified = false;
    const notifySessionReady = (id) => {
      if (!id || sessionReadyNotified) {
        return;
      }
      sessionReadyNotified = true;
      onSessionReady?.(id);
    };
    let playbackStarted = false;
    let firstBufferNotified = false;
    let wasPlayingBeforeStall = false;
    let lastPlayheadSec = 0;
    const segmentRetryCounts = new Map();
    const fragLoadLoop = new Map();

    const bufferedAheadSec = () => {
      if (!videoEl?.buffered?.length) {
        return 0;
      }
      const t = videoEl.currentTime;
      for (let i = 0; i < videoEl.buffered.length; i++) {
        const start = videoEl.buffered.start(i);
        const end = videoEl.buffered.end(i);
        if (t >= start && t <= end) {
          return Math.max(0, end - t);
        }
        if (t < start) {
          return 0;
        }
      }
      const end = videoEl.buffered.end(videoEl.buffered.length - 1);
      return Math.max(0, end - t);
    };

    const maybeResumeAfterStall = () => {
      if (destroyed || !wasPlayingBeforeStall || !videoEl.paused) {
        return;
      }
      if (bufferedAheadSec() < 0.25) {
        return;
      }
      wasPlayingBeforeStall = false;
      void videoEl.play().catch((err) => {
        if (err?.name !== 'AbortError') {
          hlsLog('resume after stall failed', { message: err?.message });
        }
      });
    };

    const noteFragLoad = (sn) => {
      if (sn === null || sn === undefined) {
        return false;
      }
      const key = String(sn);
      const now = performance.now();
      const prev = fragLoadLoop.get(key) ?? { count: 0, windowStart: now };
      if (now - prev.windowStart > FRAG_LOAD_LOOP_WINDOW_MS) {
        prev.count = 0;
        prev.windowStart = now;
      }
      prev.count += 1;
      fragLoadLoop.set(key, prev);
      if (prev.count >= FRAG_LOAD_LOOP_MAX) {
        hlsLog('frag load loop detected', { sn, count: prev.count, windowMs: FRAG_LOAD_LOOP_WINDOW_MS });
        return true;
      }
      return false;
    };

    const failPlayback = (message, data) => {
      cleanup();
      setLoading(false);
      const err = new Error(message);
      if (data?.response?.code) {
        err.status = data.response.code;
      }
      if (playbackStarted) {
        onFatalError?.(err, data);
      } else {
        reject(err);
      }
    };

    hls = new Hls({
      enableWorker: true,
      // Keep prefetch window tight: each segment triggers a live ffmpeg encode on the server.
      maxBufferLength: deliveryTuning.bufferAheadSec,
      maxMaxBufferLength: deliveryTuning.bufferAheadSec,
      backBufferLength: 10,
      maxBufferSize: 40 * 1000 * 1000,
      maxBufferHole: 2.0,
      maxFragLookUpTolerance: 0.5,
      startFragPrefetch: true,
      testBandwidth: false,
      startPosition: initialStartPosition,
      fragLoadPolicy: {
        default: {
          maxTimeToFirstByteMs: 60_000,
          maxLoadTimeMs: 120_000,
          timeoutRetry: {
            maxNumRetry: SEGMENT_RETRY_MAX,
            retryDelayMs: SEGMENT_RETRY_BASE_MS,
            maxRetryDelayMs: SEGMENT_RETRY_BASE_MS * 8,
          },
          errorRetry: {
            maxNumRetry: SEGMENT_RETRY_MAX,
            retryDelayMs: SEGMENT_RETRY_BASE_MS,
            maxRetryDelayMs: SEGMENT_RETRY_BASE_MS * 8,
          },
        },
      },
      xhrSetup(xhr, requestUrl) {
        xhr.withCredentials = true;
        if (HLS_TRANSCODE_DEBUG) {
          hlsLog('xhr open', { url: requestUrl });
        }
        xhr.addEventListener('load', () => {
          if (!requestUrl.includes('playlist.m3u8')) {
            return;
          }
          const headerSession = xhr.getResponseHeader('X-Transcode-Session');
          if (headerSession) {
            notifySessionReady(headerSession);
          }
          const fromHeaders = parseHLSConfigFromHeaders(xhr);
          const fromPlaylist = fromHeaders ? null : parseHLSConfigFromPlaylist(xhr.responseText);
          const parsed = fromHeaders || fromPlaylist;
          if (parsed) {
            deliveryTuning = hlsPlayerTuning(normalizeHLSConfig({ ...deliveryTuning, ...parsed }));
            hls.config.maxBufferLength = deliveryTuning.bufferAheadSec;
            hls.config.maxMaxBufferLength = deliveryTuning.bufferAheadSec;
            hlsLog('HLS delivery config', deliveryTuning);
          }
        }, { once: false });
      },
    });

    hls.on(Hls.Events.MANIFEST_LOADING, (_event, data) => {
      timing.mark('manifestLoading');
      hlsLog('manifest loading', { url: data.url, elapsedMs: timing.elapsed() });
    });

    hls.on(Hls.Events.MANIFEST_LOADED, (_event, data) => {
      timing.mark('manifestLoaded');
      const manifestTiming = fragLoadTiming(data.stats);
      hlsLog('manifest loaded', {
        levels: data.levels?.length,
        firstLevel: data.firstLevel,
        elapsedMs: timing.elapsed(),
        network: manifestTiming,
      });
    });

    const seekTo = (position) => {
      if (destroyed || !hls) {
        return;
      }
      const t = Number.isFinite(position) && position >= 0 ? position : -1;
      hls.startLoad(t >= 0 ? t : -1);
      if (t >= 0 && videoEl) {
        try {
          videoEl.currentTime = t;
        } catch {
          /* buffer not ready */
        }
      }
    };

    hls.on(Hls.Events.MANIFEST_PARSED, (_event, data) => {
      timing.mark('manifestParsed');
      hlsLog('manifest parsed', {
        levels: data.levels?.length,
        startPosition: initialStartPosition,
        elapsedMs: timing.elapsed(),
      });
      // Ensure the first segment is requested even while paused (spinner visible).
      hls.startLoad(initialStartPosition);
      playbackStarted = true;
      stopPlayheadAudit = installPlayheadAudit(videoEl);
      resolve({ destroy: cleanup, seekTo });
    });

    hls.on(Hls.Events.FRAG_LOADING, (_event, data) => {
      hlsLog('frag loading', {
        frag: describeFrag(data.frag),
        elapsedMs: timing.elapsed(),
      });
    });

    hls.on(Hls.Events.FRAG_LOADED, (_event, data) => {
      segmentLoadCount += 1;
      if (noteFragLoad(data.frag?.sn)) {
        failPlayback('HLS fragment load loop — timeline mismatch or stale session cache');
        return;
      }
      const network = fragLoadTiming(data.stats);
      hlsLog('frag loaded', {
        frag: describeFrag(data.frag),
        elapsedMs: timing.elapsed(),
        segmentIndex: segmentLoadCount,
        network,
      });
    });

    hls.on(Hls.Events.FRAG_BUFFERED, (_event, data) => {
      const frag = describeFrag(data.frag);
      hlsLog('frag buffered', {
        frag,
        currentTime: videoEl.currentTime,
        buffered: videoEl.buffered.length > 0
          ? { start: videoEl.buffered.start(0), end: videoEl.buffered.end(videoEl.buffered.length - 1) }
          : null,
        videoSize: `${videoEl.videoWidth}x${videoEl.videoHeight}`,
        elapsedMs: timing.elapsed(),
      });
      finishWatchdogOnReady();
      if (!firstBufferNotified) {
        firstBufferNotified = true;
        timing.mark('firstBuffered');
        hlsLog('startup timing summary', timing.summary({
          firstFragSn: frag?.sn,
          firstFragType: frag?.type,
          segmentsLoaded: segmentLoadCount,
          bufferedAheadSec: bufferedAheadSec(),
        }));
        onFirstBuffered?.(videoEl, data.frag);
      }
      maybeResumeAfterStall();
    });

    hls.on(Hls.Events.BUFFER_STALLED, () => {
      hlsLog('buffer stalled', {
        currentTime: videoEl.currentTime,
        readyState: videoEl.readyState,
        networkState: videoEl.networkState,
        bufferedAhead: bufferedAheadSec(),
      });
    });

    hls.on(Hls.Events.ERROR, (_event, data) => {
      hlsLog('error', {
        fatal: data.fatal,
        type: data.type,
        details: data.details,
        reason: data.reason,
        response: data.response,
        frag: describeFrag(data.frag),
      });
      const status = data.response?.code;
      const isSegmentLoad = data.type === Hls.ErrorTypes.NETWORK_ERROR
        && (data.details === Hls.ErrorDetails.FRAG_LOAD_ERROR
          || data.details === Hls.ErrorDetails.FRAG_LOAD_TIMEOUT);
      if (!data.fatal && isSegmentLoad && status >= 500) {
        const sn = data.frag?.sn ?? 'unknown';
        const attempts = (segmentRetryCounts.get(sn) ?? 0) + 1;
        segmentRetryCounts.set(sn, attempts);
        if (attempts <= SEGMENT_RETRY_MAX) {
          hlsLog('segment retry scheduled', { sn, attempts, status });
          return;
        }
      }
      if (!data.fatal) {
        if (data.details === Hls.ErrorDetails.BUFFER_STALLED_ERROR) {
          return;
        }
        if (data.details === Hls.ErrorDetails.BUFFER_APPEND_ERROR) {
          hlsLog('buffer append error', {
            reason: data.reason,
            frag: describeFrag(data.frag),
            currentTime: videoEl.currentTime,
          });
        }
        return;
      }
      cleanup();
      setLoading(false);
      const err = new Error(`HLS playback failed: ${data.type}/${data.details}`);
      if (status) {
        err.status = status;
      }
      if (playbackStarted) {
        onFatalError?.(err, data);
      } else {
        reject(err);
      }
    });

    videoEl.addEventListener('seeking', () => {
      const t = videoEl.currentTime;
      if (!Number.isFinite(t) || t < 0) {
        return;
      }
      const jumped = Math.abs(t - lastPlayheadSec) > deliveryTuning.seekJumpSec;
      lastPlayheadSec = t;
      onSeek?.({
        playheadSec: t,
        seeked: jumped,
        session: sessionId,
      });
    });

    videoEl.addEventListener('timeupdate', () => {
      if (Number.isFinite(videoEl.currentTime)) {
        lastPlayheadSec = videoEl.currentTime;
      }
    });

    videoEl.addEventListener('waiting', () => {
      hlsLog('video waiting', {
        currentTime: videoEl.currentTime,
        bufferedAhead: bufferedAheadSec(),
      });
      if (playbackStarted && !videoEl.paused) {
        wasPlayingBeforeStall = true;
      }
    });

    videoEl.addEventListener('playing', () => {
      wasPlayingBeforeStall = false;
      hlsLog('video playing', { currentTime: videoEl.currentTime });
    });

    hls.loadSource(url);
    hls.attachMedia(videoEl);
  });
}
