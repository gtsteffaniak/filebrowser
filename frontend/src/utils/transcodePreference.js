import { parseTranscodeModeFromQuery } from './playbackUrl.js';

/** localStorage key for the preferred HLS transcode profile when transcoding. */
export const TRANSCODE_PREFERENCE_STORAGE_KEY = 'filebrowser.transcodeMode';

export const DEFAULT_TRANSCODE_MODE = 'quality';

const VALID_PREFERRED_MODES = new Set(['quality', 'datasaver']);

/**
 * @returns {'quality' | 'datasaver'}
 */
export function loadPreferredTranscodeMode() {
  try {
    const raw = localStorage.getItem(TRANSCODE_PREFERENCE_STORAGE_KEY);
    if (VALID_PREFERRED_MODES.has(raw)) {
      return raw;
    }
  } catch {
    /* private browsing / blocked storage */
  }
  return DEFAULT_TRANSCODE_MODE;
}

/**
 * URL transcode param wins; otherwise use localStorage (default quality).
 * @param {Record<string, unknown> | undefined} query
 * @returns {'quality' | 'datasaver'}
 */
export function resolveTranscodeModeForPlayback(query) {
  return parseTranscodeModeFromQuery(query) ?? loadPreferredTranscodeMode();
}

/**
 * @param {Record<string, unknown> | undefined} query
 * @returns {boolean}
 */
export function isTranscodeModeRequestedInUrl(query) {
  return parseTranscodeModeFromQuery(query) !== null;
}

/**
 * @param {'quality' | 'datasaver' | string} mode
 */
export function savePreferredTranscodeMode(mode) {
  if (!VALID_PREFERRED_MODES.has(mode)) {
    return;
  }
  try {
    localStorage.setItem(TRANSCODE_PREFERENCE_STORAGE_KEY, mode);
  } catch {
    /* private browsing / blocked storage */
  }
}
