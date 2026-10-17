import { getTranscodeSessions } from '@/api/transcode.js';
import { isTranscodeAvailable, normalizeTranscodeSessionStatus } from '@/plyr/transcodeMode.js';
import { needsTranscodeFirst } from '@/utils/canBrowserPlayNative.js';
import { parseTranscodeModeFromQuery } from '@/utils/playbackQuery.js';

/** Map API session snapshot to the preview offer UI model. */
export function videoTranscodeOfferFromStatus(status, { source, path } = {}) {
  const normalized = normalizeTranscodeSessionStatus(status, { source, path });
  let mode = 'offer';
  if (normalized.enabled === false) {
    mode = 'unsupported';
  } else if (!normalized.canStart) {
    if (normalized.blockReason === 'user_limit') {
      mode = 'user_limit';
    } else if (normalized.blockReason === 'global_limit') {
      mode = 'system_limit';
    }
  }
  return {
    mode,
    sessions: normalized.sessions || [],
    systemLimit: normalized.globalLimit,
    userLimit: normalized.userLimit,
    status: normalized,
  };
}

/** Fetch transcode capacity and normalize for the current file. */
export async function fetchTranscodeOffer({ source, path }) {
  if (!isTranscodeAvailable()) {
    return videoTranscodeOfferFromStatus(
      { enabled: false, canStart: false, sessions: [] },
      { source, path },
    );
  }
  const status = await getTranscodeSessions();
  return videoTranscodeOfferFromStatus(status, { source, path });
}

/**
 * Before mounting plyr, offer transcode when metadata indicates native playback will fail.
 * Returns an offer view, or null when no offer should be shown.
 */
export async function proactiveVideoTranscodeOffer({
  expectedPath,
  currentPath,
  source,
  previewType,
  isShare,
  route,
  metadata,
  mimeType,
  fileName,
  transcodePlaybackActive,
}) {
  if (currentPath !== expectedPath) {
    return null;
  }
  if (previewType !== 'video' || isShare) {
    return null;
  }
  if (transcodePlaybackActive || parseTranscodeModeFromQuery(route?.query)) {
    return null;
  }
  const meta = metadata || {};
  if (!needsTranscodeFirst({
    videoCodec: meta.videoCodec,
    audioCodec: meta.audioCodec,
    mimeType,
    fileName,
  })) {
    return null;
  }
  return fetchTranscodeOffer({ source, path: expectedPath });
}
