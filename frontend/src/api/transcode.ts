import type { StartTranscodeSessionBody, TranscodeHeartbeatBody, TranscodeSession, TranscodeStatus } from "@/types/transcode.ts";
import { getApiPath } from "@/utils/url.js";
import { fetchJSON, fetchURL } from "./utils.ts";

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
