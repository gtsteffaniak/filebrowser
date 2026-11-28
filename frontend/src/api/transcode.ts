import { getApiPath } from "@/utils/url.js";
import { fetchJSON, fetchURL } from "./utils.ts";

// mirroring backend/internal/transcoding/types.go

/** Quality preset a transcode session encodes with */
export type TranscodeProfile = "quality" | "balanced" | "datasaver";
export type TranscodeSessionState = "starting" | "running" | "stopping" | "completed" | "failed";

/** Public view of a transcode session (`SessionInfo`) */
export interface TranscodeSession {
  id: string;
  username?: string;
  source: string;
  path: string;
  fileName?: string;
  profile: TranscodeProfile;
  state: TranscodeSessionState;
  startedAt: number;
  heartbeatIntervalSec: number;
  /** Length of the servable timeline: every session serves the full-length on-demand playlist */
  durationSec?: number;
  /** Playhead the client requested at session start */
  startSec?: number;
  endSec?: number;
  /** Where the session playlist and segments are served from (absolute URL or server-relative path) */
  deliveryBaseUrl?: string;
  reused?: boolean;
  vodPlaylist?: boolean;
  /** Resolved video encoder */
  encoder?: string;
  /** Acceleration backend behind `encoder` ("nvenc", "vaapi", ...) */
  accel?: string;
}

/** Why a session cannot start right now. */
export type TranscodeBlockReason = "user_limit" | "global_limit";

/**
 * Session list/status (`SnapshotResponse`). The client also builds statuses itself, without the
 * counters, when it knows the answer without asking the server (transcoding off, request failed).
 */
export interface TranscodeStatus {
  enabled: boolean;
  canStart: boolean;
  /** Empty once the client has judged the block irrelevant */
  blockReason?: TranscodeBlockReason | "";
  sessions: TranscodeSession[];
  globalLimit?: number;
  userLimit?: number;
  globalActive?: number;
  userActive?: number;
  heartbeatIntervalSec?: number;
}

/** Body of POST /api/media/transcode/sessions (`StartRequest`). */
export interface StartTranscodeSessionBody {
  source: string;
  path: string;
  viewToken: string;
  profile: TranscodeProfile;
  /** Session this one supersedes (profile switch, recovery). */
  replaceSessionId?: string;
  clientId?: string;
  startSec?: number;
}

/** Body of PATCH /api/media/transcode/sessions/{id}/heartbeat (`HeartbeatRequest`). */
export interface TranscodeHeartbeatBody {
  playheadSec: number;
  paused: boolean;
  clientId?: string;
}

/** POST /api/media/transcode/sessions */
export function startTranscodeSession(body: StartTranscodeSessionBody, signal?: AbortSignal): Promise<TranscodeSession> {
  return fetchJSON<TranscodeSession>(getApiPath("media/transcode/sessions"), {
    method: "POST",
    signal,
    headers: { "Content-Type": "application/json" },
    body: JSON.stringify(body),
  });
}

/** GET /api/media/transcode/sessions */
export function getTranscodeSessions(params: Record<string, string> = {}): Promise<TranscodeStatus> {
  return fetchJSON<TranscodeStatus>(getApiPath("media/transcode/sessions", params));
}

/**
 * DELETE /api/media/transcode/sessions/{id}
 *
 * `keepalive` lets the request outlive the page (tab close / unload). It is
 * required on the unload path because a plain fetch is cancelled the moment the
 * document is torn down, which would leave the server session counted against
 * the per-user limit until the janitor reclaims it.
 */
export async function stopTranscodeSession(
  sessionId: string,
  { keepalive = false }: { keepalive?: boolean } = {},
): Promise<void> {
  await fetchURL(getApiPath(`media/transcode/sessions/${encodeURIComponent(sessionId)}`), {
    method: "DELETE",
    keepalive,
  });
}

/** PATCH /api/media/transcode/sessions/{id}/heartbeat */
export async function heartbeatTranscodeSession(sessionId: string, body: TranscodeHeartbeatBody): Promise<void> {
  await fetchURL(getApiPath(`media/transcode/sessions/${encodeURIComponent(sessionId)}/heartbeat`), {
    method: "PATCH",
    headers: { "Content-Type": "application/json" },
    body: JSON.stringify(body),
  });
}

/** Absolute playlist URL for hls.js (session delivery base + /index.m3u8). */
export function getTranscodePlaylistURL(session: Pick<TranscodeSession, "deliveryBaseUrl">): string {
  const base = session.deliveryBaseUrl;
  if (!base) {
    throw new Error("missing transcode delivery URL");
  }
  if (/^https?:\/\//i.test(base)) {
    return `${base.replace(/\/$/, "")}/index.m3u8`;
  }
  return `${window.origin}${base}/index.m3u8`;
}

export function transcodeEventsURL(): string {
  return getApiPath("media/transcode/sessions/events");
}
