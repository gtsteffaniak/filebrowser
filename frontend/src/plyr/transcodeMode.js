/** Whether this server can serve transcoded playback right now. */

import { globalVars } from '@/utils/constants.js';

/** Transcode availability is fixed for the process lifetime via page constants. */
export function isTranscodeAvailable() {
  return globalVars?.mediaAvailable === true && globalVars?.transcodeEnabled === true;
}

/** Supported transcode playback modes in the player UI. */
export const TRANSCODE_MODES = ['native', 'quality', 'balanced', 'datasaver'];

/** Modes that use server-side HLS transcoding. */
export const TRANSCODE_HLS_MODES = ['quality', 'balanced', 'datasaver'];

export function normalizeTranscodeMode(mode) {
  const value = String(mode || 'native').toLowerCase();
  return TRANSCODE_MODES.includes(value) ? value : 'native';
}

export function isTranscodeHlsMode(mode) {
  return TRANSCODE_HLS_MODES.includes(normalizeTranscodeMode(mode));
}

export function transcodeLog() {}
export function transcodeWarn() {}
export function transcodeError() {}
export function transcodeVerbose() {}

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
