/** Whether this server can serve transcoded playback right now. */

import { state } from '@/store/index.ts';
import { globalVars } from '@/utils/constants.js';

/**
 * `globalVars` are static page constants, and `ffmpegAvailable` mirrors ffmpeg
 * availability, which cannot change without a restart. Transcoding is ffmpeg-only,
 * so it gates the feature outright. The transcode toggle itself is read from the
 * live media settings so an admin change applies without a page reload.
 */
export function isTranscodeAvailable() {
  if (globalVars?.ffmpegAvailable !== true) {
    return false;
  }
  const live = state?.mediaPlayerSettings;
  if (live) {
    return live.enabled === true;
  }
  return globalVars.transcodeEnabled === true;
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
