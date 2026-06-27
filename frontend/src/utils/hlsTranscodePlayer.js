import Hls from 'hls.js';

const TRANSCODE_START_TIMEOUT_MS = 30_000;
/** On-demand segments are expensive; cap how far ahead of playhead hls.js prefetches. */
const TRANSCODE_BUFFER_AHEAD_SEC = 12;
/** Resume playback once at least this many seconds are buffered ahead of the playhead. */
const TRANSCODE_RESUME_BUFFER_SEC = 2;

function hlsLog(message, detail) {
  if (detail === undefined) {
    console.info('[hls-transcode]', message);
    return;
  }
  console.info('[hls-transcode]', message, detail);
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
export function startHlsTranscodePlayback(videoEl, url, { signal, onLoadingChange, onFirstBuffered, onFatalError, startPosition = -1 } = {}) {
  if (!videoEl) {
    return Promise.reject(new Error('video element required'));
  }

  hlsLog('starting playback', { url, startPosition });

  let hls = null;
  let destroyed = false;
  let loadingHidden = false;
  let watchdog = null;
  let rejectPromise = null;

  const setLoading = (loading) => {
    hlsLog('loading state', { loading });
    onLoadingChange?.(loading);
  };

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
      const onReady = () => {
        hlsLog('native HLS canplay');
        finishWatchdogOnReady();
        resolve({ destroy: cleanup });
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
    let playbackStarted = false;
    let firstBufferNotified = false;
    let waitingForEncode = false;
    let resumeAfterBuffer = false;
    const initialStartPosition = Number.isFinite(startPosition) && startPosition >= 0
      ? startPosition
      : -1;

    const bufferedAheadSec = () => {
      if (!videoEl?.buffered?.length) {
        return 0;
      }
      const end = videoEl.buffered.end(videoEl.buffered.length - 1);
      return Math.max(0, end - videoEl.currentTime);
    };

    const enterEncodeWait = () => {
      if (waitingForEncode || destroyed) {
        return;
      }
      waitingForEncode = true;
      resumeAfterBuffer = !videoEl.paused;
      videoEl.pause();
      setLoading(true);
      hlsLog('encode wait', {
        currentTime: videoEl.currentTime,
        bufferedAhead: bufferedAheadSec(),
      });
    };

    const leaveEncodeWait = () => {
      if (!waitingForEncode || destroyed) {
        return;
      }
      waitingForEncode = false;
      if (loadingHidden) {
        setLoading(false);
      } else {
        hideLoadingOnce();
      }
      if (resumeAfterBuffer) {
        resumeAfterBuffer = false;
        void videoEl.play().catch((err) => {
          if (err?.name !== 'AbortError') {
            hlsLog('resume after encode wait failed', { message: err?.message });
          }
        });
      }
      hlsLog('encode wait done', {
        currentTime: videoEl.currentTime,
        bufferedAhead: bufferedAheadSec(),
      });
    };

    hls = new Hls({
      enableWorker: true,
      // Keep prefetch window tight: each segment triggers a live ffmpeg encode on the server.
      maxBufferLength: TRANSCODE_BUFFER_AHEAD_SEC,
      maxMaxBufferLength: TRANSCODE_BUFFER_AHEAD_SEC,
      backBufferLength: 10,
      maxBufferSize: 40 * 1000 * 1000,
      maxBufferHole: 0.5,
      maxFragLookUpTolerance: 0.25,
      startFragPrefetch: true,
      testBandwidth: false,
      startPosition: initialStartPosition,
      xhrSetup(xhr, requestUrl) {
        xhr.withCredentials = true;
        hlsLog('xhr open', { url: requestUrl });
      },
    });

    hls.on(Hls.Events.MANIFEST_LOADING, (_event, data) => {
      hlsLog('manifest loading', { url: data.url });
    });

    hls.on(Hls.Events.MANIFEST_LOADED, (_event, data) => {
      hlsLog('manifest loaded', {
        levels: data.levels?.length,
        firstLevel: data.firstLevel,
        stats: data.stats,
      });
    });

    hls.on(Hls.Events.MANIFEST_PARSED, (_event, data) => {
      hlsLog('manifest parsed', { levels: data.levels?.length, startPosition: initialStartPosition });
      // Ensure the first segment is requested even while paused (spinner visible).
      hls.startLoad(initialStartPosition);
      playbackStarted = true;
      resolve({ destroy: cleanup });
    });

    hls.on(Hls.Events.FRAG_LOADING, (_event, data) => {
      hlsLog('frag loading', describeFrag(data.frag));
    });

    hls.on(Hls.Events.FRAG_LOADED, (_event, data) => {
      hlsLog('frag loaded', {
        frag: describeFrag(data.frag),
        stats: data.stats,
      });
    });

    hls.on(Hls.Events.FRAG_BUFFERED, (_event, data) => {
      hlsLog('frag buffered', {
        frag: describeFrag(data.frag),
        currentTime: videoEl.currentTime,
        buffered: videoEl.buffered.length > 0
          ? { start: videoEl.buffered.start(0), end: videoEl.buffered.end(videoEl.buffered.length - 1) }
          : null,
        videoSize: `${videoEl.videoWidth}x${videoEl.videoHeight}`,
      });
      finishWatchdogOnReady();
      if (!firstBufferNotified) {
        firstBufferNotified = true;
        onFirstBuffered?.(videoEl, data.frag);
      }
      if (waitingForEncode && bufferedAheadSec() >= TRANSCODE_RESUME_BUFFER_SEC) {
        leaveEncodeWait();
      }
    });

    hls.on(Hls.Events.BUFFER_STALLED, () => {
      hlsLog('buffer stalled', {
        currentTime: videoEl.currentTime,
        readyState: videoEl.readyState,
        networkState: videoEl.networkState,
        bufferedAhead: bufferedAheadSec(),
      });
      enterEncodeWait();
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
      if (!data.fatal) {
        if (data.details === Hls.ErrorDetails.BUFFER_STALLED_ERROR) {
          enterEncodeWait();
        }
        return;
      }
      const status = data.response?.code;
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

    videoEl.addEventListener('waiting', () => {
      hlsLog('video waiting', {
        currentTime: videoEl.currentTime,
        bufferedAhead: bufferedAheadSec(),
      });
      if (playbackStarted && bufferedAheadSec() < 0.5) {
        enterEncodeWait();
      }
    });

    videoEl.addEventListener('playing', () => {
      hlsLog('video playing', { currentTime: videoEl.currentTime });
    });

    hls.loadSource(url);
    hls.attachMedia(videoEl);
  });
}
