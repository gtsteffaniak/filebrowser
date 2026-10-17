import {
  heartbeatTranscodeSession,
  startTranscodeSession,
  stopTranscodeSession,
} from '@/api/transcode.js';
import { transcodeLog, transcodeWarn } from '@/plyr/transcodeDebug.js';

let clientCounter = 0;

export function createTranscodeClientId(prefix = 'plyr') {
  clientCounter += 1;
  return `${prefix}-${Date.now()}-${clientCounter}`;
}

/**
 * True while the session's generation is still producing: live-paced ffmpeg
 * runs ~1x, so playback faster than 1x would only drain the buffered lead and
 * stall. Completed/ready generations can serve any speed on demand.
 */
export function isLivePacedSession(session) {
  return session != null && session.state !== 'completed' && session.state !== 'ready';
}

/**
 * Manages one transcode playback session: start/reuse, heartbeat, detach/stop.
 */
export class TranscodeSessionController {
  constructor({ source, path, viewToken, profile, clientId }) {
    this.source = source;
    this.path = path;
    this.viewToken = viewToken;
    this.profile = profile;
    this.clientId = clientId || createTranscodeClientId();
    this.session = null;
    this.heartbeatTimer = null;
    this.lastPlayhead = 0;
    this.paused = true;
  }

  async start({ replaceSessionId, startSec } = {}) {
    try {
      return await this.startOnce({ replaceSessionId, startSec });
    } catch (err) {
      // The session we were told to replace is already gone; retry without it.
      if (err?.status === 404 && replaceSessionId) {
        transcodeWarn('replace target missing; retrying without replaceSessionId', { replaceSessionId });
        return this.startOnce({ startSec });
      }
      throw err;
    }
  }

  async startOnce({ replaceSessionId, startSec } = {}) {
    transcodeLog('session start request', {
      source: this.source,
      path: this.path,
      profile: this.profile,
      clientId: this.clientId,
      replaceSessionId,
      startSec,
      hasViewToken: Boolean(this.viewToken),
    });
    const result = await startTranscodeSession({
      source: this.source,
      path: this.path,
      viewToken: this.viewToken,
      profile: this.profile,
      replaceSessionId,
      clientId: this.clientId,
      startSec,
    });
    this.session = result;
    transcodeLog('session start response', {
      id: result?.id,
      state: result?.state,
      reused: result?.reused,
      deliveryBaseUrl: result?.deliveryBaseUrl || result?.deliveryBaseURL,
      heartbeatIntervalSec: result?.heartbeatIntervalSec,
    });
    this.startHeartbeat();
    return result;
  }

  startHeartbeat() {
    this.stopHeartbeat();
    const intervalSec = this.session?.heartbeatIntervalSec || 12;
    this.heartbeatTimer = window.setInterval(() => {
      if (!this.session?.id) {
        return;
      }
      heartbeatTranscodeSession(this.session.id, {
        playheadSec: this.lastPlayhead,
        paused: this.paused,
        clientId: this.clientId,
      }).catch((err) => {
        transcodeWarn('heartbeat failed', {
          sessionId: this.session?.id,
          message: err?.message,
          status: err?.status,
        });
        if (err?.status === 404 || err?.status === 410) {
          // The session is already gone server-side; retrying the lease is
          // pointless and keeps erroring every interval.
          this.stopHeartbeat();
        }
      });
    }, Math.max(intervalSec, 5) * 1000);
  }

  updatePlayback({ currentTime, paused }) {
    if (Number.isFinite(currentTime)) {
      this.lastPlayhead = currentTime;
    }
    this.paused = Boolean(paused);
  }

  stopHeartbeat() {
    if (this.heartbeatTimer) {
      clearInterval(this.heartbeatTimer);
      this.heartbeatTimer = null;
    }
  }

  /** Detach client lease without stopping the server session (refresh/reuse). */
  detach() {
    this.stopHeartbeat();
  }

  /** Explicitly stop the server session. */
  async stop() {
    this.stopHeartbeat();
    const id = this.session?.id;
    this.session = null;
    if (id) {
      await stopTranscodeSession(id).catch(() => {});
    }
  }
}
