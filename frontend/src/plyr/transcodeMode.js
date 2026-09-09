/** Supported transcode playback modes in the player UI. */
export const TRANSCODE_MODES = ['native', 'quality', 'datasaver'];

/** Modes that use server-side HLS transcoding. */
export const TRANSCODE_HLS_MODES = ['quality', 'datasaver'];

export function normalizeTranscodeMode(mode) {
  const value = String(mode || 'native').toLowerCase();
  return TRANSCODE_MODES.includes(value) ? value : 'native';
}

export function isTranscodeHlsMode(mode) {
  return TRANSCODE_HLS_MODES.includes(normalizeTranscodeMode(mode));
}

/** Treat same-file active sessions as capacity-neutral for the current viewer. */
export function normalizeTranscodeSessionStatus(status, { source, path } = {}) {
  if (!status || status.canStart || status.blockReason !== 'user_limit') {
    return status;
  }
  const sameFileActive = (status.sessions || []).some(
    (session) => session.source === source && session.path === path,
  );
  if (!sameFileActive) {
    return status;
  }
  return { ...status, canStart: true, blockReason: '' };
}
