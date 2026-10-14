import { parseTranscodeModeFromQuery, PLAYBACK_TRANSCODE_QUERY_KEY } from '@/utils/playbackQuery.js';

export const TRANSCODE_PROFILES = {
  quality: 'quality',
  balanced: 'balanced',
  datasaver: 'datasaver',
};

/** Map shareable playback query mode to backend profile. */
export function profileFromTranscodeMode(mode) {
  return TRANSCODE_PROFILES[mode] ?? TRANSCODE_PROFILES.quality;
}

export function transcodeModeFromRouteQuery(query) {
  return parseTranscodeModeFromQuery(query);
}

export function shouldPreferTranscode(query, forceNative = false) {
  if (forceNative) {
    return false;
  }
  return Boolean(parseTranscodeModeFromQuery(query));
}

export { PLAYBACK_TRANSCODE_QUERY_KEY };
