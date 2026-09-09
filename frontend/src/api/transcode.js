import { fetchURL, adjustedData } from './utils';
import { getApiPath } from '@/utils/url.js';

/** POST /api/media/transcode/sessions */
export async function startTranscodeSession(body) {
  const res = await fetchURL(getApiPath('media/transcode/sessions'), {
    method: 'POST',
    headers: { 'Content-Type': 'application/json' },
    body: JSON.stringify(body),
  });
  const data = await res.json();
  return adjustedData(data);
}

/** GET /api/media/transcode/sessions */
export async function getTranscodeSessions(params = {}) {
  const res = await fetchURL(getApiPath('media/transcode/sessions', params));
  return adjustedData(await res.json());
}

/** DELETE /api/media/transcode/sessions/{id} */
export async function stopTranscodeSession(sessionId) {
  await fetchURL(getApiPath(`media/transcode/sessions/${encodeURIComponent(sessionId)}`), {
    method: 'DELETE',
  });
}

/** PATCH /api/media/transcode/sessions/{id}/heartbeat */
export async function heartbeatTranscodeSession(sessionId, body) {
  await fetchURL(getApiPath(`media/transcode/sessions/${encodeURIComponent(sessionId)}/heartbeat`), {
    method: 'PATCH',
    headers: { 'Content-Type': 'application/json' },
    body: JSON.stringify(body),
  });
}

/** Absolute playlist URL for hls.js (session delivery base + /index.m3u8). */
export function getTranscodePlaylistURL(session) {
  const base = session?.deliveryBaseUrl || session?.deliveryBaseURL;
  if (!base) {
    throw new Error('missing transcode delivery URL');
  }
  if (/^https?:\/\//i.test(base)) {
    return `${base.replace(/\/$/, '')}/index.m3u8`;
  }
  return `${window.origin}${base}/index.m3u8`;
}

export function transcodeEventsURL() {
  return getApiPath('media/transcode/sessions/events');
}
