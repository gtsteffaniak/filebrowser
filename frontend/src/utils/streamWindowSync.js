const COOKIE_NAME = 'fb-sw';
const COOKIE_MAX_AGE_SEC = 120;

function encodePayload(payload) {
  return encodeURIComponent(JSON.stringify(payload));
}

/**
 * Writes playback position into a cookie synchronously so the very next range
 * request on /api/media/stream includes it (avoids POST race on seek).
 */
export function setStreamWindowCookie({
  streamToken,
  sessionId,
  currentTime,
  duration,
  seeking,
}) {
  if (!streamToken || !sessionId) {
    return;
  }
  if (!Number.isFinite(duration) || duration <= 0) {
    return;
  }
  const payload = {
    streamToken,
    sessionId,
    currentTime: Number.isFinite(currentTime) ? currentTime : 0,
    duration,
    seeking: Boolean(seeking),
  };
  document.cookie = `${COOKIE_NAME}=${encodePayload(payload)}; path=/; max-age=${COOKIE_MAX_AGE_SEC}; SameSite=Lax`;
}

export function clearStreamWindowCookie() {
  document.cookie = `${COOKIE_NAME}=; path=/; max-age=0; SameSite=Lax`;
}

/**
 * Keeps the stream playback window cookie aligned with the playhead.
 */
export function createStreamWindowSync({ streamToken, sessionId, getPlaybackState }) {
  let intervalId = null;

  function sync() {
    const playback = getPlaybackState();
    setStreamWindowCookie({
      streamToken,
      sessionId,
      currentTime: playback.currentTime,
      duration: playback.duration,
      seeking: playback.seeking,
    });
  }

  return {
    start() {
      sync();
      if (intervalId !== null) {
        return;
      }
      intervalId = window.setInterval(sync, 3000);
    },
    onSeeking() {
      sync();
    },
    onSeeked() {
      sync();
    },
    stop() {
      if (intervalId !== null) {
        window.clearInterval(intervalId);
        intervalId = null;
      }
      clearStreamWindowCookie();
    },
  };
}
