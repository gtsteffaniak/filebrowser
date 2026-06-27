import { getApiPath } from '@/utils/url';
import { state } from '@/store';

/** @type {{ source: string, path: string } | null} */
let activeSession = null;
let pageHookInstalled = false;

function buildReleaseUrl(source) {
  return `${window.origin}${getApiPath('media/transcode/sessions', { source })}`;
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

export function registerTranscodeSession(source, path) {
  if (!source || !path) {
    return;
  }
  activeSession = { source, path };
  installPageHook();
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
