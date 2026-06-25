const VIDEO_AUDIO_MIME = 'video/mp4; codecs="avc1.42E01E, mp4a.40.2"';
const VIDEO_ONLY_MIME = 'video/mp4; codecs="avc1.42E01E"';

/**
 * Streams fMP4 from url into videoEl via Media Source Extensions.
 * Returns a controller with destroy() for cleanup.
 */
export async function startFmp4MsePlayback(videoEl, url, { signal, hasAudio = true } = {}) {
  if (!videoEl) {
    throw new Error('video element required');
  }
  if (typeof MediaSource === 'undefined') {
    throw new Error('MediaSource not supported');
  }

  const mime = hasAudio ? VIDEO_AUDIO_MIME : VIDEO_ONLY_MIME;
  if (!MediaSource.isTypeSupported(mime)) {
    throw new Error(`MIME type not supported: ${mime}`);
  }

  const mediaSource = new MediaSource();
  const objectUrl = URL.createObjectURL(mediaSource);
  videoEl.src = objectUrl;

  await new Promise((resolve, reject) => {
    const onOpen = () => {
      cleanup();
      resolve();
    };
    const onError = () => {
      cleanup();
      reject(new Error('MediaSource error'));
    };
    const onAbort = () => {
      cleanup();
      reject(new DOMException('Aborted', 'AbortError'));
    };
    const cleanup = () => {
      mediaSource.removeEventListener('sourceopen', onOpen);
      mediaSource.removeEventListener('error', onError);
      signal?.removeEventListener('abort', onAbort);
    };
    mediaSource.addEventListener('sourceopen', onOpen);
    mediaSource.addEventListener('error', onError);
    signal?.addEventListener('abort', onAbort, { once: true });
    if (signal?.aborted) {
      onAbort();
    }
  });

  const sourceBuffer = mediaSource.addSourceBuffer(mime);
  const queue = [];
  let closed = false;
  let reader = null;
  let playbackStarted = false;

  const tryStartPlayback = () => {
    if (playbackStarted || closed) {
      return;
    }
    if (sourceBuffer.buffered.length === 0) {
      return;
    }
    playbackStarted = true;
    videoEl.play().catch(() => {});
  };

  const pump = () => {
    if (closed || sourceBuffer.updating || queue.length === 0) {
      return;
    }
    try {
      sourceBuffer.appendBuffer(queue.shift());
    } catch (err) {
      console.error('fMP4 appendBuffer failed:', err);
    }
  };

  sourceBuffer.addEventListener('updateend', () => {
    pump();
    tryStartPlayback();
  });
  sourceBuffer.addEventListener('error', () => {
    console.error('fMP4 SourceBuffer error');
  });

  const response = await fetch(url, { signal, credentials: 'include' });
  if (!response.ok) {
    const err = new Error(`transcode fetch failed: ${response.status}`);
    err.status = response.status;
    throw err;
  }
  if (!response.body) {
    throw new Error('transcode response has no body');
  }

  reader = response.body.getReader();

  const readLoop = async () => {
    while (!closed) {
      const { done, value } = await reader.read();
      if (done) {
        const endStream = () => {
          if (!sourceBuffer.updating && queue.length === 0 && mediaSource.readyState === 'open') {
            try {
              mediaSource.endOfStream();
            } catch {
              /* ignore */
            }
          }
        };
        if (sourceBuffer.updating) {
          sourceBuffer.addEventListener('updateend', endStream, { once: true });
        } else {
          endStream();
        }
        break;
      }
      queue.push(value);
      pump();
    }
  };

  readLoop().catch((err) => {
    if (!closed && err?.name !== 'AbortError') {
      console.error('fMP4 read loop failed:', err);
    }
  });

  return {
    destroy() {
      if (closed) {
        return;
      }
      closed = true;
      reader?.cancel().catch(() => {});
      if (mediaSource.readyState === 'open') {
        try {
          mediaSource.endOfStream();
        } catch {
          /* ignore */
        }
      }
      URL.revokeObjectURL(objectUrl);
      videoEl.removeAttribute('src');
      videoEl.load();
    },
  };
}
