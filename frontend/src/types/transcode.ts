// from backend/internal/transcoding/types.go

/** Quality preset a transcode session encodes with */
export type TranscodeProfile = "quality" | "balanced" | "datasaver";

export type TranscodeSessionState = "starting" | "running" | "stopping" | "completed" | "failed";

/** View of a transcode session (`SessionInfo`) */
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
