import {
  heartbeatTranscodeSession,
  startTranscodeSession,
  stopTranscodeSession,
  type TranscodeProfile,
  type TranscodeSession,
} from "@/api/transcode.ts";
import { errorMessage, httpStatusOf, type PlayerError } from "@/player/playerErrors.ts";
import { transcodeLog, transcodeWarn } from "@/player/transcodeDebug.ts";

let clientCounter = 0;

export function createTranscodeClientId(prefix = "plyr"): string {
  clientCounter += 1;
  return `${prefix}-${Date.now()}-${clientCounter}`;
}

export interface TranscodeSessionControllerOptions {
  source: string;
  path: string;
  viewToken: string;
  profile: TranscodeProfile;
  clientId?: string;
}

export interface StartSessionOptions {
  replaceSessionId?: string;
  startSec?: number;
}

/** Manages one transcode playback session: start/reuse, heartbeat, detach/stop */
export class TranscodeSessionController {
  readonly source: string;
  readonly path: string;
  readonly viewToken: string;
  readonly profile: TranscodeProfile;
  readonly clientId: string;
  session: TranscodeSession | null = null;
  heartbeatTimer: number | null = null;
  lastPlayhead = 0;
  paused = true;

  constructor({ source, path, viewToken, profile, clientId }: TranscodeSessionControllerOptions) {
    this.source = source;
    this.path = path;
    this.viewToken = viewToken;
    this.profile = profile;
    this.clientId = clientId || createTranscodeClientId();
  }

  async start({ replaceSessionId, startSec }: StartSessionOptions = {}): Promise<TranscodeSession> {
    try {
      return await this.startOnce({ replaceSessionId, startSec });
    } catch (err) {
      // The session we were told to replace is already gone so retry without it.
      if (httpStatusOf(err as PlayerError) === 404 && replaceSessionId) {
        transcodeWarn("replace target missing; retrying without replaceSessionId", { replaceSessionId });
        return this.startOnce({ startSec });
      }
      throw err;
    }
  }

  async startOnce({ replaceSessionId, startSec }: StartSessionOptions = {}): Promise<TranscodeSession> {
    transcodeLog("session start request", {
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
    transcodeLog("session start response", {
      id: result.id,
      state: result.state,
      reused: result.reused,
      deliveryBaseUrl: result.deliveryBaseUrl,
      heartbeatIntervalSec: result.heartbeatIntervalSec,
    });
    this.startHeartbeat();
    return result;
  }

  startHeartbeat(): void {
    this.stopHeartbeat();
    const intervalSec = this.session?.heartbeatIntervalSec || 12;
    this.heartbeatTimer = window.setInterval(
      () => {
        const sessionId = this.session?.id;
        if (!sessionId) {
          return;
        }
        heartbeatTranscodeSession(sessionId, {
          playheadSec: this.lastPlayhead,
          paused: this.paused,
          clientId: this.clientId,
        }).catch((err: PlayerError) => {
          const status = httpStatusOf(err);
          transcodeWarn("heartbeat failed", {
            sessionId: this.session?.id,
            message: errorMessage(err),
            status,
          });
          if (status === 404 || status === 410) {
            // The session is already gone server-side; retrying the lease is pointless and keeps erroring every interval.
            this.stopHeartbeat();
          }
        });
      },
      Math.max(intervalSec, 5) * 1000,
    );
  }

  updatePlayback({ currentTime, paused }: { currentTime?: number; paused?: boolean }): void {
    if (currentTime !== undefined && Number.isFinite(currentTime)) {
      this.lastPlayhead = currentTime;
    }
    this.paused = Boolean(paused);
  }

  stopHeartbeat(): void {
    if (this.heartbeatTimer !== null) {
      clearInterval(this.heartbeatTimer);
      this.heartbeatTimer = null;
    }
  }
  /** Detach client lease without stopping the server session (refresh/reuse). */
  detach(): void {
    this.stopHeartbeat();
  }

  /**
   * Explicitly stop the server session. `keepalive` must be set on the unload
   * path so the DELETE is not cancelled as the document is torn down.
   */
  async stop({ keepalive = false }: { keepalive?: boolean } = {}): Promise<void> {
    this.stopHeartbeat();
    const id = this.session?.id;
    this.session = null;
    if (id) {
      await stopTranscodeSession(id, { keepalive }).catch(() => {});
    }
  }
}
