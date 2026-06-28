import { getApiPath } from '@/utils/url';
import { state } from '@/store';

/** @type {{ source: string, path: string, sessionId: string | null } | null} */
let activeSession = null;
let pageHookInstalled = false;
let pingTimer = null;

const PING_INTERVAL_MS = 30_000;

function buildReleaseUrl(source) {
  return `${window.origin}${getApiPath('media/transcode/sessions', { source })}`;
}

function buildPingUrl() {
  return `${window.origin}${getApiPath('media/transcode/sessions/ping')}`;
}

/**
 * Notify the server that playback is active; optionally invalidate forward cache on seek.
 * @param {{ session: string, playheadSec?: number, seekIndex?: number, seeked?: boolean }} payload
 */
export function pingTranscodeSession(payload) {
  if (!payload?.session) {
    return;
  }
  void fetch(buildPingUrl(), {
    method: 'POST',
    credentials: 'same-origin',
    headers: {
      'Content-Type': 'application/json',
      sessionId: state.sessionId,
    },
    body: JSON.stringify(payload),
  }).catch(() => {});
}

export function startTranscodeSessionPing(sessionKey, getPlayheadSec) {
  stopTranscodeSessionPing();
  if (!sessionKey) {
    return;
  }
  const send = () => {
    const playheadSec = typeof getPlayheadSec === 'function' ? getPlayheadSec() : 0;
    pingTranscodeSession({ session: sessionKey, playheadSec });
  };
  send();
  pingTimer = setInterval(send, PING_INTERVAL_MS);
}

export function stopTranscodeSessionPing() {
  if (pingTimer !== null) {
    clearInterval(pingTimer);
    pingTimer = null;
  }
}

/**
 * Release all transcode sessions for a source. Use keepalive on pagehide/tab close.
 * @param {string} source
 * @param {{ keepalive?: boolean }} [options]
 */
export function sendReleaseAllTranscodeSessions(source, { keepalive = false } = {}) {
  if (!source) {
    return;
  }
  void fetch(buildReleaseUrl(source), {
    method: 'DELETE',
    credentials: 'same-origin',
    keepalive,
    headers: {
      sessionId: state.sessionId,
    },
  }).catch(() => {});
}

export function registerTranscodeSession(source, path, sessionId = null) {
  if (!source || !path) {
    return;
  }
  activeSession = { source, path, sessionId };
  installPageHook();
}

export function updateTranscodeSessionId(sessionId) {
  if (activeSession) {
    activeSession.sessionId = sessionId;
  }
}

export function unregisterTranscodeSession(source, path) {
  if (!activeSession) {
    return;
  }
  if (activeSession.source === source && activeSession.path === path) {
    activeSession = null;
  }
}

export function releaseRegisteredTranscodeSession({ keepalive = false } = {}) {
  stopTranscodeSessionPing();
  if (!activeSession?.source) {
    return;
  }
  const { source } = activeSession;
  activeSession = null;
  sendReleaseAllTranscodeSessions(source, { keepalive });
}

function installPageHook() {
  if (pageHookInstalled) {
    return;
  }
  pageHookInstalled = true;
  window.addEventListener('pagehide', () => {
    releaseRegisteredTranscodeSession({ keepalive: true });
  });
}
